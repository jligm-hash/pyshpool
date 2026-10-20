#!/usr/bin/env bash
# Example command worker for pysh-agent --worker command.
#
# The prompt arrives on stdin and in $PYSH_PROMPT_FILE. This worker writes its
# result to $PYSH_OUTPUT_FILE; printing the result on stdout also works.
set -euo pipefail

prompt="$(cat)"
{
  printf '# Command worker result\n\n'
  printf 'Task: `%s`\n\n' "${PYSH_TASK_ID:-unknown}"
  printf 'Received prompt:\n\n'
  printf '%s\n' "$prompt"
} > "${PYSH_OUTPUT_FILE:?PYSH_OUTPUT_FILE is not set}"
