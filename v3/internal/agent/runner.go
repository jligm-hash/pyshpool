package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jligm-hash/pyshpool/v3/internal/core"
)

// Config controls planning and execution.
type Config struct {
	ProviderName    string
	Worker          string
	WorkerCommand   string
	Model           string
	BaseURL         string
	APIKey          string
	Parallel        int
	Retries         int
	MaxOutputTokens int
	MaxTokens       int
	Temperature     float64
	TaskTimeout     time.Duration
	RequestTimeout  time.Duration
	WorkDir         string
	Cwd             string
	Shell           string
	NoSynthesize    bool
	// Progress is called once per finished task. It runs on the scheduler
	// goroutine, so implementations do not need their own lock.
	Progress func(TaskResult)
}

// Runner owns a provider and executes plans.
type Runner struct {
	cfg      Config
	provider Provider
}

// NewRunner validates config, fills defaults, and constructs the provider.
func NewRunner(cfg Config) (*Runner, error) {
	if cfg.Worker == "" {
		cfg.Worker = "api"
	}
	if cfg.Worker != "api" && cfg.Worker != "command" {
		return nil, fmt.Errorf("worker must be api or command, got %q", cfg.Worker)
	}
	if cfg.Worker == "command" && strings.TrimSpace(cfg.WorkerCommand) == "" {
		return nil, fmt.Errorf("--worker-command is required when --worker=command")
	}
	if cfg.Parallel <= 0 {
		cfg.Parallel = 4
	}
	if cfg.Retries < 0 {
		cfg.Retries = 0
	}
	if cfg.MaxOutputTokens <= 0 {
		cfg.MaxOutputTokens = 2048
	}
	if cfg.TaskTimeout <= 0 {
		cfg.TaskTimeout = 10 * time.Minute
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 2 * time.Minute
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = ".pysh-agent"
	}
	if cfg.Shell == "" {
		cfg.Shell = core.DetectShell()
	}
	provider, err := NewProvider(cfg.ProviderName, ProviderConfig{
		APIKey:         cfg.APIKey,
		BaseURL:        cfg.BaseURL,
		Model:          cfg.Model,
		RequestTimeout: cfg.RequestTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &Runner{cfg: cfg, provider: provider}, nil
}

// Provider returns the configured provider.
func (r *Runner) Provider() Provider { return r.provider }

// WithProgress returns a copy of the runner with a progress callback. The
// provider is shared, and the original runner is left unchanged.
func (r *Runner) WithProgress(fn func(TaskResult)) *Runner {
	cp := *r
	cp.cfg.Progress = fn
	return &cp
}

// Plan generates a plan with the configured provider.
func (r *Runner) Plan(ctx context.Context, goal string) (Plan, error) {
	return (&Planner{
		Provider:    r.provider,
		Model:       r.cfg.Model,
		MaxTokens:   r.cfg.MaxOutputTokens,
		Temperature: r.cfg.Temperature,
	}).Plan(ctx, goal)
}

// Execute runs a validated plan and persists every artifact below store.
func (r *Runner) Execute(ctx context.Context, store *Store, plan Plan) (Summary, error) {
	if err := Validate(plan); err != nil {
		return Summary{}, err
	}
	if store == nil {
		return Summary{}, fmt.Errorf("store is nil")
	}
	if err := store.WritePlan(plan); err != nil {
		return Summary{}, fmt.Errorf("write plan: %w", err)
	}

	started := time.Now().UTC()
	summary := Summary{
		RunID:     store.RunID,
		RunDir:    store.Dir,
		Goal:      plan.Goal,
		Status:    "success",
		StartedAt: started.Format(time.RFC3339Nano),
		Total:     len(plan.Tasks),
	}

	results := make(map[string]TaskResult, len(plan.Tasks))
	outputs := make(map[string]string, len(plan.Tasks))
	startedSet := make(map[string]bool, len(plan.Tasks))
	done := make(map[string]bool, len(plan.Tasks))

	// Resume: reuse only tasks whose meta says success and whose output exists.
	for id, res := range store.LoadCompleted(plan) {
		results[id] = res
		startedSet[id] = true
		done[id] = true
		if out, err := store.ReadOutput(id); err == nil {
			outputs[id] = out
		}
	}

	type taskOutcome struct {
		result TaskResult
		output string
		usage  Usage
	}
	outcomes := make(chan taskOutcome, len(plan.Tasks))
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	running := 0
	finished := len(done)
	totalUsage := Usage{}
	budgetExceeded := false

	markSkipped := func(task Task, reason string) {
		if startedSet[task.ID] {
			return
		}
		startedSet[task.ID] = true
		done[task.ID] = true
		finished++
		res := TaskResult{
			ID:     task.ID,
			Title:  task.Title,
			Status: core.StatusSkipped,
			Error:  reason,
		}
		results[task.ID] = res
		meta := TaskMeta{ID: task.ID, Title: task.Title, Status: core.StatusSkipped, Error: reason, Worker: r.cfg.Worker}
		_ = store.WriteMeta(meta)
		_ = store.AppendEvent(Event{
			Time: time.Now().UTC().Format(time.RFC3339Nano), RunID: store.RunID,
			TaskID: task.ID, Status: core.StatusSkipped, Error: reason,
		})
		if r.cfg.Progress != nil {
			r.cfg.Progress(res)
		}
	}

	for finished < len(plan.Tasks) {
		madeProgress := false

		for _, task := range plan.Tasks {
			if startedSet[task.ID] {
				continue
			}

			if budgetExceeded {
				markSkipped(task, "token budget exceeded")
				madeProgress = true
				continue
			}

			ready := true
			depFailed := false
			for _, dep := range task.DependsOn {
				if !done[dep] {
					ready = false
					break
				}
				if results[dep].Status != core.StatusSuccess {
					depFailed = true
				}
			}
			if !ready {
				continue
			}
			if depFailed {
				markSkipped(task, "dependency did not succeed")
				madeProgress = true
				continue
			}
			if running >= r.cfg.Parallel {
				continue
			}

			startedSet[task.ID] = true
			running++
			madeProgress = true
			deps := make(map[string]string, len(task.DependsOn))
			for _, dep := range task.DependsOn {
				deps[dep] = outputs[dep]
			}
			go func(task Task, deps map[string]string) {
				results, output, usage := r.runTask(runCtx, store, plan.Goal, task, deps)
				outcomes <- taskOutcome{result: results, output: output, usage: usage}
			}(task, deps)
		}

		if running == 0 {
			if finished >= len(plan.Tasks) {
				break
			}
			if !madeProgress {
				// Should be unreachable after Validate, but never spin.
				for _, task := range plan.Tasks {
					if !startedSet[task.ID] {
						markSkipped(task, "no runnable task (dependency cycle or failed dependency)")
					}
				}
				break
			}
			continue
		}

		oc := <-outcomes
		running--
		results[oc.result.ID] = oc.result
		startedSet[oc.result.ID] = true
		done[oc.result.ID] = true
		finished++
		totalUsage = totalUsage.Add(oc.usage)
		if oc.result.Status == core.StatusSuccess {
			outputs[oc.result.ID] = oc.output
		}

		if r.cfg.MaxTokens > 0 && totalUsage.InputTokens+totalUsage.OutputTokens > r.cfg.MaxTokens {
			budgetExceeded = true
			summary.Error = fmt.Sprintf("token budget exceeded (%d > %d)", totalUsage.InputTokens+totalUsage.OutputTokens, r.cfg.MaxTokens)
			cancel()
		}

		_ = store.AppendEvent(Event{
			Time:         time.Now().UTC().Format(time.RFC3339Nano),
			RunID:        store.RunID,
			TaskID:       oc.result.ID,
			Status:       oc.result.Status,
			Attempts:     oc.result.Attempts,
			DurationMS:   oc.result.DurationMS,
			Error:        oc.result.Error,
			InputTokens:  oc.usage.InputTokens,
			OutputTokens: oc.usage.OutputTokens,
		})
		if r.cfg.Progress != nil {
			r.cfg.Progress(oc.result)
		}
	}

	summary.Usage = totalUsage
	for _, task := range plan.Tasks {
		res, ok := results[task.ID]
		if !ok {
			res = TaskResult{ID: task.ID, Title: task.Title, Status: core.StatusSkipped, Error: "not run"}
			results[task.ID] = res
		}
		switch res.Status {
		case core.StatusSuccess:
			summary.Succeeded++
		case core.StatusSkipped:
			summary.Skipped++
		default:
			summary.Failed++
		}
		summary.Results = append(summary.Results, res)
	}
	sort.Slice(summary.Results, func(i, j int) bool { return summary.Results[i].ID < summary.Results[j].ID })

	allSucceeded := summary.Failed == 0 && summary.Skipped == 0
	if budgetExceeded {
		summary.Status = "partial"
		if summary.Succeeded == 0 {
			summary.Status = "failed"
		}
		if summary.Error == "" {
			summary.Error = "token budget exceeded"
		}
	}
	shouldSynthesize := allSucceeded && !r.cfg.NoSynthesize && !budgetExceeded && ctx.Err() == nil

	if shouldSynthesize {
		res, final, usage, err := r.synthesize(ctx, store, plan, outputs)
		totalUsage = totalUsage.Add(usage)
		summary.Usage = totalUsage
		summary.Results = append(summary.Results, res)
		if err != nil {
			summary.Status = "partial"
			summary.Error = "synthesis failed: " + err.Error()
			if final == "" {
				final = combineOutputs(plan, outputs)
			}
		}
		if final != "" {
			if path, writeErr := store.WriteFinal(final); writeErr == nil {
				summary.FinalPath = path
			} else {
				summary.Status = "partial"
				if summary.Error == "" {
					summary.Error = "write final: " + writeErr.Error()
				}
			}
		}
		_ = store.AppendEvent(Event{
			Time: time.Now().UTC().Format(time.RFC3339Nano), RunID: store.RunID,
			TaskID: "__synthesis__", Status: res.Status, Attempts: res.Attempts,
			DurationMS: res.DurationMS, Error: res.Error,
			InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		})
	} else {
		final := combineOutputs(plan, outputs)
		if final != "" {
			if path, err := store.WriteFinal(final); err == nil {
				summary.FinalPath = path
			} else {
				summary.Status = "partial"
				if summary.Error == "" {
					summary.Error = "write final: " + err.Error()
				}
			}
		}
		if !allSucceeded {
			summary.Status = "partial"
			if summary.Succeeded == 0 {
				summary.Status = "failed"
			}
			if summary.Error == "" {
				summary.Error = "one or more tasks failed; synthesis skipped"
			}
		}
	}

	if ctx.Err() != nil {
		summary.Status = "canceled"
		if summary.Error == "" {
			summary.Error = "run canceled"
		}
	}

	finishedAt := time.Now().UTC()
	summary.FinishedAt = finishedAt.Format(time.RFC3339Nano)
	summary.DurationMS = finishedAt.Sub(started).Milliseconds()
	if err := store.WriteSummary(summary); err != nil {
		return summary, err
	}
	return summary, nil
}

func (r *Runner) runTask(ctx context.Context, store *Store, goal string, task Task, deps map[string]string) (TaskResult, string, Usage) {
	started := time.Now().UTC()
	result := TaskResult{ID: task.ID, Title: task.Title, Status: core.StatusFailed}
	meta := TaskMeta{
		ID:        task.ID,
		Title:     task.Title,
		Status:    core.StatusFailed,
		StartedAt: started.Format(time.RFC3339Nano),
		Worker:    r.cfg.Worker,
	}
	prompt := buildTaskPrompt(goal, task, deps)
	if err := store.PrepareTask(task.ID); err != nil {
		result.Error = err.Error()
		return result, "", Usage{}
	}
	if err := store.WritePrompt(task.ID, prompt); err != nil {
		result.Error = err.Error()
		return result, "", Usage{}
	}

	maxAttempts := r.cfg.Retries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var totalUsage Usage
	taskCtx := ctx
	var cancelTask context.CancelFunc
	if r.cfg.TaskTimeout > 0 {
		taskCtx, cancelTask = context.WithTimeout(ctx, r.cfg.TaskTimeout)
		defer cancelTask()
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt
		if taskCtx.Err() != nil {
			if ctx.Err() != nil {
				result.Status = core.StatusCanceled
				result.Error = "run canceled"
			} else {
				result.Status = core.StatusTimeout
				result.Error = fmt.Sprintf("timeout after %s", r.cfg.TaskTimeout)
			}
			break
		}

		var output, model string
		var usage Usage
		var err error
		if r.cfg.Worker == "command" {
			output, err = r.runCommandWorker(taskCtx, store, goal, task, prompt)
			model = "command"
		} else {
			resp, callErr := r.provider.Complete(taskCtx, CompletionRequest{
				Model:       r.cfg.Model,
				MaxTokens:   r.cfg.MaxOutputTokens,
				Temperature: r.cfg.Temperature,
				Messages: []Message{
					{Role: "system", Content: workerSystemPrompt},
					{Role: "user", Content: workerUserPrompt(goal, task, deps)},
				},
				Metadata: map[string]string{
					"kind":    "task",
					"goal":    goal,
					"task_id": task.ID,
					"title":   task.Title,
				},
			})
			err = callErr
			if callErr == nil {
				output = resp.Text
				usage = resp.Usage
				model = resp.Model
			}
		}
		timedOut := err != nil && taskCtx.Err() == context.DeadlineExceeded
		totalUsage = totalUsage.Add(usage)
		if err == nil && strings.TrimSpace(output) == "" {
			err = fmt.Errorf("worker returned empty output")
		}
		if err == nil {
			path, writeErr := store.WriteOutput(task.ID, output)
			if writeErr != nil {
				err = fmt.Errorf("write output: %w", writeErr)
			} else {
				result.Status = core.StatusSuccess
				result.OutputPath = path
				result.Error = ""
				meta.Status = core.StatusSuccess
				meta.Model = model
				if r.cfg.Worker == "command" {
					meta.Provider = "command"
				} else {
					meta.Provider = r.provider.Name()
				}
				meta.Usage = totalUsage
				meta.Error = ""
				result.Usage = totalUsage
				result.DurationMS = time.Since(started).Milliseconds()
				meta.DurationMS = result.DurationMS
				meta.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
				_ = store.WriteMeta(meta)
				return result, output, totalUsage
			}
		}

		if ctx.Err() != nil {
			result.Status = core.StatusCanceled
			result.Error = "run canceled"
			break
		}
		if timedOut {
			result.Status = core.StatusTimeout
			result.Error = fmt.Sprintf("timeout after %s", r.cfg.TaskTimeout)
			break
		}
		result.Status = core.StatusFailed
		result.Error = err.Error()
		if attempt < maxAttempts {
			if !sleepContext(taskCtx, retryDelay(attempt)) {
				if ctx.Err() != nil {
					result.Status = core.StatusCanceled
					result.Error = "run canceled"
				} else {
					result.Status = core.StatusTimeout
					result.Error = fmt.Sprintf("timeout after %s", r.cfg.TaskTimeout)
				}
				break
			}
		}
	}

	result.DurationMS = time.Since(started).Milliseconds()
	result.Usage = totalUsage
	meta.Status = result.Status
	meta.Error = result.Error
	meta.Usage = totalUsage
	meta.DurationMS = result.DurationMS
	meta.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = store.WriteMeta(meta)
	return result, "", totalUsage
}

func (r *Runner) runCommandWorker(ctx context.Context, store *Store, goal string, task Task, prompt string) (string, error) {
	dir := store.TaskDir(task.ID)
	promptPath := filepath.Join(dir, "prompt.txt")
	outputPath := filepath.Join(dir, "output.md")
	stdoutPath := filepath.Join(dir, "stdout.log")
	stderrPath := filepath.Join(dir, "stderr.log")

	expanded := r.cfg.WorkerCommand
	replacements := map[string]string{
		"{{prompt_file}}": promptPath,
		"{{output_file}}": outputPath,
		"{{task_id}}":     task.ID,
		"{{goal}}":        goal,
	}
	for key, value := range replacements {
		expanded = strings.ReplaceAll(expanded, key, shellQuote(value))
	}

	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return "", fmt.Errorf("create stdout log: %w", err)
	}
	defer stdout.Close()
	stderr, err := os.Create(stderrPath)
	if err != nil {
		return "", fmt.Errorf("create stderr log: %w", err)
	}
	defer stderr.Close()

	cmd := exec.Command(r.cfg.Shell, "-c", expanded)
	if r.cfg.Cwd != "" {
		cmd.Dir = r.cfg.Cwd
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(),
		"PYSH_PROMPT_FILE="+promptPath,
		"PYSH_OUTPUT_FILE="+outputPath,
		"PYSH_TASK_ID="+task.ID,
		"PYSH_GOAL="+goal,
	)
	promptFile, err := os.Open(promptPath)
	if err != nil {
		return "", fmt.Errorf("open prompt: %w", err)
	}
	defer promptFile.Close()
	cmd.Stdin = promptFile
	core.ConfigureProcess(cmd)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start command worker: %w", err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	var runErr error
	select {
	case runErr = <-waitCh:
	case <-ctx.Done():
		core.KillProcess(cmd)
		runErr = <-waitCh
		if runErr == nil {
			runErr = ctx.Err()
		}
	}
	if data, err := os.ReadFile(outputPath); err == nil && strings.TrimSpace(string(data)) != "" {
		if runErr != nil {
			return "", fmt.Errorf("command worker failed: %w", runErr)
		}
		return string(data), nil
	}
	if runErr != nil {
		tail := tailFile(stderrPath, 1200)
		return "", fmt.Errorf("command worker failed: %w: %s", runErr, tail)
	}
	// A command that prints its answer on stdout is supported too.
	if data, err := os.ReadFile(stdoutPath); err == nil && strings.TrimSpace(string(data)) != "" {
		return string(data), nil
	}
	return "", fmt.Errorf("command worker produced no output (write to {{output_file}} or stdout)")
}

func (r *Runner) synthesize(ctx context.Context, store *Store, plan Plan, outputs map[string]string) (TaskResult, string, Usage, error) {
	started := time.Now().UTC()
	ids := make([]string, 0, len(outputs))
	for id := range outputs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	resp, err := r.provider.Complete(ctx, CompletionRequest{
		Model:       r.cfg.Model,
		MaxTokens:   r.cfg.MaxOutputTokens,
		Temperature: r.cfg.Temperature,
		Messages:    synthesisMessages(plan.Goal, plan, outputs),
		Metadata: map[string]string{
			"kind":  "synthesis",
			"goal":  plan.Goal,
			"tasks": strings.Join(ids, ","),
		},
	})
	res := TaskResult{ID: "__synthesis__", Title: "Synthesize final result", Attempts: 1}
	if err != nil {
		res.Status = core.StatusFailed
		res.Error = err.Error()
		res.DurationMS = time.Since(started).Milliseconds()
		return res, "", Usage{}, err
	}
	res.Status = core.StatusSuccess
	res.DurationMS = time.Since(started).Milliseconds()
	res.Usage = resp.Usage
	return res, resp.Text, resp.Usage, nil
}

func combineOutputs(plan Plan, outputs map[string]string) string {
	var b strings.Builder
	b.WriteString("# Partial results\n\n")
	b.WriteString("Synthesis was skipped. The successful worker outputs are collected below.\n")
	for _, task := range plan.Tasks {
		out, ok := outputs[task.ID]
		if !ok || strings.TrimSpace(out) == "" {
			continue
		}
		fmt.Fprintf(&b, "\n## %s", task.ID)
		if task.Title != "" {
			fmt.Fprintf(&b, " — %s", task.Title)
		}
		fmt.Fprintf(&b, "\n%s\n", out)
	}
	return b.String()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func tailFile(path string, max int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(data))
	if len(s) <= max {
		return s
	}
	return "..." + s[len(s)-max:]
}

func retryDelay(attempt int) time.Duration {
	d := time.Duration(attempt) * 200 * time.Millisecond
	if d > 2*time.Second {
		d = 2 * time.Second
	}
	return d
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
