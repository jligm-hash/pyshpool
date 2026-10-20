package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MockProvider is deterministic and requires no API key. It exists so the
// full plan/run/resume pipeline can be exercised in tests and demos.
type MockProvider struct{}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	select {
	case <-ctx.Done():
		return CompletionResponse{}, ctx.Err()
	default:
	}

	kind := req.Metadata["kind"]
	goal := req.Metadata["goal"]
	if goal == "" {
		goal = "mock goal"
	}

	var text string
	switch kind {
	case "plan":
		plan := Plan{
			Goal: goal,
			Tasks: []Task{
				{ID: "analyze-a", Title: "Analyze angle A", Prompt: "Identify the key facts and constraints for angle A of the goal."},
				{ID: "analyze-b", Title: "Analyze angle B", Prompt: "Identify the key facts and constraints for angle B of the goal."},
				{ID: "draft", Title: "Draft the result", Prompt: "Combine the analyses into a concise draft.", DependsOn: []string{"analyze-a", "analyze-b"}},
				{ID: "review", Title: "Review the draft", Prompt: "Review the draft for gaps, contradictions, and clarity.", DependsOn: []string{"draft"}},
			},
		}
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return CompletionResponse{}, err
		}
		text = string(data)
	case "task":
		id := req.Metadata["task_id"]
		title := req.Metadata["title"]
		if title == "" {
			title = id
		}
		text = fmt.Sprintf("# %s\n\nMock worker output for task `%s`.\n\nGoal: %s\n\nThis output is deterministic and is intended for demos and tests.", title, id, goal)
	case "synthesis":
		ids := req.Metadata["tasks"]
		if ids == "" {
			ids = "(none)"
		}
		text = fmt.Sprintf("# Result\n\nMock synthesis for goal: %s\n\nInputs: %s\n\nAll worker outputs were combined successfully.", goal, ids)
	default:
		text = "Mock response.\n\n" + lastUserContent(req.Messages)
	}

	return CompletionResponse{
		Text:  strings.TrimSpace(text),
		Model: "mock",
		Usage: Usage{
			InputTokens:  estimateTokens(messagesText(req.Messages)),
			OutputTokens: estimateTokens(text),
		},
	}, nil
}

func messagesText(messages []Message) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	return b.String()
}

func lastUserContent(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func estimateTokens(s string) int {
	n := len([]rune(s)) / 4
	if n < 1 {
		n = 1
	}
	return n
}
