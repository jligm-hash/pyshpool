// Command pysh-agent plans a goal, runs the resulting tasks in parallel, and
// synthesizes the results. It is intentionally a separate binary from pysh:
// the fast shell runner stays small and the agent runner can evolve on its own.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jligm-hash/pyshpool/v3/internal/agent"
	"github.com/jligm-hash/pyshpool/v3/internal/core"
	"github.com/jligm-hash/pyshpool/v3/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "plan":
		return cmdPlan(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "version", "--version", "-version":
		fmt.Println(version.StringFor("pysh-agent"))
		return 0
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "pysh-agent: unknown command %q\n\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `pysh-agent - plan a goal, run parallel workers, synthesize the result

Usage:
  pysh-agent plan -g GOAL [flags]       create and print a plan (no execution)
  pysh-agent run  -g GOAL [flags]       plan and execute
  pysh-agent run  --plan PLAN.json      execute an existing plan
  pysh-agent run  --resume RUN          resume a previous run
  pysh-agent version

The default provider is "mock", so the demo works without an API key.
Use --provider openai or --provider anthropic for real workers.

Run "pysh-agent plan -h" or "pysh-agent run -h" for flags.
`)
}

type providerFlags struct {
	provider       string
	model          string
	baseURL        string
	apiKey         string
	requestTimeout time.Duration
	maxOutput      int
	temperature    float64
}

func (p *providerFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&p.provider, "provider", "mock", "mock, openai, or anthropic")
	fs.StringVar(&p.model, "model", "", "model name (provider default if empty)")
	fs.StringVar(&p.baseURL, "base-url", "", "API base URL (or OPENAI_BASE_URL / ANTHROPIC_BASE_URL)")
	fs.StringVar(&p.apiKey, "api-key", "", "API key (prefer the provider environment variable)")
	fs.DurationVar(&p.requestTimeout, "request-timeout", 2*time.Minute, "timeout for one API request")
	fs.IntVar(&p.maxOutput, "max-output-tokens", 2048, "max output tokens per call")
	fs.Float64Var(&p.temperature, "temperature", 0.2, "sampling temperature")
}

func (p *providerFlags) config() agent.Config {
	return agent.Config{
		ProviderName:    p.provider,
		Model:           p.model,
		BaseURL:         p.baseURL,
		APIKey:          p.apiKey,
		MaxOutputTokens: p.maxOutput,
		Temperature:     p.temperature,
		RequestTimeout:  p.requestTimeout,
	}
}

func cmdPlan(args []string) int {
	fs := flag.NewFlagSet("pysh-agent plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		goal    string
		outPath string
		pf      providerFlags
	)
	fs.StringVar(&goal, "g", "", "goal to decompose")
	fs.StringVar(&goal, "goal", "", "goal to decompose (alias for -g)")
	fs.StringVar(&outPath, "out", "", "write the plan to this path instead of stdout")
	fs.StringVar(&outPath, "plan-out", "", "write the plan to this path (alias for --out)")
	pf.register(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "pysh-agent plan: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if strings.TrimSpace(goal) == "" {
		fmt.Fprintln(os.Stderr, "pysh-agent plan: -g GOAL is required")
		return 2
	}
	if pf.maxOutput <= 0 {
		fmt.Fprintln(os.Stderr, "pysh-agent plan: --max-output-tokens must be positive")
		return 2
	}
	if pf.requestTimeout <= 0 {
		fmt.Fprintln(os.Stderr, "pysh-agent plan: --request-timeout must be positive")
		return 2
	}

	cfg := pf.config()
	cfg.Worker = "api"
	runner, err := agent.NewRunner(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh-agent plan: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	plan, err := runner.Plan(ctx, goal)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh-agent plan: %v\n", err)
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}

	if outPath != "" {
		if err := writePlan(outPath, plan); err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent plan: %v\n", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "pysh-agent plan: wrote %s\n", outPath)
		return 0
	}
	printPlan(plan)
	return 0
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("pysh-agent run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		goal        string
		planPath    string
		resume      string
		planOnly    bool
		dryRun      bool
		planOut     string
		worker      string
		workerCmd   string
		parallel    int
		retries     int
		maxTokens   int
		taskTimeout time.Duration
		workDir     string
		cwd         string
		shell       string
		noSynth     bool
		quiet       bool
		jsonOut     bool
		pf          providerFlags
	)
	fs.StringVar(&goal, "g", "", "goal to plan and execute")
	fs.StringVar(&goal, "goal", "", "goal to plan and execute (alias for -g)")
	fs.StringVar(&planPath, "plan", "", "execute an existing plan JSON file")
	fs.StringVar(&resume, "resume", "", "resume a run directory or run ID")
	fs.BoolVar(&planOnly, "plan-only", false, "create and print the plan without executing")
	fs.BoolVar(&dryRun, "dry-run", false, "print the plan without executing")
	fs.StringVar(&planOut, "plan-out", "", "also write the generated plan to this path")
	fs.StringVar(&worker, "worker", "api", "worker backend: api or command")
	fs.StringVar(&workerCmd, "worker-command", "", "command for --worker=command")
	fs.IntVar(&parallel, "j", 4, "maximum parallel workers")
	fs.IntVar(&parallel, "parallel", 4, "maximum parallel workers (alias for -j)")
	fs.IntVar(&retries, "retries", 2, "retries after a failed task")
	fs.IntVar(&maxTokens, "max-tokens", 0, "total token budget (0 = unlimited)")
	fs.DurationVar(&taskTimeout, "task-timeout", 10*time.Minute, "timeout for one task")
	fs.StringVar(&workDir, "workdir", ".pysh-agent", "directory for runs/ and artifacts")
	fs.StringVar(&cwd, "cwd", "", "working directory for command workers")
	fs.StringVar(&shell, "shell", "", "shell for command workers (default: bash, else sh)")
	fs.BoolVar(&noSynth, "no-synthesize", false, "skip synthesis and keep combined worker outputs")
	fs.BoolVar(&quiet, "quiet", false, "suppress per-task progress")
	fs.BoolVar(&jsonOut, "json", false, "print the summary as JSON")
	pf.register(fs)

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "pysh-agent run: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}

	sources := 0
	for _, s := range []string{goal, planPath, resume} {
		if strings.TrimSpace(s) != "" {
			sources++
		}
	}
	if sources == 0 {
		fmt.Fprintln(os.Stderr, "pysh-agent run: one of -g GOAL, --plan PLAN.json, or --resume RUN is required")
		return 2
	}
	if sources > 1 {
		fmt.Fprintln(os.Stderr, "pysh-agent run: -g, --plan, and --resume are mutually exclusive")
		return 2
	}
	if planOnly && resume != "" {
		fmt.Fprintln(os.Stderr, "pysh-agent run: --plan-only cannot be combined with --resume")
		return 2
	}
	if parallel < 1 {
		fmt.Fprintln(os.Stderr, "pysh-agent run: -j must be at least 1")
		return 2
	}
	if retries < 0 {
		fmt.Fprintln(os.Stderr, "pysh-agent run: --retries cannot be negative")
		return 2
	}
	if maxTokens < 0 {
		fmt.Fprintln(os.Stderr, "pysh-agent run: --max-tokens cannot be negative")
		return 2
	}
	if taskTimeout <= 0 {
		fmt.Fprintln(os.Stderr, "pysh-agent run: --task-timeout must be positive")
		return 2
	}
	if worker != "api" && worker != "command" {
		fmt.Fprintln(os.Stderr, "pysh-agent run: --worker must be api or command")
		return 2
	}
	if worker == "command" && strings.TrimSpace(workerCmd) == "" {
		fmt.Fprintln(os.Stderr, "pysh-agent run: --worker-command is required with --worker=command")
		return 2
	}

	cfg := pf.config()
	cfg.Worker = worker
	cfg.WorkerCommand = workerCmd
	cfg.Parallel = parallel
	cfg.Retries = retries
	cfg.MaxTokens = maxTokens
	cfg.TaskTimeout = taskTimeout
	cfg.WorkDir = workDir
	cfg.Cwd = cwd
	cfg.Shell = shell
	cfg.NoSynthesize = noSynth

	runner, err := agent.NewRunner(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		plan  agent.Plan
		store *agent.Store
	)
	switch {
	case resume != "":
		store, err = agent.ResolveStore(workDir, resume)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
			return 2
		}
		plan, err = store.ReadPlan()
		if err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
			return 2
		}
		if err := agent.Validate(plan); err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: invalid plan in %s: %v\n", store.Dir, err)
			return 2
		}
	case planPath != "":
		plan, err = agent.LoadPlan(planPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
			return 2
		}
	case goal != "":
		plan, err = runner.Plan(ctx, goal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
			if ctx.Err() != nil {
				return 130
			}
			return 1
		}
	}

	if planOnly || dryRun {
		printPlan(plan)
		if planOut != "" {
			if err := writePlan(planOut, plan); err != nil {
				fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
				return 1
			}
			fmt.Fprintf(os.Stderr, "pysh-agent run: wrote %s\n", planOut)
		}
		return 0
	}

	if store == nil {
		store, err = agent.CreateStore(workDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
			return 1
		}
		if err := store.WritePlan(plan); err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: write plan: %v\n", err)
			return 1
		}
	}

	if planOut != "" {
		if err := writePlan(planOut, plan); err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
			return 1
		}
	}

	runner = withProgress(runner, quiet)
	summary, err := runner.Execute(ctx, store, plan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh-agent run: %v\n", err)
		return 1
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			fmt.Fprintf(os.Stderr, "pysh-agent run: encode summary: %v\n", err)
			return 1
		}
	} else {
		printSummary(summary)
	}

	if ctx.Err() != nil || summary.Status == "canceled" {
		return 130
	}
	if summary.Status != "success" {
		return 1
	}
	return 0
}

// withProgress installs a terminal progress callback. Agent.Config is copied
// by NewRunner, so the callback must be set before construction. This helper
// rebuilds a runner with the same configuration via a small wrapper.
func withProgress(r *agent.Runner, quiet bool) *agent.Runner {
	if quiet {
		return r
	}
	return r.WithProgress(func(res agent.TaskResult) {
		fmt.Fprintf(os.Stderr, "[%s] %-8s %6dms tokens=%d/%d %s\n",
			res.ID, res.Status, res.DurationMS,
			res.Usage.InputTokens, res.Usage.OutputTokens, res.Error)
	})
}

func printPlan(plan agent.Plan) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(plan)
}

func writePlan(path string, plan agent.Plan) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create plan directory: %w", err)
		}
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("encode plan: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	return nil
}

func printSummary(s agent.Summary) {
	fmt.Printf("\nrun  %s\n", s.RunID)
	fmt.Printf("dir  %s\n", s.RunDir)
	fmt.Printf("done %d tasks in %dms (ok=%d failed=%d skipped=%d)\n",
		s.Total, s.DurationMS, s.Succeeded, s.Failed, s.Skipped)
	if s.FinalPath != "" {
		fmt.Printf("final %s\n", s.FinalPath)
	}
	if s.Error != "" {
		fmt.Printf("error %s\n", s.Error)
	}
	for _, res := range s.Results {
		if res.Status == core.StatusSuccess {
			continue
		}
		fmt.Printf("  ! %s (%s): %s\n", res.ID, res.Status, res.Error)
	}
}
