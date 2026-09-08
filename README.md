# needle-controller

An OpenAI-compatible managed VM-start service powered by the embedded Needle 2 model. Send one Chinese user instruction; the server requires Needle 2 to select the built-in tool, validates the result, and submits one Infrastructure Control request.

> **v0.3.0 automatically executes one managed tool: `pve_vm_start`.** Use immutable v0.2.0 instead if you need a general model endpoint that accepts client-provided tools and returns `tool_calls` without execution.

## Request flow

```text
final Chinese user message
→ strict input gate and controlled English normalization
→ embedded Needle 2 semantic tool selection
→ model result/confidence/grounding/ID validation
→ one POST to Infrastructure Control
→ final Chinese OpenAI assistant response
```

Chinese rules never execute the API directly. A missing, failed, low-confidence, mismatched, ungrounded, or negated Needle decision stops before Infrastructure Control.

## API

| Method | Path | Authentication | Purpose |
|---|---|---|---|
| GET | `/healthz` | public | Go process liveness |
| GET | `/readyz` | public | Embedded Needle model state |
| GET | `/v1/models` | Bearer `NEEDLE_API_KEY` | Model discovery |
| POST | `/v1/chat/completions` | Bearer `NEEDLE_API_KEY` | Managed Chinese VM start |

## Run

Linux AMD64 only:

```bash
cp .env.example .env
# Set NEEDLE_API_KEY, INFRA_CONTROL_API_BASE_URL and INFRA_CONTROL_API_TOKEN.
docker compose up -d --build
```

The image runs as `nonroot`, supports a read-only root filesystem, embeds the hash-pinned Needle Engine 2.0.3, and contains no Python, pip, Go toolchain, or Hugging Face runtime client.

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz
```

`/healthz` responds immediately. `/readyz` returns 503 while the native model loads, then 200 when ready. Native load failure remains failed until restart.

## Minimal request

The client does not provide tools:

```bash
curl -s http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer ${NEEDLE_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "needle-2",
    "messages": [
      {"role": "user", "content": "开启 VM 3052"}
    ]
  }'
```

Successful Infrastructure Control HTTP 202 produces:

```json
{
  "object": "chat.completion",
  "model": "needle-2",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "VM 3052 的启动请求已提交。"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0,
    "estimated": true
  },
  "x_needle": {
    "type": "call",
    "tool": "pve_vm_start",
    "arguments": {"vmid": 3052},
    "confidence": 0.97,
    "validation": {"ungrounded": [], "negation": false},
    "executed": true,
    "upstream_status": 202,
    "warnings": []
  }
}
```

`启动请求已提交` does not mean the VM has completed startup. Query VM state before repeating a request after any ambiguous network/upstream error.

## Agent clients

Agents may send system prompts, history, tools, tool choice, response format, and sampling parameters. The server accepts but ignores them. It processes only `messages[len(messages)-1]`, which must be a non-empty plain-text `role: "user"` message. The final content may be a string or one or more OpenAI `{type: "text", text: "..."}` parts; text parts are joined in order. Images, audio, files, unknown parts, and empty parts are rejected.

Example:

```json
{
  "model": "needle-2",
  "messages": [
    {"role": "system", "content": "ignored"},
    {"role": "user", "content": "历史请求也会被忽略"},
    {"role": "assistant", "content": "ignored history"},
    {"role": "user", "content": "开启 VM 3052"}
  ],
  "tools": [{"malformed": "ignored"}],
  "tool_choice": "required"
}
```

The response includes category warnings such as:

```json
[
  "earlier messages were ignored; only the final user message is processed",
  "client-provided tools were ignored; the managed tool catalog is fixed",
  "tool_choice was ignored; the managed tool catalog is fixed"
]
```

Ignored content is never sent to Needle or copied into warnings/logs. If the final message is not a plain-text user message, the server does not search backward and returns `user_message_required`.

## Supported Chinese commands

Examples:

```text
开启 3052 这个 VM
启动虚拟机 3052
把 VM 3052 开起来
给 3052 号虚拟机开机
打开 VM 3052
```

These normalize internally to `Start VM 3052`, then Needle 2 must independently return exactly `pve_vm_start({"vmid":3052})`.

Rejected before inference include negative/conflicting operations, multiple/no IDs, missing VM semantics, English-only input, and ambiguous `VMware` matches:

```text
不要开启 VM 3052
关闭 VM 3052
重启 VM 3052
启动 VM 3052 和 3053
开启 VMware 3052
```

## Execution safety

All gates are mandatory:

- exactly one Needle function call;
- function name `pve_vm_start`;
- exactly one positive integer `vmid` argument;
- model VM ID equals the ID in the final Chinese input;
- `confidence >= NEEDLE_MIN_CONFIDENCE` (default `0.6`);
- complete validation metadata;
- `ungrounded` is empty;
- `negation` is false.

The server then sends at most one request:

```http
POST {INFRA_CONTROL_API_BASE_URL}/api/v1/pve/vms/{vmid}/start
Authorization: Bearer <INFRA_CONTROL_API_TOKEN>
```

It does not follow redirects or retry failures. Only HTTP 202 is success. The model cannot provide the URL, method, headers, or credentials.

## Streaming

Add:

```json
{"stream": true, "stream_options": {"include_usage": true}}
```

After model selection and upstream execution finish, the server emits synthesized SSE: assistant role, Chinese content, terminal `stop` metadata, optional usage, and `data: [DONE]`. It never emits a `tool_calls` delta because the tool has already executed. Streaming does not improve time to first byte.

## Configuration

| Variable | Required | Default |
|---|---:|---|
| `NEEDLE_API_KEY` | yes | — |
| `NEEDLE_MODEL_ID` | no | `needle-2` |
| `NEEDLE_MIN_CONFIDENCE` | no | `0.6` |
| `NEEDLE_MAX_MESSAGE_LENGTH` | no | `512` |
| `NEEDLE_MAX_NEW_TOKENS` | no | `256` |
| `NEEDLE_MAX_QUEUE_DEPTH` | no | `32` |
| `NEEDLE_BUFFER_SIZE` | no | `1048576` |
| `NEEDLE_TOOL_INDEX_PATH` | no | empty |
| `NEEDLE_ENGINE_SLOW_CALL` | no | `30s` |
| `INFRA_CONTROL_API_BASE_URL` | yes | — |
| `INFRA_CONTROL_API_TOKEN` | yes | — |
| `INFRA_CONTROL_TIMEOUT` | no | `30s` |

The Infrastructure base URL must be an HTTP(S) origin without credentials, path, query, or fragment. Tokens never enter Needle prompts or responses.

## Embedded engine

- Engine: 2.0.3
- Hugging Face revision: `32e9e3a93b205f786929697446ae669cf0a84579`
- Wheel SHA-256: `d23df1d0babeb7323dcaf860dfaf833bbd7d2229b205f691c05c9cbc6d3d3653`
- `libneedle.so` SHA-256: `0d2e125f36269067407ca4460f2d01b9371887366e5949243de9f03d0d93bc78`

See `licenses/THIRD_PARTY_NOTICES.md`.

## Development

Run project execution in Linux AMD64 Docker:

```bash
make test
make vet
make race
make native-test
make image
```
