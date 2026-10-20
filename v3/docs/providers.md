# Providers

`pysh-agent` talks to providers through one interface:

```go
type Provider interface {
    Name() string
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}
```

The planner, workers, and synthesizer all use it. No provider SDK is required.

## `mock`

The default. It needs no key and returns deterministic plans, worker outputs,
and synthesis text. Use it for demos, tests, and CI.

```bash
pysh-agent run -g "Try the pipeline"
```

The mock plan has four tasks:

- `analyze-a` and `analyze-b` — independent;
- `draft` — depends on both;
- `review` — depends on `draft`.

## OpenAI-compatible

```bash
export OPENAI_API_KEY=...
pysh-agent run -g "..." --provider openai --model gpt-4o-mini
```

| Setting | Default |
|---|---|
| Base URL | `https://api.openai.com/v1` (or `OPENAI_BASE_URL`) |
| API key | `OPENAI_API_KEY` |
| Model | `gpt-4o-mini` |
| Endpoint | `POST <base>/chat/completions` |
| JSON mode | `response_format: {"type":"json_object"}` for planning |

An API key is required only when the base URL is the public OpenAI endpoint.
For a local server, an empty key is allowed:

```bash
pysh-agent run \
  -g "..." \
  --provider openai \
  --base-url http://localhost:11434/v1 \
  --model llama3.1
```

This works with servers that implement the OpenAI chat-completions API, such as
vLLM, Ollama, and LM Studio. Check that the server supports the model name and
the `response_format` field; if it does not, planning may return prose instead
of JSON.

Azure OpenAI uses a different URL shape and `api-version` query parameter. Use
an OpenAI-compatible gateway in front of it, or add a dedicated provider.

## Anthropic

```bash
export ANTHROPIC_API_KEY=...
pysh-agent run -g "..." --provider anthropic --model claude-3-5-sonnet-latest
```

| Setting | Default |
|---|---|
| Base URL | `https://api.anthropic.com` (or `ANTHROPIC_BASE_URL`) |
| API key | `ANTHROPIC_API_KEY` (required) |
| Model | `claude-3-5-sonnet-latest` |
| Endpoint | `POST <base>/v1/messages` |
| JSON mode | Appends “return only JSON” to the system prompt |

System messages are separated from user/assistant messages as the Anthropic
Messages API requires.

## Environment variables

| Variable | Used by |
|---|---|
| `OPENAI_API_KEY` | OpenAI-compatible provider |
| `OPENAI_BASE_URL` | OpenAI-compatible provider |
| `ANTHROPIC_API_KEY` | Anthropic provider |
| `ANTHROPIC_BASE_URL` | Anthropic provider |

Flags override environment variables. Prefer environment variables over
`--api-key`: command-line arguments are visible in process listings and shell
history.

## Timeouts and retries

- `--request-timeout` bounds one HTTP request (default 2m).
- `--task-timeout` bounds one worker task, including retries (default 10m).
- `--retries` retries failed tasks. Provider HTTP errors and empty responses
  are retryable; cancellation is not.

Provider HTTP errors include the status code and a bounded response body. API
keys are never included in error messages.

## Usage and cost

The summary records `input_tokens` and `output_tokens` reported by the
provider. `pysh-agent` does not embed a price table and does not bill you; use
`--max-tokens` as a coarse guard and your provider's dashboard for actual
costs.

## Model selection

Use a strong model for planning and synthesis, and a cheaper model for workers
if your provider supports two model names. The current CLI uses one `--model`
for all calls. Per-role model overrides are a planned enhancement.

## Failure behavior

- A missing key for a public endpoint is a startup error (exit 2).
- An HTTP error fails the task; the task is retried according to `--retries`.
- An empty response is treated as a failure.
- Planning output is stripped of markdown fences and surrounding prose before
  JSON parsing; invalid JSON is rejected with no execution.
