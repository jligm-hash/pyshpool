// Command pysh runs shell tasks in parallel or generates Slurm job arrays.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jligm-hash/pyshpool/v3/internal/core"
	"github.com/jligm-hash/pyshpool/v3/internal/slurm"
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
	case "run":
		return cmdRun(args[1:])
	case "submit":
		return cmdSubmit(args[1:])
	case "version", "--version", "-version":
		fmt.Println(version.String())
		return 0
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "pysh: unknown command %q\n\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `pysh - fast parallel shell runner and Slurm submitter

Usage:
  pysh run    -i TASKS [flags]     run commands locally in parallel
  pysh submit -i TASKS [flags]     generate or submit a Slurm job array
  pysh version

Run "pysh run -h" or "pysh submit -h" for flags.
`)
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("pysh run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		input    string
		conc     string
		timeout  time.Duration
		retries  int
		failFast bool
		workDir  string
		shell    string
		cwd      string
		quiet    bool
		jsonOut  bool
		dryRun   bool
		env      stringList
	)
	fs.StringVar(&input, "i", "", "task file (one shell command per line)")
	fs.StringVar(&input, "input", "", "task file (alias for -i)")
	fs.StringVar(&conc, "c", "0", "workers, or max for all CPUs")
	fs.StringVar(&conc, "concurrency", "0", "workers (alias for -c)")
	fs.DurationVar(&timeout, "timeout", 0, "per-task timeout, e.g. 10m (0 = none)")
	fs.IntVar(&retries, "retries", 0, "retries after a failed attempt")
	fs.BoolVar(&failFast, "fail-fast", false, "stop starting tasks after the first failure")
	fs.StringVar(&workDir, "workdir", ".pysh", "directory for runs/ logs and summaries")
	fs.StringVar(&shell, "shell", "", "shell to execute commands (default: bash, else sh)")
	fs.StringVar(&cwd, "cwd", "", "working directory for tasks")
	fs.BoolVar(&quiet, "quiet", false, "suppress per-task progress")
	fs.BoolVar(&jsonOut, "json", false, "print the summary as JSON")
	fs.BoolVar(&dryRun, "dry-run", false, "print commands without running them")
	fs.Var(&env, "env", "extra KEY=VALUE for tasks (repeatable)")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if input == "" && fs.NArg() == 1 {
		input = fs.Arg(0)
	} else if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "pysh run: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "pysh run: -i TASKS is required")
		return 2
	}
	if timeout < 0 {
		fmt.Fprintln(os.Stderr, "pysh run: --timeout cannot be negative")
		return 2
	}
	if retries < 0 {
		fmt.Fprintln(os.Stderr, "pysh run: --retries cannot be negative")
		return 2
	}
	for _, e := range env {
		if !strings.Contains(e, "=") {
			fmt.Fprintf(os.Stderr, "pysh run: --env value %q is not KEY=VALUE\n", e)
			return 2
		}
	}

	concurrency, err := parseConcurrency(conc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh run: %v\n", err)
		return 2
	}

	tasks, err := core.ReadTasks(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh run: %v\n", err)
		return 2
	}

	if dryRun {
		for _, t := range tasks {
			fmt.Printf("%d\t%s\n", t.Index, t.Command)
		}
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := core.NewRunner(core.Config{
		Concurrency: concurrency,
		Timeout:     timeout,
		Retries:     retries,
		FailFast:    failFast,
		Shell:       shell,
		WorkDir:     workDir,
		Cwd:         cwd,
		Env:         env,
		Progress: func(res core.Result) {
			if quiet {
				return
			}
			line := fmt.Sprintf("[%d/%d] %-8s exit=%d %6dms  %s",
				res.Index, len(tasks), res.Status, res.ExitCode, res.DurationMS,
				core.FormatCommand(res.Command, 80))
			fmt.Fprintln(os.Stderr, line)
		},
	})

	summary, err := runner.Run(ctx, tasks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh run: %v\n", err)
		return 1
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			fmt.Fprintf(os.Stderr, "pysh run: encode summary: %v\n", err)
			return 1
		}
	} else {
		printSummary(summary)
	}

	if ctx.Err() != nil {
		return 130
	}
	if summary.Failed > 0 || summary.TimedOut > 0 || summary.Canceled > 0 {
		return 1
	}
	return 0
}

func parseConcurrency(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" || s == "max" {
		return runtime.NumCPU(), nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("-c must be a positive integer or max, got %q", s)
	}
	return n, nil
}

func printSummary(s core.Summary) {
	fmt.Printf("\nrun %s\n", s.RunID)
	fmt.Printf("dir  %s\n", s.RunDir)
	fmt.Printf("done %d/%d in %dms (ok=%d failed=%d timeout=%d canceled=%d skipped=%d)\n",
		s.Succeeded+s.Failed+s.TimedOut+s.Canceled+s.Skipped, s.Total, s.DurationMS,
		s.Succeeded, s.Failed, s.TimedOut, s.Canceled, s.Skipped)
	for _, res := range s.Results {
		if res.Success() {
			continue
		}
		fmt.Printf("  ! task %d (%s): %s", res.Index, res.Status, res.Error)
		if res.LogPath != "" {
			fmt.Printf(" [%s]", res.LogPath)
		}
		fmt.Println()
	}
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func cmdSubmit(args []string) int {
	fs := flag.NewFlagSet("pysh submit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		input      string
		jobName    string
		partition  string
		account    string
		nodes      int
		cpus       int
		array      string
		arrayLimit int
		timeLimit  string
		memory     string
		gres       string
		nodeList   string
		exclude    string
		mailType   string
		mailUser   string
		workDir    string
		output     string
		errorPath  string
		scriptOut  string
		submit     bool
		dryRun     bool
		quiet      bool
		extra      stringList
	)
	fs.StringVar(&input, "i", "", "task file (one shell command per line)")
	fs.StringVar(&input, "input", "", "task file (alias for -i)")
	fs.StringVar(&jobName, "J", "pysh", "Slurm job name")
	fs.StringVar(&jobName, "job-name", "pysh", "Slurm job name (alias for -J)")
	fs.StringVar(&partition, "p", "", "Slurm partition")
	fs.StringVar(&partition, "partition", "", "Slurm partition (alias for -p)")
	fs.StringVar(&account, "A", "", "Slurm account (omitted when empty)")
	fs.StringVar(&account, "account", "", "Slurm account (alias for -A)")
	fs.IntVar(&nodes, "N", 1, "nodes per array element")
	fs.IntVar(&nodes, "nodes", 1, "nodes per array element (alias for -N)")
	fs.IntVar(&cpus, "n", 1, "CPUs per task")
	fs.IntVar(&cpus, "cpus-per-task", 1, "CPUs per task (alias for -n)")
	fs.StringVar(&array, "array", "", "Slurm array expression (default: 1-<lines>)")
	fs.IntVar(&arrayLimit, "array-limit", 0, "max concurrent array elements (%%K)")
	fs.StringVar(&timeLimit, "time", "", "Slurm time limit, e.g. 04:00:00")
	fs.StringVar(&memory, "mem", "", "Slurm memory, e.g. 8G")
	fs.StringVar(&gres, "gres", "", "Slurm generic resources, e.g. gpu:1")
	fs.StringVar(&nodeList, "w", "", "Slurm node list")
	fs.StringVar(&nodeList, "nodelist", "", "Slurm node list (alias for -w)")
	fs.StringVar(&exclude, "exclude", "", "Slurm nodes to exclude")
	fs.StringVar(&mailType, "mail-type", "", "Slurm mail type")
	fs.StringVar(&mailUser, "mail-user", "", "Slurm mail user")
	fs.StringVar(&workDir, "workdir", "", "working directory (default: current directory)")
	fs.StringVar(&output, "output", "pysh-%A_%a.out", "Slurm output pattern")
	fs.StringVar(&errorPath, "error", "", "Slurm error pattern (default: same as --output)")
	fs.StringVar(&scriptOut, "script-out", "", "write the script to this path")
	fs.BoolVar(&submit, "submit", false, "submit with sbatch")
	fs.BoolVar(&dryRun, "dry-run", false, "print the script instead of submitting")
	fs.BoolVar(&quiet, "quiet", false, "suppress status messages")
	fs.Var(&extra, "sbatch-opt", "extra #SBATCH option (repeatable)")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if input == "" && fs.NArg() == 1 {
		input = fs.Arg(0)
	} else if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "pysh submit: unexpected arguments: %s\n", strings.Join(fs.Args(), " "))
		return 2
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "pysh submit: -i TASKS is required")
		return 2
	}
	if submit && dryRun {
		fmt.Fprintln(os.Stderr, "pysh submit: --submit and --dry-run are mutually exclusive")
		return 2
	}
	if nodes < 1 {
		fmt.Fprintln(os.Stderr, "pysh submit: -N must be at least 1")
		return 2
	}
	if cpus < 0 {
		fmt.Fprintln(os.Stderr, "pysh submit: -n cannot be negative")
		return 2
	}
	if arrayLimit < 0 {
		fmt.Fprintln(os.Stderr, "pysh submit: --array-limit cannot be negative")
		return 2
	}

	script, absTask, err := slurm.Generate(slurm.Config{
		TaskFile:     input,
		JobName:      jobName,
		Partition:    partition,
		Account:      account,
		Nodes:        nodes,
		CPUsPerTask:  cpus,
		Array:        array,
		ArrayLimit:   arrayLimit,
		TimeLimit:    timeLimit,
		Memory:       memory,
		GRES:         gres,
		NodeList:     nodeList,
		Exclude:      exclude,
		MailType:     mailType,
		MailUser:     mailUser,
		WorkDir:      workDir,
		Output:       output,
		Error:        errorPath,
		ExtraOptions: extra,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pysh submit: %v\n", err)
		return 2
	}
	if !quiet {
		fmt.Fprintf(os.Stderr, "pysh submit: task file %s\n", absTask)
	}

	if scriptOut != "" {
		if err := os.MkdirAll(filepath.Dir(scriptOut), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "pysh submit: create script directory: %v\n", err)
			return 1
		}
		if err := os.WriteFile(scriptOut, []byte(script), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "pysh submit: write script: %v\n", err)
			return 1
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "pysh submit: wrote %s\n", scriptOut)
		}
	}

	if !submit {
		if scriptOut == "" || dryRun {
			fmt.Print(script)
		}
		return 0
	}

	path := scriptOut
	if path == "" {
		dir := filepath.Join(".pysh", "submits", time.Now().UTC().Format("20060102-150405"))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "pysh submit: create submit directory: %v\n", err)
			return 1
		}
		path = filepath.Join(dir, "submit.sbatch")
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "pysh submit: write script: %v\n", err)
			return 1
		}
	}

	cmd := exec.Command("sbatch", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "pysh submit: sbatch failed: %v\n", err)
		return 1
	}
	return 0
}
