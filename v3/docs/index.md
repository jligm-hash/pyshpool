# pyshpool

**Run independent work in parallel — and see exactly what happened.**

`pyshpool` is a small, dependency-free toolkit with two separate binaries:

<div class="grid cards" markdown>

-   :material-console: **`pysh` — fast shell runner**

    Feed it one complete shell command per line. It runs them with bounded
    concurrency, captures a log per task, handles timeouts and retries, and
    writes a machine-readable summary. On Slurm it generates a native job array
    instead of splitting your input file.

    [:octicons-arrow-right-24: `pysh` reference](pysh.md)

-   :material-robot: **`pysh-agent` — parallel agentic work**

    Turn a goal into a small JSON DAG, run independent tasks in parallel, pass
    outputs along dependency edges, and synthesize a final result. The default
    `mock` provider makes the whole pipeline runnable without an API key.

    [:octicons-arrow-right-24: `pysh-agent` reference](agent.md)

</div>

## Why another parallel runner?

The original v2 tools solved a real problem, but they accumulated avoidable
failure modes: fixed filenames in the current directory, silent job failures,
a splitter that duplicates the final task when nodes outnumber tasks, an empty
`#SBATCH -A` directive, and opaque binaries that depend on a specific glibc.

v3 starts from the same convenience — **one command per line** — and adds:

- **No fixed files in your working directory.** Every run gets
  `.pysh/runs/<timestamp>-<id>/` or `.pysh-agent/runs/...`.
- **No silent failures.** Exit codes, timeouts, retries, and cancellation are
  recorded; the process exits non-zero when work fails.
- **No custom Slurm splitter.** One array element maps to one input line, so
  there is no duplicated tail and no generated `tmp.git`.
- **No opaque binaries.** Go source, standard library only, reproducible
  builds, and self-contained executables.
- **A separate agent runner.** Parallel shell jobs and LLM/agent fan-out have
  different failure modes; they are separate commands but share the same
  scheduler ideas.

## 30-second demo

```bash
cd v3
make build

./dist/pysh run -i examples/tasks.dat -c 4
```

```text
[1/4] success  exit=0      8ms  mkdir -p demo-out && echo "hello from task 1" ...
...
run 20261020-064815-57e86a
dir /.../.pysh/runs/20261020-064815-57e86a
done 4/4 in 15ms (ok=4 failed=0 timeout=0 canceled=0 skipped=0)
```

Agent mode needs no key with the default mock provider:

```bash
./dist/pysh-agent run -g "Explain why static binaries help HPC users"
```

```text
[analyze-a] success   0ms tokens=160/52
[analyze-b] success   0ms tokens=171/52
[draft]     success   0ms tokens=284/52
[review]    success   0ms tokens=225/51

run  20261020-064837-a3ad97
final /.../.pysh-agent/runs/20261020-064837-a3ad97/final.md
```

## Design in one paragraph

`pysh` treats each non-comment line as a complete `bash -c` command. A single
scheduler process starts subprocesses in their own process groups, streams
their output to per-task logs, and enforces a semaphore. `pysh-agent` uses the
same idea at a higher level: a planner emits a validated DAG, ready nodes run
concurrently, dependency outputs are injected into worker prompts, and a
synthesizer merges the leaves. Both write an append-only event log and a JSON
summary so a run can be audited or resumed.

## Status

v3 is under active development. The v2 files are deliberately untouched; see
[Migration from v2](migration.md) for the behavioral differences and the
compatibility flags.

## License

Apache-2.0.
