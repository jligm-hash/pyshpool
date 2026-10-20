package agent

import (
	"fmt"
	"sort"
	"strings"
)

const maxDependencyChars = 16000
const maxSynthesisChars = 60000

const workerSystemPrompt = `You are a focused worker in pysh-agent. Complete only the assigned task.
Use the dependency outputs as evidence, but do not repeat them wholesale.
Return concise, well-structured Markdown. State uncertainties and assumptions explicitly.
Do not claim to have run commands, browsed the web, or edited files unless the task actually did so.`

func workerUserPrompt(goal string, task Task, deps map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Overall goal\n%s\n\n", goal)
	fmt.Fprintf(&b, "# Your task\nID: %s\n", task.ID)
	if task.Title != "" {
		fmt.Fprintf(&b, "Title: %s\n", task.Title)
	}
	fmt.Fprintf(&b, "\n%s\n", task.Prompt)
	if task.Acceptance != "" {
		fmt.Fprintf(&b, "\n# Acceptance criteria\n%s\n", task.Acceptance)
	}
	if len(deps) > 0 {
		b.WriteString("\n# Dependency outputs\n")
		ids := make([]string, 0, len(deps))
		for id := range deps {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Fprintf(&b, "\n## %s\n%s\n", id, truncateText(deps[id], maxDependencyChars))
		}
	}
	return b.String()
}

func buildTaskPrompt(goal string, task Task, deps map[string]string) string {
	return workerSystemPrompt + "\n\n" + workerUserPrompt(goal, task, deps)
}

func synthesisMessages(goal string, plan Plan, outputs map[string]string) []Message {
	var b strings.Builder
	fmt.Fprintf(&b, "# Goal\n%s\n\n", goal)
	b.WriteString("# Worker outputs\n")
	total := 0
	for _, task := range plan.Tasks {
		out, ok := outputs[task.ID]
		if !ok || strings.TrimSpace(out) == "" {
			continue
		}
		remaining := maxSynthesisChars - total
		if remaining <= 0 {
			b.WriteString("\n(remaining outputs omitted for context length)\n")
			break
		}
		text := truncateText(out, remaining)
		fmt.Fprintf(&b, "\n## %s", task.ID)
		if task.Title != "" {
			fmt.Fprintf(&b, " — %s", task.Title)
		}
		fmt.Fprintf(&b, "\n%s\n", text)
		total += len(text)
	}
	b.WriteString("\n# Instructions\n")
	b.WriteString("Write the final deliverable for the goal. Merge duplicate points, resolve contradictions, ")
	b.WriteString("preserve important caveats, and cite task IDs where a claim comes from a specific worker. ")
	b.WriteString("Return Markdown only, with no preamble about being an AI.")

	return []Message{
		{Role: "system", Content: "You are the synthesizer for pysh-agent. Produce the final, self-contained answer."},
		{Role: "user", Content: b.String()},
	}
}

func truncateText(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max < 20 {
		return string(runes[:max])
	}
	return string(runes[:max-15]) + "\n...[truncated]"
}
