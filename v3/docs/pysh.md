# `pysh` — fast parallel shell runner

`pysh` reads a file of complete shell commands and runs them concurrently with
a fixed number of workers. It is designed for batches of independent jobs where
each command already knows its own inputs and outputs.

## Task file format

- One command per line.
- Blank lines are ignored.
- Lines whose first non-space character is `#` are ignored.
- Everything else is passed unchanged to the shell, including quotes,
  pipes, redirections, and trailing whitespace.
- CRLF line endings are accepted.

```bash
# tasks.dat
python analyze.py --shard 1 > logs/1.log 2>&1
python analyze.py --shard 2 > logs/2.log 2>&1
mkdir -p results && cp input.dat results/input.dat
```

Commands run with `bash -c` when `bash` is available, otherwise `sh -c`.

## Usage

```bash
pysh run -i TASKS [flags]
pysh run TASKS [flags]          # positional input also works
```

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-i`, `--input` | — | Task file. Required unless passed positionally. |
| `-c`, `--concurrency` | CPU count | Number of workers. `max` or `0` means all CPUs. |
| `--timeout` | `0` | Per-attempt timeout, e.g. `30m`. `0` disables it. |
| `--retries` | `0` | Extra attempts after a failed exit or start error. |
| `--fail-fast` | `false` | Cancel the run after the first unsuccessful task. |
| `--workdir` | `.pysh` | Base directory for `runs/`. |
| `--cwd` | current directory | Working directory for each command. Must exist. |
| `--shell` | auto | Shell executable. |
| `--env` | — | Extra `KEY=VALUE` for tasks. Repeatable. |
| `--quiet` | `false` | Suppress per-task progress lines. |
| `--json` | `false` | Print the summary as JSON to stdout. |
| `--dry-run` | `false` | Print parsed commands without running them. |

`-c 0`, `-c max`, and an omitted `-c` all mean “use `runtime.NumCPU()`”.
On a Slurm allocation, `NumCPU` may report the host's CPUs rather than the
allocation, so pass an explicit `-c` inside batch jobs.

### Old flag compatibility

The v2 Python runner accepted `-i` and `-c`; both work here with the same
meaning. A missing task file or an empty list is a hard error (exit 2), not a
silent “0 jobs” success.

## What happens to a task

For each command, `pysh`:

1. Starts the shell in a new process group.
2. Redirects stdout and stderr to `tasks/<index>.log`.
3. Waits for the process, enforcing `--timeout`.
4. On timeout or cancellation, sends `SIGKILL` to the whole process group, so
   pipelines and child processes do not survive.
5. Records status, exit code, attempts, timestamps, and duration.

Retries apply to non-zero exits and start errors. A timeout or a canceled run
is not retried.

## Run directory

```text
.pysh/runs/20261020-064815-57e86a/
├── summary.json
├── events.jsonl
└── tasks/
    ├── 0001.log
    └── 0002.log
```

`summary.json` contains the run ID, directory, counts, and one result per
task. `events.jsonl` is append-only and convenient for streaming into another
tool.

Example summary excerpt:

```json
{
  "run_id": "20261020-064815-57e86a",
  "total": 4,
  "succeeded": 3,
  "failed": 1,
  "timed_out": 0,
  "canceled": 0,
  "skipped": 0,
  "results": [
    {
      "index": 1,
      "line": 1,
      "command": "echo one",
      "status": "success",
      "exit_code": 0,
      "attempts": 1,
      "duration_ms": 8,
      "log_path": ".../tasks/0001.log"
    }
  ]
}
```

## Progress and output

Progress goes to stderr, one line per completed task:

```text
[2/4] success  exit=0      8ms  mkdir -p demo-out && echo ...
[4/4] failed   exit=1     12ms  python missing.py
```

The final summary goes to stdout. `--json` replaces the human summary with
JSON; progress and errors remain on stderr, so stdout can be piped safely.

## Examples

### Limit concurrency and add timeouts

```bash
pysh run -i tasks.dat -c 8 --timeout 20m --retries 2
```

### Run in a scratch directory

```bash
pysh run -i tasks.dat --cwd /scratch/$USER/batch --workdir /scratch/$USER/pysh-logs
```

`--cwd` must already exist; `--workdir` is created.

### Pass environment variables

```bash
pysh run -i tasks.dat --env OMP_NUM_THREADS=1 --env CUDA_VISIBLE_DEVICES=0
```

### Fail fast

```bash
pysh run -i tasks.dat --fail-fast
```

Running tasks are canceled (their process groups are killed), and tasks that
never started are recorded as `skipped`.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | All tasks succeeded. |
| `1` | At least one task failed, timed out, or was canceled. |
| `2` | Usage error, missing/empty task file, invalid flag. |
| `130` | Interrupted by SIGINT/SIGTERM. |

A failure is never silent: the summary counts it, the process exits 1, and the
per-task log path is printed.

## Security note

Per-task logs contain the full command line and all stdout/stderr that was not
redirected elsewhere. If a command embeds a token or password, it will be
written to `.pysh/runs/.../tasks/*.log`. Keep secrets in environment variables
or files with restricted permissions, and treat the run directory accordingly.

## Relationship to GNU parallel and xargs

`pysh` is intentionally small. GNU parallel remains the better choice for very
large batches, SSH fan-out, job logs with resume, and complex retry policies.
`xargs -P` remains the zero-install minimum. `pysh` focuses on:

- complete shell lines with redirections,
- predictable per-task logs and a JSON summary,
- process-group cleanup,
- a native Slurm path that does not split files.

If you need GNU parallel semantics, use GNU parallel. If you need the same
input file to work locally and as a Slurm array, `pysh` is the bridge.
