package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Planner asks a provider for a structured plan.
type Planner struct {
	Provider    Provider
	Model       string
	MaxTokens   int
	Temperature float64
}

// Plan generates and validates a plan for goal.
func (p *Planner) Plan(ctx context.Context, goal string) (Plan, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return Plan{}, fmt.Errorf("goal is empty")
	}
	if p.Provider == nil {
		return Plan{}, fmt.Errorf("planner has no provider")
	}
	req := CompletionRequest{
		Model:       p.Model,
		MaxTokens:   p.MaxTokens,
		Temperature: p.Temperature,
		JSON:        true,
		Messages: []Message{
			{Role: "system", Content: plannerSystemPrompt},
			{Role: "user", Content: "Goal:\n" + goal},
		},
		Metadata: map[string]string{"kind": "plan", "goal": goal},
	}
	resp, err := p.Provider.Complete(ctx, req)
	if err != nil {
		return Plan{}, fmt.Errorf("planner: %w", err)
	}
	raw, err := extractJSON(resp.Text)
	if err != nil {
		return Plan{}, fmt.Errorf("planner returned no JSON: %w", err)
	}
	var plan Plan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return Plan{}, fmt.Errorf("planner returned invalid JSON: %w", err)
	}
	if strings.TrimSpace(plan.Goal) == "" {
		plan.Goal = goal
	}
	if err := Validate(plan); err != nil {
		return Plan{}, fmt.Errorf("planner returned an invalid plan: %w", err)
	}
	return plan, nil
}

// LoadPlan reads and validates a plan file.
func LoadPlan(path string) (Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, fmt.Errorf("read plan: %w", err)
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("decode plan: %w", err)
	}
	if err := Validate(plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

// ExtractJSON returns the JSON payload from a model response, tolerating a
// markdown fence or surrounding prose.
func extractJSON(text string) (string, error) {
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start < 0 || end < start {
		return "", fmt.Errorf("no JSON object found")
	}
	raw := strings.TrimSpace(s[start : end+1])
	if !json.Valid([]byte(raw)) {
		return "", fmt.Errorf("JSON is not valid")
	}
	return raw, nil
}

const plannerSystemPrompt = `You are the planner for pysh-agent, a parallel task runner.

Decompose the user's goal into a small directed acyclic graph (DAG) of concrete tasks. Tasks that do not depend on each other will run in parallel, so prefer independent branches where possible.

Return ONLY a JSON object with this shape:
{
  "goal": "the original goal",
  "tasks": [
    {
      "id": "short-stable-id",
      "title": "short label",
      "prompt": "self-contained instructions for one worker",
      "depends_on": ["other-task-id"],
      "acceptance": "optional check for a good result"
    }
  ]
}

Rules:
- Use 3 to 8 tasks (never more than 32).
- IDs must match [A-Za-z0-9][A-Za-z0-9._-]{0,63} and be unique.
- Dependencies must refer to IDs in this same plan and must not form a cycle.
- Each prompt must be understandable without the other tasks' prompts.
- Workers are text-only. Do not ask them to run shell commands or edit files.
- Do not include markdown fences or commentary outside the JSON.`
