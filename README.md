# needle-controller

A compact OpenAI-compatible tool-calling service backed by the embedded 45M-parameter Needle 2 engine. It runs as one Go process, invokes `libneedle.so` through CGO, and requires no model download at runtime.

> **Needle 2 is not a chat model.** It selects function tools and produces structured arguments. This server never executes tools and does not generate normal free-form answers.

## API

| Method | Path | Authentication | Purpose |
|---|---|---|---|
| GET | `/healthz` | public | Go process liveness |
| GET | `/readyz` | public | Embedded model loading/ready/failed state |
| GET | `/v1/models` | Bearer `NEEDLE_API_KEY` | OpenAI model discovery |
| POST | `/v1/chat/completions` | Bearer `NEEDLE_API_KEY` | Dynamic function-tool selection |

## Run

The production image supports `linux/amd64` only.

```bash
cp .env.example .env
# Replace NEEDLE_API_KEY, then:
docker compose up -d --build
```

The image runs as `nonroot`, supports a read-only root filesystem, and contains no Python, pip, Go toolchain, or Hugging Face client. The pinned model engine is embedded during the image build, so startup and inference work without network access.

Check startup:

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz
```

`/healthz` returns immediately. `/readyz` returns HTTP 503 with `loading` until the background native probe succeeds, then HTTP 200 with `ready`. A failed load remains `failed` until the container restarts.

## Function calling with curl

```bash
curl -s http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer ${NEEDLE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "needle-2",
    "messages": [{"role": "user", "content": "Start VM 3052"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "pve_vm_start",
        "description": "Start a Proxmox VE virtual machine",
        "parameters": {
          "type": "object",
          "properties": {"vmid": {"type": "integer"}},
          "required": ["vmid"],
          "additionalProperties": false
        }
      }
    }]
  }'
```

Example response:

```json
{
  "object": "chat.completion",
  "model": "needle-2",
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "type": "function",
        "function": {
          "name": "pve_vm_start",
          "arguments": "{\"vmid\":3052}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }],
  "x_needle": {
    "confidence": 0.97,
    "validation": {"ungrounded": [], "negation": false}
  }
}
```

The caller must validate arguments, apply authorization/risk policy, execute `pve_vm_start`, and send the result back in a subsequent request. `docs/api.md` describes one possible external Infrastructure Control tool catalogue; this server does not call those APIs.

## OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8080/v1",
    api_key="your-needle-api-key",
)

response = client.chat.completions.create(
    model="needle-2",
    messages=[{"role": "user", "content": "Weather in Lagos"}],
    tools=[{
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "Get weather for a city",
            "parameters": {
                "type": "object",
                "properties": {"city": {"type": "string"}},
                "required": ["city"],
                "additionalProperties": False,
            },
        },
    }],
)
print(response.choices[0].message.tool_calls)
```

## Standard tool loop

After executing a returned tool, resend the full history with its result:

```json
{
  "model": "needle-2",
  "messages": [
    {"role": "user", "content": "Weather in Lagos"},
    {"role": "assistant", "content": null},
    {"role": "tool", "tool_call_id": "call_...", "content": "{\"city\":\"Lagos\",\"temp_c\":27}"}
  ],
  "tools": ["the same function tool definitions"]
}
```

Each HTTP request is isolated: the server resets, initializes with that request's tools, and replays all user/tool turns. Assistant history is ignored because the native ABI cannot inject assistant turns; deterministic Needle regenerates its own intermediate decisions. The default maximum is 32 replay calls.

## Streaming

Set:

```json
{"stream": true, "stream_options": {"include_usage": true}}
```

The server emits OpenAI `chat.completion.chunk` SSE events and ends with:

```text
data: [DONE]
```

Streaming is synthesized after native inference finishes; it does not improve time to first byte.

## Safety signals

Every completion includes `x_needle` when available:

- `confidence`: calibrated score for the base model;
- `validation.ungrounded`: arguments not grounded in input;
- `validation.negation`: detected negation;
- `reasoning`: short model derivation;
- native throughput and memory metrics;
- warnings for accepted-but-ignored sampling options.

The server deliberately returns low-confidence and ungrounded calls unchanged. The external agent decides its own threshold and must validate arguments again at the tool execution boundary. High-impact tools require confirmation or another explicit safety policy.

Chinese and mixed-language input are accepted unchanged, but the base model is materially more reliable in English. Domain agents should normalize controlled Chinese business commands before calling this service and verify that returned arguments match the original request.

## Supported request subset

Supported:

- text-only `system`, `developer`, `user`, `assistant`, and `tool` messages;
- dynamic OpenAI function tools;
- `tool_choice`: `auto`, `required` (not enforceable), or a named function;
- `max_tokens` or `max_completion_tokens`;
- synthesized streaming and optional usage chunk.

Rejected:

- requests without tools;
- `tool_choice: "none"`;
- `response_format`;
- images, audio, files, and non-function tools;
- `n > 1`;
- unsupported or ambiguous JSON Schema features.

Sampling parameters such as `temperature`, `top_p`, `seed`, and `stop` are accepted but ignored because Needle decoding is deterministic; warnings report this explicitly.

### JSON Schema allowlist

Supported types: `object`, `array`, `string`, `integer`, `number`, `boolean`.

Supported keywords: `type`, `properties`, `required`, `description`, `enum`, `const`, `items`, numeric/string/array bounds, `pattern`, `format`, `uniqueItems`, and `additionalProperties: false`.

Rejected without lossy conversion: `$ref`, `$defs`, `definitions`, `oneOf`, `anyOf`, `allOf`, `not`, conditionals, recursion, unknown keywords, and free-form `additionalProperties`.

## Configuration

| Variable | Required | Default |
|---|---:|---|
| `NEEDLE_API_KEY` | yes | — |
| `NEEDLE_MODEL_ID` | no | `needle-2` |
| `NEEDLE_MAX_NEW_TOKENS` | no | `256` |
| `NEEDLE_MAX_QUEUE_DEPTH` | no | `32` |
| `NEEDLE_MAX_REPLAY_STEPS` | no | `32` |
| `NEEDLE_BUFFER_SIZE` | no | `1048576` |
| `NEEDLE_TOOL_INDEX_PATH` | no | empty |
| `NEEDLE_ENGINE_SLOW_CALL` | no | `30s` |

Native calls serialize on one locked OS thread. Queue waiting is cancelable; a native call already in progress cannot be interrupted. Size the go-zero HTTP timeout for queue wait plus inference.

## Embedded engine

- Cactus Compute Needle Engine: `2.0.3`
- Hugging Face revision: `32e9e3a93b205f786929697446ae669cf0a84579`
- Wheel SHA-256: `d23df1d0babeb7323dcaf860dfaf833bbd7d2229b205f691c05c9cbc6d3d3653`
- `libneedle.so` SHA-256: `0d2e125f36269067407ca4460f2d01b9371887366e5949243de9f03d0d93bc78`

See `licenses/THIRD_PARTY_NOTICES.md` and `licenses/Apache-2.0.txt`.

## Development

Project execution belongs in a Linux AMD64 Docker environment:

```bash
make test
make vet
make race        # requires CGO and /opt/needle/libneedle.so
make native-test # requires the pinned library
make image
```

## Migration from v0.1.1

v0.2.0 is intentionally breaking. See `CHANGELOG.md`. Keep using immutable `v0.1.1` if you need the former PVE-specific endpoint that automatically executed Infrastructure Control actions.
