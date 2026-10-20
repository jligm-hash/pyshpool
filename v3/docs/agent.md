# `pysh-agent` — parallel agentic work

`pysh-agent` decomposes a goal into a small directed acyclic graph (DAG), runs
independent tasks concurrently, passes dependency outputs to downstream tasks,
and synthesizes a final answer.

It is deliberately **not** a general agent framework. There is no chat loop, no
tool marketplace, and no hidden global state. A plan is a JSON document you can
review, edit, diff, and re-run.

## Pipeline

```text
goal
  │
  ▼
planner ──► plan.json (validated DAG)
  │
  ▼
scheduler ──► ready tasks in parallel ──► tasks/<id>/output.md
  │
  ▼
synthesizer ──► final.md
```

The planner, workers, and synthesizer all go through a small `Provider`
interface. The default provider is `mock`, which needs no API key and produces
deterministic output for demos and tests.

## Commands

```bash
pysh-agent plan -g GOAL [flags]       # print or write a plan
pysh-agent run  -g GOAL [flags]       # plan and execute
pysh-agent run  --plan PLAN.json      # execute an existing plan
pysh-agent run  --resume RUN          # resume a previous run
pysh-agent version
```

### Plan only

```bash
pysh-agent plan \
  -g "Compare three sorting algorithms" \
  --provider openai \
  --model gpt-4o-mini \
  --out plan.json
```

Review `plan.json` before spending tokens. `run --plan plan.json --dry-run`
prints the same validated plan without executing it.

### Execute and resume

```bash
pysh-agent run -g "Compare three sorting algorithms" --workdir .pysh-agent
pysh-agent run --resume <run-id> --workdir .pysh-agent
```

`--resume` accepts a run ID below `<workdir>/runs/` or a run directory path.
A task is reused only when its `meta.json` says `success` and its `output.md`
exists. Failed or missing tasks run again.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-g`, `--goal` | — | Goal to plan. |
| `--plan` | — | Existing plan JSON. |
| `--resume` | — | Run ID or directory to resume. |
| `--plan-only` | `false` | Plan and print; do not execute. |
| `--dry-run` | `false` | Same as plan-only for a run. |
| `--plan-out` | — | Also write the generated plan here. |
| `-j`, `--parallel` | `4` | Maximum concurrent tasks. |
| `--retries` | `2` | Retries per failed task. |
| `--task-timeout` | `10m` | Timeout for one task. |
| `--request-timeout` | `2m` | Timeout for one HTTP request. |
| `--max-output-tokens` | `2048` | Output cap per model call. |
| `--max-tokens` | `0` | Total input+output token budget; 0 is unlimited. |
| `--temperature` | `0.2` | Sampling temperature. |
| `--worker` | `api` | `api` or `command`. |
| `--worker-command` | — | Command template for command workers. |
| `--provider` | `mock` | `mock`, `openai`, or `anthropic`. |
| `--model` | provider default | Model name. |
| `--base-url` | provider default | API base URL. |
| `--api-key` | env | API key; prefer environment variables. |
| `--workdir` | `.pysh-agent` | Base for `runs/`. |
| `--cwd` | current dir | Working directory for command workers. |
| `--shell` | auto | Shell for command workers. |
| `--no-synthesize` | `false` | Skip synthesis; keep combined outputs. |
| `--quiet` | `false` | Suppress per-task progress. |
| `--json` | `false` | Print summary JSON to stdout. |

## Plan schema

```json
{
  "goal": "the original goal",
  "tasks": [
    {
      "id": "analyze-a",
      "title": "Analyze angle A",
      "prompt": "self-contained instructions for one worker",
      "depends_on": [],
      "acceptance": "optional acceptance check"
    }
  ]
}
```

Validation rules:

- non-empty goal;
- 1–32 tasks;
- IDs match `[A-Za-z0-9][A-Za-z0-9._-]{0,63}` and are unique;
- every dependency exists, is not self-referential, and is not duplicated;
- no cycles (Kahn's algorithm).

The planner output is untrusted. It is parsed, validated, and only then written
or executed.

## Run directory

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
        ├── meta.json
        ├── stdout.log
        └── stderr.log
```

- `plan.json` is the exact plan that was executed.
- `events.jsonl` records task completion, status, duration, and token usage.
- `summary.json` is the machine-readable result.
- `final.md` is the synthesized answer, or the combined worker outputs when
  synthesis is skipped.

## Failure semantics

- A failed task is retried up to `--retries`.
- Dependents of a failed task are marked `skipped`, not silently run.
- Synthesis runs only when every task succeeded, the budget was not exceeded,
  and the run was not canceled.
- If synthesis is skipped or fails, `final.md` contains the combined successful
  outputs and the summary status is `partial` or `failed`.
- The process exits 1 for `partial`/`failed`, 130 for cancellation, 0 for
  success.

## Token budget

`--max-tokens` is a total budget for planner, worker, and synthesis calls.
Usage is taken from each provider response. Once the total exceeds the budget,
no new tasks are started and the run is marked `partial`.

Budget accounting cannot stop a command worker in the middle of a shell
process, and command workers report zero tokens. Use per-task timeouts for
command workers.

## Safety

The default API worker is **text-only**. It does not run tools or edit files.
That is intentional: a text-only worker cannot accidentally delete a directory
because a model hallucinated a command.

Command workers do execute external programs. They run in `--cwd` with the
current user's permissions. Isolation via git worktrees is not implemented yet;
for code-editing agents, run one repository copy per task or wrap the command
in your own sandbox. See [Command workers](command-workers.md).

API keys are read from the environment by default. Do not put keys in a plan,
a task prompt, or a committed worker command.

## Why not a multi-agent framework?

Frameworks such as LangGraph, CrewAI, the OpenAI Agents SDK, the Claude Agent
SDK, and the Microsoft Agent Framework are useful when you need durable graphs,
human-in-the-loop checkpoints, role-based teams, or rich tool loops.

This use case is narrower: one planner, stateless workers, deterministic
dependencies, and one synthesizer. A small DAG scheduler over a provider
interface is enough, and it keeps the failure model visible. If v3 later needs
durable checkpoints or approval gates, the plan and store formats are designed
to be portable to a graph framework.
