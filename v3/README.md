# pyshpool v3

Two self-contained Go binaries for parallel work:

- **`pysh`** — run a flat list of complete shell commands locally, or submit the
  list as a native Slurm job array.
- **`pysh-agent`** — turn a goal into a small DAG, run the tasks in parallel,
  and synthesize the results.

`v3/` is additive. The original v2 scripts and binaries are untouched and
remain usable.

## Build

```bash
cd v3
make test
make build
```

Go 1.22 or newer is required. The binaries have no runtime dependencies; build
with `CGO_ENABLED=0` on Linux for a fully static executable.

## Try it

```bash
./dist/pysh run -i examples/tasks.dat -c 4
./dist/pysh-agent run -g "Explain why static binaries help HPC users"
```

The agent demo uses the deterministic `mock` provider, so it needs no API key.
Inspect `.pysh/runs/` and `.pysh-agent/runs/` for logs, summaries, and the
final agent result.

## Documentation

The full docs are in [`docs/`](docs/index.md) and are published with GitHub
Pages at <https://jligm-hash.github.io/pyshpool/>. The deployment workflow
expects **Settings → Pages → Source: GitHub Actions** to be selected once in
the repository settings.

Start with:

- [Quickstart](docs/quickstart.md)
- [`pysh` reference](docs/pysh.md)
- [Slurm arrays](docs/slurm.md)
- [`pysh-agent`](docs/agent.md)
- [Migration from v2](docs/migration.md)

## Layout

```text
v3/
├── cmd/pysh/           # fast shell runner
├── cmd/pysh-agent/     # planner/worker/synthesizer
├── internal/core/      # shared scheduler, logs, summaries
├── internal/slurm/     # job-array generator
├── internal/agent/     # plan, providers, DAG execution, store
├── examples/
├── scripts/
├── docs/
└── mkdocs.yml
```

## License

Apache-2.0, matching the repository.
