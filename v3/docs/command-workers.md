# Command workers

API workers are text-only. A **command worker** lets you plug in any executable
that reads a prompt and writes a result — including an existing agent CLI.

```bash
pysh-agent run \
  --plan plan.json \
  --worker command \
  --worker-command 'my-agent --prompt {{prompt_file}} --out {{output_file}}' \
  --no-synthesize
```

## Contract

For each task, `pysh-agent` prepares:

```text
tasks/<id>/
├── prompt.txt
├── stdout.log
└── stderr.log
```

It then runs the expanded command through the shell with:

- the prompt on **stdin**;
- `PYSH_PROMPT_FILE`, `PYSH_OUTPUT_FILE`, `PYSH_TASK_ID`, and `PYSH_GOAL` in
  the environment;
- stdout and stderr redirected to the log files above.

The worker must produce a non-empty result in one of two places:

1. write it to `{{output_file}}` (recommended), or
2. print it to stdout.

If the output file exists and is non-empty, it wins. If the command exits
non-zero, the task fails even if the output file was written.

## Placeholders

| Placeholder | Replaced with |
|---|---|
| `{{prompt_file}}` | Absolute path to `prompt.txt` |
| `{{output_file}}` | Absolute path to `output.md` |
| `{{task_id}}` | Task ID |
| `{{goal}}` | The plan goal |

Values are shell-quoted before substitution, so paths containing spaces are
safe. The expanded command is passed to `bash -c` (or the `--shell` you set).

## Environment

| Variable | Meaning |
|---|---|
| `PYSH_PROMPT_FILE` | Prompt path |
| `PYSH_OUTPUT_FILE` | Output path |
| `PYSH_TASK_ID` | Task ID |
| `PYSH_GOAL` | Plan goal |

## Example worker

[`examples/echo-worker.sh`](https://github.com/jligm-hash/pyshpool/blob/main/v3/examples/echo-worker.sh) reads stdin and writes
a small Markdown result:

```bash
#!/usr/bin/env bash
set -euo pipefail

prompt="$(cat)"
{
  printf '# Command worker result\n\n'
  printf 'Task: `%s`\n\n' "${PYSH_TASK_ID:-unknown}"
  printf '%s\n' "$prompt"
} > "${PYSH_OUTPUT_FILE:?PYSH_OUTPUT_FILE is not set}"
```

Run it against the example plan:

```bash
pysh-agent run \
  --plan examples/plan.json \
  --worker command \
  --worker-command 'bash examples/echo-worker.sh' \
  --no-synthesize \
  --workdir .pysh-agent-command
```

## Dependency outputs

For a task with dependencies, the prompt file contains the goal, the task
instructions, and the dependency outputs (truncated if very large). A command
worker receives the same assembled prompt as an API worker.

## Synthesis

Synthesis uses the configured **provider**, not the command worker. With the
default `mock` provider, that would produce mock text. For a fully local
command-worker pipeline, pass `--no-synthesize`; `final.md` then contains the
combined successful outputs.

## Isolation and safety

Command workers run with your user permissions in `--cwd`. They are not
sandboxed.

- Do not run untrusted prompts against a command worker.
- Use one repository copy or git worktree per concurrent code-editing worker.
- Keep credentials out of the command string and prompt.
- Prefer `--no-synthesize` if no API key is configured.
- Set `--task-timeout` to bound a stuck worker.

Git-worktree isolation and a tool allowlist are planned enhancements. Until
then, isolation is the caller's responsibility.

## Adapting an agent CLI

Any CLI that can read a prompt file and write an output file fits the contract:

```bash
# illustrative shape, not a guarantee of every CLI's flags
my-coding-agent \
  --non-interactive \
  --prompt-file {{prompt_file}} \
  --output {{output_file}}
```

Check your CLI's own flags and sandbox behavior. If it edits files, give each
worker its own checkout; otherwise concurrent workers will overwrite each
other.
