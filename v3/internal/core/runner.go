package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status is the terminal state of one task.
type Status string

const (
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusTimeout  Status = "timeout"
	StatusCanceled Status = "canceled"
	StatusSkipped  Status = "skipped"
)

// Config controls the local runner.
type Config struct {
	Concurrency int
	Timeout     time.Duration
	Retries     int
	FailFast    bool
	Shell       string
	WorkDir     string
	Cwd         string
	Env         []string
	// Progress is called once per completed task. It is serialized by the
	// runner, so callers do not need their own lock.
	Progress func(Result)
}

// Result describes one executed task.
type Result struct {
	Index      int    `json:"index"`
	Line       int    `json:"line"`
	Command    string `json:"command"`
	Status     Status `json:"status"`
	ExitCode   int    `json:"exit_code"`
	Attempts   int    `json:"attempts"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	DurationMS int64  `json:"duration_ms"`
	LogPath    string `json:"log_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Success reports whether the task completed with exit code zero.
func (r Result) Success() bool { return r.Status == StatusSuccess }

// Summary is the machine-readable result of a run.
type Summary struct {
	RunID       string   `json:"run_id"`
	RunDir      string   `json:"run_dir"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at"`
	DurationMS  int64    `json:"duration_ms"`
	Concurrency int      `json:"concurrency"`
	Shell       string   `json:"shell"`
	Total       int      `json:"total"`
	Succeeded   int      `json:"succeeded"`
	Failed      int      `json:"failed"`
	TimedOut    int      `json:"timed_out"`
	Canceled    int      `json:"canceled"`
	Skipped     int      `json:"skipped"`
	Results     []Result `json:"results"`
}

// Event is one line in events.jsonl.
type Event struct {
	Time       string `json:"time"`
	RunID      string `json:"run_id"`
	Index      int    `json:"index"`
	Line       int    `json:"line"`
	Status     Status `json:"status"`
	ExitCode   int    `json:"exit_code"`
	Attempts   int    `json:"attempts"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Runner executes shell tasks with bounded concurrency.
type Runner struct {
	cfg   Config
	shell string
}

// NewRunner returns a runner with sensible defaults filled in.
func NewRunner(cfg Config) *Runner {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = runtime.NumCPU()
	}
	if cfg.Retries < 0 {
		cfg.Retries = 0
	}
	if cfg.Shell == "" {
		cfg.Shell = DetectShell()
	}
	return &Runner{cfg: cfg, shell: cfg.Shell}
}

// DetectShell prefers bash and falls back to sh.
func DetectShell() string {
	for _, name := range []string{"bash", "sh"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return "sh"
}

// Run executes every task, writes logs and a summary, and returns the summary.
// A non-zero task exit does not produce a Go error; inspect Summary.Failed etc.
func (r *Runner) Run(ctx context.Context, tasks []Task) (Summary, error) {
	if len(tasks) == 0 {
		return Summary{}, fmt.Errorf("no tasks to run")
	}

	workers := r.cfg.Concurrency
	if workers > len(tasks) {
		workers = len(tasks)
	}
	if workers < 1 {
		workers = 1
	}

	runID, runDir, err := CreateRunDir(r.cfg.WorkDir)
	if err != nil {
		return Summary{}, err
	}

	started := time.Now().UTC()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan Task)
	out := make(chan Result, len(tasks))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				res := r.runOne(runCtx, runDir, task)
				out <- res
				if r.cfg.FailFast && !res.Success() {
					cancel()
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, task := range tasks {
			select {
			case <-runCtx.Done():
				return
			case jobs <- task:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	byIndex := make(map[int]Result, len(tasks))
	for res := range out {
		byIndex[res.Index] = res
		if r.cfg.Progress != nil {
			r.cfg.Progress(res)
		}
	}

	results := make([]Result, 0, len(tasks))
	summary := Summary{
		RunID:       runID,
		RunDir:      runDir,
		StartedAt:   started.Format(time.RFC3339Nano),
		Concurrency: workers,
		Shell:       r.shell,
		Total:       len(tasks),
		Results:     results,
	}
	for _, task := range tasks {
		res, ok := byIndex[task.Index]
		if !ok {
			res = Result{
				Index:    task.Index,
				Line:     task.Line,
				Command:  task.Command,
				Status:   StatusSkipped,
				ExitCode: -1,
				Error:    "not started because the run was canceled",
			}
		}
		switch res.Status {
		case StatusSuccess:
			summary.Succeeded++
		case StatusFailed:
			summary.Failed++
		case StatusTimeout:
			summary.TimedOut++
		case StatusCanceled:
			summary.Canceled++
		case StatusSkipped:
			summary.Skipped++
		}
		results = append(results, res)
	}
	summary.Results = results
	finished := time.Now().UTC()
	summary.FinishedAt = finished.Format(time.RFC3339Nano)
	summary.DurationMS = finished.Sub(started).Milliseconds()

	if err := writeJSONAtomic(filepath.Join(runDir, "summary.json"), summary); err != nil {
		return summary, err
	}
	if err := writeEvents(filepath.Join(runDir, "events.jsonl"), summary); err != nil {
		return summary, err
	}
	return summary, nil
}

func (r *Runner) runOne(ctx context.Context, runDir string, task Task) Result {
	started := time.Now().UTC()
	res := Result{
		Index:     task.Index,
		Line:      task.Line,
		Command:   task.Command,
		ExitCode:  -1,
		StartedAt: started.Format(time.RFC3339Nano),
	}
	logPath := filepath.Join(runDir, "tasks", fmt.Sprintf("%04d.log", task.Index))
	res.LogPath = logPath

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		res.Status = StatusFailed
		res.Error = fmt.Sprintf("open log: %v", err)
		res.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		res.DurationMS = time.Since(started).Milliseconds()
		return res
	}
	defer logFile.Close()

	maxAttempts := r.cfg.Retries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		res.Attempts = attempt
		if ctx.Err() != nil {
			res.Status = StatusCanceled
			res.Error = "run canceled"
			break
		}

		fmt.Fprintf(logFile, "=== pysh task %d (line %d), attempt %d/%d ===\n", task.Index, task.Line, attempt, maxAttempts)
		fmt.Fprintf(logFile, "command: %s\n", task.Command)
		fmt.Fprintf(logFile, "started: %s\n\n", time.Now().UTC().Format(time.RFC3339Nano))

		timeoutCtx := ctx
		var cancelTimeout context.CancelFunc
		var timeoutCh <-chan struct{}
		if r.cfg.Timeout > 0 {
			timeoutCtx, cancelTimeout = context.WithTimeout(ctx, r.cfg.Timeout)
			timeoutCh = timeoutCtx.Done()
		}

		cmd := exec.Command(r.shell, "-c", task.Command)
		if r.cfg.Cwd != "" {
			cmd.Dir = r.cfg.Cwd
		}
		if len(r.cfg.Env) > 0 {
			cmd.Env = append(os.Environ(), r.cfg.Env...)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		configureProcess(cmd)

		startErr := cmd.Start()
		if startErr != nil {
			if cancelTimeout != nil {
				cancelTimeout()
			}
			res.Status = StatusFailed
			res.Error = fmt.Sprintf("start: %v", startErr)
			if attempt < maxAttempts {
				fmt.Fprintf(logFile, "\nstart error: %v; retrying\n", startErr)
				if !sleepContext(ctx, retryDelay(attempt)) {
					res.Status = StatusCanceled
					res.Error = "run canceled"
					break
				}
				continue
			}
			break
		}

		waitCh := make(chan error, 1)
		go func() { waitCh <- cmd.Wait() }()

		select {
		case waitErr := <-waitCh:
			if cancelTimeout != nil {
				cancelTimeout()
			}
			if waitErr == nil {
				res.Status = StatusSuccess
				res.ExitCode = 0
				res.Error = ""
				fmt.Fprintf(logFile, "\n=== finished: success (exit 0) ===\n")
				break
			}
			res.Status = StatusFailed
			res.ExitCode = exitCode(waitErr)
			res.Error = waitErr.Error()
			fmt.Fprintf(logFile, "\n=== finished: failed (exit %d) ===\n", res.ExitCode)
			if attempt < maxAttempts {
				fmt.Fprintf(logFile, "retrying in %s\n", retryDelay(attempt))
				if !sleepContext(ctx, retryDelay(attempt)) {
					res.Status = StatusCanceled
					res.Error = "run canceled"
					break
				}
				continue
			}
		case <-timeoutCh:
			killProcess(cmd)
			<-waitCh
			if cancelTimeout != nil {
				cancelTimeout()
			}
			if ctx.Err() != nil {
				res.Status = StatusCanceled
				res.Error = "run canceled"
			} else {
				res.Status = StatusTimeout
				res.Error = fmt.Sprintf("timeout after %s", r.cfg.Timeout)
			}
			fmt.Fprintf(logFile, "\n=== finished: %s ===\n", res.Status)
		case <-ctx.Done():
			killProcess(cmd)
			<-waitCh
			if cancelTimeout != nil {
				cancelTimeout()
			}
			res.Status = StatusCanceled
			res.Error = "run canceled"
			fmt.Fprintf(logFile, "\n=== finished: canceled ===\n")
		}
		if res.Status == StatusSuccess || res.Status == StatusTimeout || res.Status == StatusCanceled {
			break
		}
	}

	finished := time.Now().UTC()
	res.FinishedAt = finished.Format(time.RFC3339Nano)
	res.DurationMS = finished.Sub(started).Milliseconds()
	if res.Status == "" {
		res.Status = StatusFailed
	}
	return res
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
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

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeEvents(path string, summary Summary) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open events: %w", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, res := range summary.Results {
		ev := Event{
			Time:       res.FinishedAt,
			RunID:      summary.RunID,
			Index:      res.Index,
			Line:       res.Line,
			Status:     res.Status,
			ExitCode:   res.ExitCode,
			Attempts:   res.Attempts,
			DurationMS: res.DurationMS,
			Error:      res.Error,
		}
		if err := enc.Encode(ev); err != nil {
			return fmt.Errorf("write event: %w", err)
		}
	}
	return nil
}

// FormatCommand shortens very long commands for terminal output.
func FormatCommand(cmd string, max int) string {
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	if max <= 0 {
		return cmd
	}
	runes := []rune(cmd)
	if len(runes) <= max {
		return cmd
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// SortResults sorts results by task index in place.
func SortResults(results []Result) {
	sort.Slice(results, func(i, j int) bool { return results[i].Index < results[j].Index })
}
