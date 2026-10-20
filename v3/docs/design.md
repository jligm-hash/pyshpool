# Design notes

## Two binaries, one shared core

`pysh` and `pysh-agent` are separate commands because their failure modes and
dependencies differ:

- `pysh` must start fast, stay small, and have no network code.
- `pysh-agent` needs HTTP providers, a plan store, and a DAG scheduler.

They share scheduling concepts — bounded concurrency, per-task logs, status
records, cancellation — but not a single bloated binary.

## Why Go, and why static

The v2 tool shipped a 16 MB PyInstaller bundle and shc-wrapped binaries. Those
solved “different users have different environments” only partially:

- `pyshNode4-static` requires glibc 2.34, so it fails on common older HPC
  images;
- the shc payload is opaque and cannot be reviewed or patched;
- there is no build recipe tying the binary to its source.

Go with `CGO_ENABLED=0` produces a single executable with no libc dependency on
Linux. macOS builds are also self-contained Go binaries. The source is the
distribution; stripping symbols and publishing checksums are optional
conveniences, not obfuscation.

## Task = one complete shell line

The input format is the product's main convenience. A line is not tokenized or
reassembled; it is passed to `bash -c` exactly as written. This preserves
redirections, pipes, quotes, and inline environment variables.

The scheduler never uses `eval`. It starts one subprocess per attempt in a new
process group, streams output to a file, and enforces a timeout by killing the
group.

## Native Slurm arrays instead of a splitter

The v2 splitter existed to give each node a chunk of work. Slurm already has a
primitive for “one task per input”: the job array. Mapping array index to line
number removes the chunk arithmetic, the duplicated-tail bug, and the generated
per-node files.

The trade-off is per-element scheduling overhead, which is why the docs discuss
grouping small tasks and using `pysh run` inside one allocation.

## Agent design

The agent pipeline is intentionally narrow:

- **Planner** produces a JSON DAG, not free-form conversation.
- **Validator** enforces IDs, dependencies, and acyclicity before execution.
- **Scheduler** runs ready nodes concurrently and injects only dependency
  outputs into downstream prompts.
- **Store** writes every prompt, output, metadata file, event, and summary.
- **Synthesizer** runs only when all tasks succeeded and the budget allows.

Text-only API workers are the default because they cannot execute tools. This
is a safety boundary, not a limitation to paper over. Command workers are the
escape hatch, with the caller responsible for isolation.

The mock provider is a first-class test fixture: it makes the entire pipeline
deterministic and runnable in CI without keys or network access.

## Failure model

- A task is `success`, `failed`, `timeout`, `canceled`, or `skipped`.
- Dependents of a failed task are skipped, never run with missing context.
- Synthesis is skipped on failure; `final.md` falls back to combined successful
  outputs.
- A run is `success` only when every task succeeded and synthesis (if enabled)
  succeeded.
- The process exit code reflects the run status.

## Non-goals

- General workflow/DAG engine with cron, sensors, and artifacts.
- Distributed scheduler outside Slurm.
- Tool-calling agent loop with a permission system.
- GUI or web dashboard.
- Replacing GNU parallel for SSH fan-out.

## Future work

- Per-role model selection (planner/worker/synthesizer).
- Git-worktree isolation for command workers.
- Optional tool allowlist and approval gates.
- GNU parallel backend for `pysh run --engine parallel`.
- Release binaries for Linux/macOS, checksums, and a signed build.
- Benchmarks against more task shapes (I/O-bound, CPU-bound, long-tail).
