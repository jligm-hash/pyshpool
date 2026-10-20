# Migration from v2

v3 is additive. No v2 file is edited, moved, renamed, or deleted. You can keep
using the old tools while evaluating the new binaries.

## At a glance

| Area | v2 | v3 |
|---|---|---|
| Runtime | Python 3 + Bash + frozen/shc binaries | Two self-contained Go binaries |
| Portability | `pyshNode4-static` needs glibc 2.34; obfuscated payloads | `CGO_ENABLED=0` Linux builds; readable source |
| Local pool | `shellPoolv2.2`, `poolRunShell-v2.2` | `pysh run` |
| Multi-node | Split files + a pool per node | `pysh submit` native Slurm array |
| Fixed filenames | `fd1`, `tmp.git`, `inpDL*.out`, `logNode*.out` | Per-run directories under `.pysh/` |
| Failure reporting | Jobs often unchecked; exit 0 on failure | Per-task status, logs, JSON summary, non-zero exit |
| Empty account | `#SBATCH -A` with no value | Directive omitted |
| Split edge case | Last task duplicated when nodes > tasks | No splitter |
| Agentic mode | — | `pysh-agent` planner/worker/synthesizer |

## Command mapping

### Local pool

```bash
# v2
bash shellPoolv2.2 -c 8 -i tasks.dat

# v3
pysh run -i tasks.dat -c 8
```

`-i` and `-c` are intentionally compatible. Unlike v2, a missing or empty task
file is an error, and failed jobs make the process exit 1.

### Slurm

```bash
# v2
./pyshpool-multiNodev2.7.1-static.sh -N 4 -n 32 -i tasks.dat

# v3
pysh submit -i tasks.dat -N 1 -n 32 --array-limit 4 --submit
```

In v3, `-N` is nodes **per array element** and is almost always 1. Concurrency
across elements is controlled by `--array-limit` (`%K`), not by a hand-written
splitter. `-n` is CPUs per element.

### Python runner

```bash
# v2
./poolRunShell-static -i tasks.dat -c 8

# v3
pysh run -i tasks.dat -c 8
```

v3 does not use `os.system`, does not print every line to stdout, and records
the actual exit status of each task.

## Behavioral differences

- **No `eval`.** Commands are passed to `bash -c` as a single string, so quoting
  and globbing behave like a shell script line.
- **No token-FIFO deadlock.** A job containing `exec` or `exit` cannot prevent
  the scheduler from making progress because there is no worker-side token to
  return.
- **No CWD pollution.** Logs and summaries go under `--workdir`; generated
  Slurm scripts go under `.pysh/submits/`.
- **Timeouts kill the process group**, including pipelines and child
  processes.
- **Retries are explicit.** `--retries N` applies to failed exits and start
  errors; timeouts and cancellation are not retried.
- **Exit codes are meaningful.** `1` for task failures, `2` for usage/input
  errors, `130` for interruption.
- **`-c max` is supported** and maps to `runtime.NumCPU()`. Inside Slurm,
  prefer an explicit count.

## What v3 intentionally does not do

- It does not replace GNU parallel's SSH fan-out or `.joblog` resume.
- It does not manage Python environments, modules, or containers.
- It does not split a task file across nodes; use a job array.
- It does not run a general agent framework or tool sandbox.
- It does not modify the v2 files. Old behavior remains available for
  comparison.

## Trying both safely

Run v3 from `v3/` so its default `.pysh/`, `.pysh-agent/`, and `demo-out/`
directories stay inside the new folder:

```bash
cd v3
make build
./dist/pysh run -i examples/tasks.dat -c 4
```

The v2 `inpTaskList.dat` refers to `demoScript.sh`, which is not present in the
repository. v3 ships its own `examples/tasks.dat` instead; the old file is
untouched.
