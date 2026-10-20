# Quickstart

## Prerequisites

- Go 1.22 or newer to build from source.
- A POSIX shell (`bash` is preferred; `sh` is the fallback).
- Optional: Slurm (`sbatch`) for `pysh submit`.
- Optional: an API key for `pysh-agent` with a real provider. The default
  `mock` provider needs nothing.

## Build

```bash
git clone https://github.com/jligm-hash/pyshpool.git
cd pyshpool/v3
make test
make build
```

The binaries are written to `dist/pysh` and `dist/pysh-agent`. To build a
fully static Linux binary:

```bash
GOOS=linux GOARCH=amd64 ./scripts/build.sh
```

`CGO_ENABLED=0` is set by the build script, so the result does not depend on
the target machine's libc.

## Run shell tasks locally

```bash
./dist/pysh run -i examples/tasks.dat -c 4
```

Each non-empty, non-comment line is one command. The example writes four files
under `demo-out/` and creates a run directory:

```text
.pysh/runs/<timestamp>-<id>/
├── summary.json
├── events.jsonl
└── tasks/
    ├── 0001.log
    ├── 0002.log
    └── ...
```

Exit status:

- `0` — every task succeeded.
- `1` — at least one task failed, timed out, or was canceled.
- `2` — usage or input error.
- `130` — interrupted with Ctrl-C.

Inspect the summary:

```bash
jq . .pysh/runs/*/summary.json
```

Use `--json` to print the same object to stdout.

### A task file with real commands

```bash
mkdir -p logs
for i in $(seq 1 100); do
  echo "python analyze.py --shard $i > logs/shard-$i.log 2>&1"
done > tasks.dat

./dist/pysh run -i tasks.dat -c 16 --timeout 30m --retries 1
```

Each task gets its own log under `.pysh/runs/.../tasks/`. The command's own
redirections still apply; the runner's log captures whatever does not go to a
file.

## Submit a Slurm array

`pysh submit` does not split the task file. It generates a native job array
where array element `N` reads line `N`:

```bash
./dist/pysh submit \
  -i tasks.dat \
  -J demo \
  -p general \
  -A my-account \
  -N 1 -n 8 \
  --array-limit 50 \
  --time 01:00:00 \
  --mem 8G \
  --submit
```

Omit `--submit` to print the script instead. `-A` is only emitted when it is
non-empty. See [Slurm arrays](slurm.md) for the details.

## Run the agent demo

```bash
./dist/pysh-agent run -g "Explain why static binaries help HPC users"
```

The default provider is `mock`; it produces a deterministic four-task plan
(`analyze-a`, `analyze-b`, `draft`, `review`) and deterministic worker output.
The run directory contains:

```text
.pysh-agent/runs/<timestamp>-<id>/
├── plan.json
├── events.jsonl
├── summary.json
├── final.md
└── tasks/
    └── analyze-a/
        ├── prompt.txt
        ├── output.md
        └── meta.json
```

Plan without executing:

```bash
./dist/pysh-agent plan -g "Compare three sorting algorithms" --out plan.json
./dist/pysh-agent run --plan plan.json --dry-run
```

Resume a previous run (successful tasks are reused):

```bash
./dist/pysh-agent run --resume <run-id> --workdir .pysh-agent
```

## Use a real provider

OpenAI-compatible:

```bash
export OPENAI_API_KEY=...
./dist/pysh-agent run \
  -g "Write a short design note" \
  --provider openai \
  --model gpt-4o-mini
```

Anthropic:

```bash
export ANTHROPIC_API_KEY=...
./dist/pysh-agent run \
  -g "Write a short design note" \
  --provider anthropic \
  --model claude-3-5-sonnet-latest
```

Local OpenAI-compatible server (vLLM, Ollama, LM Studio):

```bash
./dist/pysh-agent run \
  -g "Write a short design note" \
  --provider openai \
  --base-url http://localhost:8000/v1 \
  --model my-local-model
```

See [Providers](providers.md) for environment variables, defaults, and
limitations.

## Publishing the documentation

The repository includes `.github/workflows/v3-pages.yml`, which builds
`v3/docs` with MkDocs Material and deploys it with GitHub Pages. It runs when
docs, `v3/mkdocs.yml`, or the workflow itself changes on `main`, and can also
be started manually.

One-time setup: in the repository, open **Settings → Pages → Build and
deployment**, and set **Source** to **GitHub Actions**. The expected site URL
is <https://jligm-hash.github.io/pyshpool/>.

Preview locally:

```bash
python3 -m venv .venv-docs
. .venv-docs/bin/activate
pip install -r docs/requirements.txt
mkdocs serve
```

## Next steps

- [`pysh` reference](pysh.md)
- [Slurm arrays](slurm.md)
- [`pysh-agent`](agent.md)
- [Command workers](command-workers.md)
- [Benchmarks](benchmarks.md)
