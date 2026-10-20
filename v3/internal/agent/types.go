// Package agent implements the pysh-agent planner/worker/synthesizer pipeline.
package agent

import (
	"context"

	"github.com/jligm-hash/pyshpool/v3/internal/core"
)

// Plan is the structured decomposition produced by the planner.
type Plan struct {
	Goal  string `json:"goal"`
	Tasks []Task `json:"tasks"`
}

// Task is one node in the plan DAG.
type Task struct {
	ID         string   `json:"id"`
	Title      string   `json:"title,omitempty"`
	Prompt     string   `json:"prompt"`
	DependsOn  []string `json:"depends_on,omitempty"`
	Acceptance string   `json:"acceptance,omitempty"`
}

// Usage records token counts reported by a provider.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Add returns the sum of two usages.
func (u Usage) Add(other Usage) Usage {
	return Usage{InputTokens: u.InputTokens + other.InputTokens, OutputTokens: u.OutputTokens + other.OutputTokens}
}

// Message is one chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest is provider-independent.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	JSON        bool
	// Metadata is ignored by HTTP providers; the mock provider uses it to
	// return deterministic planner/task/synthesis outputs.
	Metadata map[string]string
}

// CompletionResponse is provider-independent.
type CompletionResponse struct {
	Text  string
	Usage Usage
	Model string
}

// Provider is the minimal LLM interface used by the planner, workers, and
// synthesizer.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// TaskMeta is persisted next to each task.
type TaskMeta struct {
	ID         string      `json:"id"`
	Title      string      `json:"title,omitempty"`
	Status     core.Status `json:"status"`
	Attempts   int         `json:"attempts"`
	StartedAt  string      `json:"started_at,omitempty"`
	FinishedAt string      `json:"finished_at,omitempty"`
	DurationMS int64       `json:"duration_ms"`
	Provider   string      `json:"provider,omitempty"`
	Model      string      `json:"model,omitempty"`
	Worker     string      `json:"worker,omitempty"`
	Usage      Usage       `json:"usage,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// TaskResult is the in-memory result for one task.
type TaskResult struct {
	ID         string      `json:"id"`
	Title      string      `json:"title,omitempty"`
	Status     core.Status `json:"status"`
	Attempts   int         `json:"attempts"`
	DurationMS int64       `json:"duration_ms"`
	OutputPath string      `json:"output_path,omitempty"`
	Error      string      `json:"error,omitempty"`
	Usage      Usage       `json:"usage,omitempty"`
}

// Summary describes a complete agent run.
type Summary struct {
	RunID      string       `json:"run_id"`
	RunDir     string       `json:"run_dir"`
	Goal       string       `json:"goal"`
	Status     string       `json:"status"`
	StartedAt  string       `json:"started_at"`
	FinishedAt string       `json:"finished_at"`
	DurationMS int64        `json:"duration_ms"`
	Total      int          `json:"total"`
	Succeeded  int          `json:"succeeded"`
	Failed     int          `json:"failed"`
	Skipped    int          `json:"skipped"`
	FinalPath  string       `json:"final_path,omitempty"`
	Error      string       `json:"error,omitempty"`
	Usage      Usage        `json:"usage"`
	Results    []TaskResult `json:"results"`
}

// Event is one line in events.jsonl.
type Event struct {
	Time         string      `json:"time"`
	RunID        string      `json:"run_id"`
	TaskID       string      `json:"task_id"`
	Status       core.Status `json:"status"`
	Attempts     int         `json:"attempts"`
	DurationMS   int64       `json:"duration_ms"`
	Error        string      `json:"error,omitempty"`
	InputTokens  int         `json:"input_tokens,omitempty"`
	OutputTokens int         `json:"output_tokens,omitempty"`
}
