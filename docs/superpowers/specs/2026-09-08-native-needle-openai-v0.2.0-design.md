# Native Needle OpenAI Service v0.2.0 — Design Specification

**Date:** 2026-09-08  
**Status:** Approved in discussion; awaiting written-spec review  
**Target release:** `v0.2.0`

## 1. Purpose

Transform `needle-controller` from a PVE-specific controller that calls a separate `needle-openai` service into a general OpenAI-compatible tool-calling model service. The new service embeds the fixed Needle Engine 2.0.3 `libneedle.so` in its container image and invokes it directly from one Go process through CGO.

The service selects tools and returns OpenAI `tool_calls`; it never executes tools. Infrastructure-specific language normalization, credentials, safety policy, and execution belong to external agents.

## 2. Goals

- One container image and one Go process.
- Direct CGO calls to the Needle native ABI; no Python or `needle-openai` runtime.
- Runtime operation without network access.
- Linux AMD64 support in the first release.
- OpenAI-compatible model discovery and Chat Completions tool calling.
- Standard multi-turn tool-result replay.
- Synthesized OpenAI SSE streaming.
- Strict pre-native validation of requests and JSON Schemas.
- One bounded queue and one locked OS thread for every native operation.
- Request-level isolation: reinitialize and replay each request completely.
- Separate liveness and asynchronous model readiness semantics.
- One required Bearer credential, `NEEDLE_API_KEY`, for all `/v1/*` endpoints.
- Preserve native confidence, reasoning, validation, and performance signals in `x_needle` without server-side confidence filtering.

## 3. Non-goals

- General text chat or free-form answer generation.
- Executing tools selected by the model.
- Calling Infrastructure Control or retaining any PVE-specific behavior.
- Translating or normalizing arbitrary Chinese input.
- Guaranteeing Chinese tool-selection quality.
- Supporting images, audio, arbitrary multimodal content, or structured `response_format` output.
- Supporting every OpenAI Chat Completions parameter or every JSON Schema keyword.
- True token-by-token streaming.
- Tokenizer-based usage accounting.
- Native session sharing across requests.
- Multi-process native crash isolation.
- Non-Linux, non-AMD64, or non-CGO production builds in v0.2.0.
- Tuned `.cact` weights in v0.2.0.

## 4. Breaking Changes from v0.1.1

Remove:

- `POST /api/v1/chat`.
- Automatic Infrastructure Control execution.
- Fixed `pve_vm_start` tool declarations.
- PVE-specific Chinese normalization and VM-ID cross-checking.
- `internal/infracontrol/` and PVE orchestration code.
- `CONTROLLER_API_TOKEN`.
- `NEEDLE_CONTROLLER_API_TOKEN` and `NEEDLE_OPENAI_API_KEY` proposed during design; neither will ship.
- `INFRA_CONTROL_BASE_URL`, `INFRA_CONTROL_API_TOKEN`, and `INFRA_CONTROL_TIMEOUT`.
- `NEEDLE_BASE_URL`, `NEEDLE_API_KEY`'s former downstream-client meaning, and `NEEDLE_TIMEOUT`.

Add or redefine:

- Required `NEEDLE_API_KEY` as the public OpenAI-compatible API credential.
- `GET /v1/models`.
- `POST /v1/chat/completions`.
- Embedded Needle Engine 2.0.3 for `linux/amd64`.
- CGO and dynamic linking to `/opt/needle/libneedle.so`.
- Native loading states and a bounded single-thread dispatcher.

The existing `v0.1.1` Git and container image tags remain immutable and available for users who need the old automatic PVE execution behavior.

## 5. Architecture

```text
                         needle-controller (one Go process)
┌─────────────────────────────────────────────────────────────────┐
│ go-zero REST                                                    │
│ ├── GET  /healthz                    public                     │
│ ├── GET  /readyz                     public                     │
│ ├── GET  /v1/models                  NEEDLE_API_KEY             │
│ └── POST /v1/chat/completions        NEEDLE_API_KEY             │
│                         │                                       │
│                         ▼                                       │
│ OpenAI protocol layer                                           │
│ ├── request/message/tool validation                             │
│ ├── safe tool_choice narrowing                                 │
│ ├── replay plan construction                                   │
│ ├── native response mapping                                    │
│ └── synthesized SSE rendering                                  │
│                         │                                       │
│                         ▼                                       │
│ Native dispatcher                                               │
│ ├── bounded queue                                               │
│ ├── one dedicated goroutine                                    │
│ ├── runtime.LockOSThread()                                     │
│ └── background load state                                      │
│                         │                                       │
│                         ▼                                       │
│ CGO ABI                                                         │
│ ├── needle_reset()                                              │
│ ├── needle_init()                                               │
│ └── needle_complete()                                           │
│                         │                                       │
│                         ▼                                       │
│ /opt/needle/libneedle.so                                        │
└─────────────────────────────────────────────────────────────────┘
```

### 5.1 Dependency boundaries

- `handler` owns HTTP decoding, headers, and status rendering.
- `openai` owns OpenAI request validation, replay planning, responses, errors, and SSE.
- `native` owns CGO ABI access, model state, request serialization, and the native output contract.
- `middleware` owns Bearer authentication.
- No handler or protocol code calls CGO directly.
- No native code knows about Infrastructure Control or arbitrary external tool execution.

## 6. Public HTTP API

### 6.1 `GET /healthz`

Public liveness endpoint. It returns HTTP 200 whenever the Go HTTP process can respond, regardless of model state.

```json
{"status":"ok"}
```

### 6.2 `GET /readyz`

Public readiness endpoint. HTTP status depends only on the embedded Needle model:

| Model state | HTTP | Response status |
|---|---:|---|
| `loading` | 503 | `loading` |
| `ready` | 200 | `ready` |
| `failed` | 503 | `failed` |

Example:

```json
{
  "status": "ready",
  "model": "ready"
}
```

Infrastructure Control status is removed because v0.2.0 has no Infrastructure Control dependency.

Responses must not expose native library paths, raw ABI errors, stack traces, or secrets.

### 6.3 `GET /v1/models`

Requires:

```http
Authorization: Bearer <NEEDLE_API_KEY>
```

Returns:

```json
{
  "object": "list",
  "data": [{
    "id": "needle-2",
    "object": "model",
    "created": 0,
    "owned_by": "cactus-compute"
  }]
}
```

The advertised identifier defaults to `needle-2` and may be changed with `NEEDLE_MODEL_ID`. This changes API naming only; it does not select another engine.

### 6.4 `POST /v1/chat/completions`

Requires the same Bearer credential. It accepts text messages and one or more OpenAI function tools, invokes Needle, and returns tool calls without executing them.

Minimal request:

```json
{
  "model": "needle-2",
  "messages": [{"role":"user","content":"Start VM 3052"}],
  "tools": [{
    "type": "function",
    "function": {
      "name": "pve_vm_start",
      "description": "Start a Proxmox VE virtual machine",
      "parameters": {
        "type": "object",
        "properties": {"vmid":{"type":"integer"}},
        "required": ["vmid"],
        "additionalProperties": false
      }
    }
  }]
}
```

The service does not know what `pve_vm_start` does and never calls it. The caller executes returned tool calls and may submit tool results in a subsequent Chat Completions request.

## 7. OpenAI Request Compatibility

### 7.1 Supported fields

| Field | Behavior |
|---|---|
| `model` | Required; exact match to configured `NEEDLE_MODEL_ID`. |
| `messages` | Required, non-empty, supported roles/content only. |
| `tools` | Required; one or more `type: function` tools. |
| `tool_choice` | Supports absent/`auto`, `required`, or named function. |
| `max_tokens` | Optional positive integer; defaults to configured maximum. |
| `max_completion_tokens` | Accepted alias; conflict with `max_tokens` is rejected. |
| `stream` | Supports `false` and synthesized SSE for `true`. |
| `stream_options.include_usage` | Supports an optional final usage chunk. |
| `user` | Accepted, ignored. |
| Unknown top-level fields | Accepted for client compatibility unless they conflict with explicitly unsupported behavior. |

### 7.2 Unsupported fields and behavior

- Any `response_format` returns HTTP 400 with `response_format_not_supported`.
- `tool_choice: "none"` returns HTTP 400 with `tools_required`.
- `n > 1` returns HTTP 400.
- Image or audio content returns HTTP 400.
- Non-function tools return HTTP 400.
- Unknown or unsupported JSON Schema constructs return HTTP 400 with parameter `tools`.
- Sampling fields such as `temperature`, `top_p`, `seed`, penalties, and stop settings are accepted but ignored and listed in `x_needle.warnings`.
- The service never claims these controls affect deterministic decoding.

### 7.3 `tool_choice`

- Absent or `auto`: pass all validated tools to Needle.
- Named function: validate that the name exists, then pass only that tool.
- `required`: pass all tools; add a warning that Needle cannot enforce a call.
- `none`: reject because this service requires at least one callable function and cannot provide free-form output.

## 8. Message Validation and Replay

### 8.1 Roles

Supported roles:

- `system`
- `developer`
- `user`
- `assistant`
- `tool`
- legacy `function` only if represented as text and mapped unambiguously

### 8.2 Content

- Text strings are accepted.
- OpenAI text content parts may be flattened in order.
- Image, audio, file, or unknown content parts are rejected.
- Tool-result content must be text.
- Empty conversations and conversations with no executable user/tool turn are rejected.

### 8.3 System facts

Text from `system` and `developer` messages is joined in request order and passed to `needle_init`. Documentation must state that Needle treats system text primarily as environment facts and does not reliably follow arbitrary large-model instruction prompts.

### 8.4 Turns

- Each `user` message becomes one native `needle_complete` call.
- Consecutive `tool` messages are combined into a JSON array and become one native `needle_complete` call.
- `assistant` messages are not injected because the native ABI exposes no assistant-history operation.
- During full replay, deterministic Needle regenerates its own intermediate decisions.
- The final native response produced by the last replayed user/tool turn becomes the HTTP response.

### 8.5 Isolation

Every request is independent:

```text
needle_reset()
needle_init(system, toolsJSON, toolIndexPath)
complete(turn 1)
complete(turn 2)
...
return final result
```

No request-prefix or toolset cache ships in v0.2.0. Requests from separate agents cannot share logical history.

### 8.6 Limits

- `NEEDLE_MAX_REPLAY_STEPS` defaults to 32.
- Replay steps count native `complete` calls, including grouped tool-result turns.
- Requests exceeding the limit fail before entering the native queue.
- Message, body, and tool-catalog byte limits are enforced before native calls.

## 9. Tool and JSON Schema Validation

Validation occurs in pure Go before a request enters the native queue.

### 9.1 Tool limits

- At least 1 tool.
- At most 64 tools by default.
- Tool catalog serialized size at most 256 KiB by default.
- Tool type must be exactly `function`.
- Tool names must match `^[A-Za-z_][A-Za-z0-9_-]{0,63}$`.
- Tool names must be unique.
- Description length is at most 1024 Unicode characters.
- Schema nesting depth is at most 8.
- Each object has at most 64 properties.

### 9.2 Supported JSON types

- `object`
- `array`
- `string`
- `integer`
- `number`
- `boolean`

### 9.3 Allowed keywords

- `type`
- `properties`
- `required`
- `description`
- `enum`
- `const`
- `items`
- `minimum`
- `maximum`
- `exclusiveMinimum`
- `exclusiveMaximum`
- `multipleOf`
- `minLength`
- `maxLength`
- `pattern`
- `format`
- `minItems`
- `maxItems`
- `uniqueItems`
- `additionalProperties`

`additionalProperties` is accepted only when its value is `false`. A value of `true` or a schema object is rejected.

### 9.4 Rejected schema features

- `$ref`
- `$defs`
- `definitions`
- `oneOf`
- `anyOf`
- `allOf`
- `not`
- `if` / `then` / `else`
- recursive schemas
- unknown keywords

No unsupported construct is stripped or simplified. Validation fails rather than weakening the caller's contract.

## 10. Native ABI

The ABI inferred from the pinned upstream Python binding is declared in `native/include/needle.h`:

```c
#include <stdint.h>

int needle_init(
    const char *system,
    const char *tools_json,
    const char *tool_index_path
);

int needle_complete(
    const char *text,
    int max_new_tokens,
    char *output_buffer,
    int output_buffer_size
);

void needle_reset(void);

int needle_load(
    const char *weights_blob,
    uint64_t weights_size
);
```

v0.2.0 uses `needle_init`, `needle_complete`, and `needle_reset`. `needle_load` may be declared but is not called; custom `.cact` support remains out of scope.

### 10.1 Build constraints

Production implementation:

```go
//go:build linux && amd64 && cgo
```

Unsupported-build implementation:

```go
//go:build !linux || !amd64 || !cgo
```

The unsupported implementation reports that native Needle requires Linux AMD64 with CGO and never claims readiness.

### 10.2 Dynamic linking

The production image contains:

```text
/needle-controller
/opt/needle/libneedle.so
```

The Go build links with `-lneedle` and an RPATH of `/opt/needle`. `LD_LIBRARY_PATH` is not required for normal operation. The runtime base is `gcr.io/distroless/cc-debian12:nonroot`, not the current static image, because the engine uses a dynamic glibc-compatible library.

## 11. Native Dispatcher

### 11.1 Thread confinement

A dedicated goroutine starts during service construction:

```go
func (d *Dispatcher) loop() {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    d.initialize()
    for job := range d.queue {
        d.execute(job)
    }
}
```

Every `needle_init`, `needle_complete`, and `needle_reset` call occurs only in this loop. HTTP handlers cannot access the ABI.

### 11.2 Queue

- Capacity defaults to 32 through `NEEDLE_MAX_QUEUE_DEPTH`.
- Requests submitted while the model is not ready return 503 without queueing.
- A full queue returns OpenAI HTTP 429 `engine_busy`.
- Waiting jobs observe caller context cancellation and are skipped before native execution.
- No unbounded internal queue exists.

### 11.3 Cancellation

- Cancellation while waiting prevents execution.
- Once native execution begins, `needle_complete` runs until it returns.
- If the caller disconnects during native execution, the dispatcher completes the native operation, restores a safe state, and discards delivery when appropriate.
- A second native request never starts before the first C call returns.
- Threads are never killed.
- Ordinary request timeouts do not terminate the process.

### 11.4 Error handling

- `needle_init < 0`: mark the engine `failed`; current request returns 502 and future requests return 503.
- `needle_complete < 0`: fail the current request, reset, and retain `ready` unless reset/next initialization proves otherwise.
- Unparseable output: return 502, reset, do not log the full output buffer.
- Native SIGSEGV: process exits; Docker/Kubernetes restarts it.
- No native operation is automatically retried.

## 12. Model Lifecycle

### 12.1 States

```go
type EngineState string

const (
    EngineLoading EngineState = "loading"
    EngineReady   EngineState = "ready"
    EngineFailed  EngineState = "failed"
)
```

Allowed transitions:

```text
loading -> ready
loading -> failed
ready   -> failed
```

There is no transition out of `failed` in the same process.

### 12.2 Background initialization

- HTTP starts first, making `/healthz` available.
- A background task submits initialization to the locked native thread.
- `/readyz` reports 503 while loading.
- Inference requests return OpenAI HTTP 503 `model_not_ready` while loading or failed.
- Initialization success sets `ready`.
- Initialization failure sets `failed`, records a sanitized diagnostic, and does not retry.
- Recovery is by container restart.

The initialization probe must exercise `needle_init` and one minimal, side-effect-free `needle_complete` request against a fixed probe schema. Probe output is validated but never exposed as a user completion.

## 13. Output Buffer

- `NEEDLE_BUFFER_SIZE` defaults to 1 MiB.
- Minimum: 64 KiB.
- Maximum: 8 MiB.
- Allocation occurs once in the native worker.
- The first byte is cleared before every `needle_complete` call.
- Output length is determined by the first NUL byte.
- Absence of NUL termination is treated as truncation and a 502 error.
- Invalid JSON never causes the full raw buffer to enter logs or API responses.

## 14. Native Response Contract

The service decodes the native envelope fields needed by OpenAI mapping:

- `type`
- `success`
- `error`
- `error_code`
- `function_calls`
- `reasoning`
- `confidence`
- `validation`
- `prefill_tps`
- `decode_tps`
- `peak_ram_mb`

Function-call arguments are native JSON objects. OpenAI responses encode each one as a JSON string in `message.tool_calls[].function.arguments`.

The service validates that returned tool names belong to the effective request toolset. It does not validate model arguments against the schema a second time beyond structural JSON correctness because Needle's decoding grammar is the primary schema enforcer; external agents remain responsible for validating arguments at their execution boundary.

## 15. OpenAI Response Mapping

### 15.1 Tool calls

A native call maps to:

- `object: "chat.completion"`
- one choice
- `message.role: "assistant"`
- `message.content: null`
- generated stable-per-response call IDs
- `finish_reason: "tool_calls"`

### 15.2 No selected tool

If tools were declared but Needle returns no function calls:

- HTTP 200
- `message.role: "assistant"`
- `message.content: null`
- no `tool_calls`
- `finish_reason: "stop"`
- reasoning remains only in `x_needle`

The service never promotes internal reasoning into a free-form assistant answer.

### 15.3 Usage

v0.2.0 does not include a tokenizer. Usage is explicit and honest:

```json
{
  "prompt_tokens": 0,
  "completion_tokens": 0,
  "total_tokens": 0,
  "estimated": true
}
```

### 15.4 `x_needle`

Always include available native data:

```json
{
  "type": "call",
  "confidence": 0.9044,
  "reasoning": "...",
  "validation": {
    "ungrounded": [],
    "negation": false
  },
  "prefill_tps": 57.8,
  "decode_tps": 16.4,
  "peak_ram_mb": 147.6,
  "warnings": []
}
```

The general endpoint does not suppress tool calls based on confidence, grounding, or negation. Callers must apply policy appropriate to the tool's risk.

## 16. Synthesized Streaming

For `stream: true`, wait for complete native inference, then emit well-formed SSE:

1. role delta;
2. tool-call name and ID delta for each call;
3. argument string fragments;
4. terminal finish-reason chunk carrying `x_needle`;
5. optional usage chunk when `stream_options.include_usage` is true;
6. `data: [DONE]`.

All chunks share one completion ID, creation time, and model ID. Headers include:

```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

Documentation must state that streaming is synthesized and time-to-first-chunk equals full inference time.

## 17. Authentication

- `GET /healthz` and `GET /readyz` are public.
- Every `/v1/*` endpoint requires exactly one `Authorization: Bearer <NEEDLE_API_KEY>` header.
- Missing and wrong credentials return the same OpenAI-format HTTP 401 response.
- Token comparison uses constant-time digest comparison.
- `NEEDLE_API_KEY` is required at startup.
- There is no compatibility fallback to `CONTROLLER_API_TOKEN`, `NEEDLE_OPENAI_API_KEY`, or `NEEDLE_CONTROLLER_API_TOKEN`.

## 18. Error Format

All `/v1/*` errors use:

```json
{
  "error": {
    "message": "...",
    "type": "invalid_request_error",
    "param": "tools",
    "code": "tools_required"
  }
}
```

Core mappings:

| Condition | HTTP | Code |
|---|---:|---|
| Invalid JSON/messages/tools/schema | 400 | `invalid_request_error` or specific validation code |
| No tools / `tool_choice: none` | 400 | `tools_required` |
| Unsupported `response_format` | 400 | `response_format_not_supported` |
| Unsupported model | 404 | `model_not_found` |
| Missing/wrong API key | 401 | `invalid_api_key` |
| Model loading or failed | 503 | `model_not_ready` |
| Native queue full | 429 | `engine_busy` |
| Native engine operation error | 502 | `engine_error` |
| Replay limit exceeded | 400 | `replay_limit_exceeded` |

The public API never returns raw C buffers, internal stack traces, dynamic-library paths, or credentials.

## 19. Configuration

Remove v0.1.x Controller/Infrastructure/remote-Needle variables. v0.2.0 uses:

| Variable | Required | Default | Purpose |
|---|---:|---|---|
| `NEEDLE_API_KEY` | yes | none | Bearer auth for `/v1/*`. |
| `NEEDLE_MODEL_ID` | no | `needle-2` | Advertised and accepted model ID. |
| `NEEDLE_MAX_NEW_TOKENS` | no | `256` | Default native generation cap. |
| `NEEDLE_MAX_QUEUE_DEPTH` | no | `32` | Native job queue capacity. |
| `NEEDLE_MAX_REPLAY_STEPS` | no | `32` | Maximum native replay calls per request. |
| `NEEDLE_BUFFER_SIZE` | no | `1048576` | Native output buffer, 64 KiB–8 MiB. |
| `NEEDLE_TOOL_INDEX_PATH` | no | empty | Optional persistent tool-index path. |
| `NEEDLE_ENGINE_SLOW_CALL` | no | `30s` | Slow native-call logging threshold. |

Existing go-zero host, port, and top-level HTTP timeout remain configurable. HTTP timeout must exceed expected queue plus native execution duration; documentation must explain this relationship.

## 20. Build and Image

### 20.1 Pinned artifact

Lock:

- Engine version: `2.0.3`.
- Hugging Face repository: `Cactus-Compute/needle2`.
- Wheel path: `python/cactus_needle-2.0.3-py3-none-manylinux2014_x86_64.whl`.
- Extracted path: `needle/libneedle.so`.

`build/needle-engine.env` records exact wheel and library SHA-256 values obtained during an approved artifact probe. Formal builds do not accept build arguments that replace these values.

### 20.2 Docker stages

```text
needle-fetcher: fixed artifact download, wheel hash check, extraction, .so hash check
builder:        CGO Linux AMD64 build and dependency inspection
runtime:        distroless cc Debian 12 nonroot
```

Final image contains only:

```text
/needle-controller
/opt/needle/libneedle.so
/etc/needle-controller/config.yaml
/licenses/needle/LICENSE
/licenses/needle/THIRD_PARTY_NOTICES.md
```

It excludes Python, pip, Go, compilers, Hugging Face libraries, source trees, caches, and download credentials.

### 20.3 Reproducibility and offline runtime

- Both wheel and `.so` hashes are mandatory.
- Final image startup performs no download.
- A separate final-stage build or runtime smoke test with network disabled proves no runtime network dependency.
- `ldd` is run in a compatible inspection stage and every runtime dependency is accounted for in the final base.
- Image runs as `nonroot:nonroot` with a read-only root filesystem.

## 21. Licensing Gate

Source is Apache-2.0, but embedding and publishing the Hugging Face wheel's native binary requires artifact-level verification.

Before any image containing `libneedle.so` is pushed:

1. Fetch Hugging Face repository metadata and record its declared license.
2. Inspect the wheel for `LICENSE`, `NOTICE`, metadata license fields, and extra redistribution terms.
3. Confirm the wheel and source attribution are consistent.
4. Record wheel and `.so` SHA-256 values.
5. Add Apache-2.0 and required notices to the final image.
6. Confirm no term prohibits redistribution to the intended private or public registry.

If licensing remains ambiguous, development may continue through local technical validation, but publishing an image containing the binary is blocked. Git source that does not embed the binary may still be pushed.

## 22. Repository Layout

```text
cmd/needle-controller/
internal/
├── apierror/
├── config/
├── handler/
├── middleware/
├── native/
├── openai/
└── svc/
native/include/needle.h
build/needle-engine.env
licenses/
├── Apache-2.0.txt
└── THIRD_PARTY_NOTICES.md
etc/needle-controller.yaml
Dockerfile
docker-compose.yml
README.md
needle-controller.api
```

Remove `internal/infracontrol/` and PVE-specific `internal/logic/` code once replacement tests are in place.

## 23. Testing Strategy

Implementation follows TDD and all dependency resolution, builds, tests, native execution, and image inspection run through Docker API tooling.

### 23.1 Pure Go protocol tests

- OpenAI request decoding and permissive unknown top-level fields.
- Required model/messages/tools.
- Text-only content flattening.
- Rejected multimodal content.
- `tool_choice` safe subset.
- Unsupported response format and `n > 1`.
- OpenAI error envelopes and statuses.
- Normal and no-call completion mapping.
- Stable response IDs and encoded argument strings.
- Always-present `x_needle` safety data.

### 23.2 Schema tests

- Every allowed keyword and type.
- Unknown keyword rejection.
- `$ref`, combinator, conditional, recursive, and free additional-properties rejection.
- Duplicate names, malformed names, size limits, depth limits, property limits, and description limits.
- Named `tool_choice` narrowing.

### 23.3 Replay tests

- System/developer joining.
- User turns.
- Assistant omission.
- Consecutive tool-result grouping.
- Tool-result text restrictions.
- Full reset/init/replay for every request.
- No cross-request state reuse.
- Replay-limit rejection before queueing.
- Reset after native errors.

### 23.4 Dispatcher tests

Using a fake ABI:

- Background `loading -> ready` and `loading -> failed`.
- No retry after failed initialization.
- All native calls are serialized.
- Queue-full response.
- Waiting cancellation prevents execution.
- Cancellation after entry does not overlap the next call.
- Init failure marks failed.
- Complete failure resets and returns an error.
- Buffer NUL/truncation and malformed JSON handling.

OS-thread confinement is implemented in production and verified through code structure plus a test ABI that records thread identity where supported.

### 23.5 Authentication and endpoint tests

- Public liveness/readiness.
- Protected `/v1/models` and `/v1/chat/completions`.
- Missing/wrong/exact key behavior.
- Model readiness errors.
- OpenAI JSON content type and SSE headers.
- Request IDs and secret-free responses/logs.

### 23.6 SSE tests

- Role chunk.
- Tool-call start/name/ID.
- Argument fragments reassemble to exact JSON.
- Finish reason and `x_needle`.
- Optional usage chunk.
- `[DONE]`.
- One shared completion ID.

### 23.7 Real native integration tests

Inside a Linux AMD64 CGO container with the pinned library:

```text
Start VM 3052
+ pve_vm_start schema
-> exactly one pve_vm_start call with vmid 3052
```

Also verify:

- confidence exists;
- valid JSON output;
- repeated independent requests are isolated;
- concurrent HTTP requests never overlap native calls;
- loading/readiness transition;
- final runtime starts without network;
- no Python, pip, Go toolchain, or Hugging Face client exists in final image.

### 23.8 Release verification

Before v0.2.0:

```text
go test ./...
go test -race ./...
go vet ./...
```

Additionally:

- verify wheel and `.so` hashes;
- inspect dynamic dependencies;
- build final image;
- run non-root/read-only/offline smoke tests;
- exercise `/v1/models` and real `/v1/chat/completions`;
- verify Git diff contains no credentials or binary artifact accidentally committed;
- satisfy the licensing gate before Registry push.

## 24. Documentation and Release Notes

README must clearly state:

- Needle is a tool-calling model, not a chat model.
- The server never executes tools.
- External agents own tool implementation, argument validation, authorization, risk controls, and confidence thresholds.
- Chinese input is accepted but quality is not guaranteed; domain agents should normalize business language when needed.
- Streaming is synthesized.
- Supported OpenAI fields and JSON Schema subset.
- Linux AMD64-only support.
- Model load/readiness behavior.
- Offline runtime and embedded engine version/hash.

Release notes must include:

```text
BREAKING:
- Removed POST /api/v1/chat
- Removed Infrastructure Control execution
- Removed CONTROLLER_API_TOKEN and INFRA_CONTROL_* configuration
- Removed remote NEEDLE_BASE_URL dependency
- Added required NEEDLE_API_KEY
- Added OpenAI-compatible /v1/models and /v1/chat/completions
- Embedded Needle Engine 2.0.3 for linux/amd64
- Tools are selected but never executed by the server
```

## 25. Acceptance Criteria

v0.2.0 is ready when:

1. One non-root Go process directly invokes the pinned `libneedle.so` through CGO.
2. The final image contains no Python, `needle-openai`, pip, Go toolchain, or Hugging Face client.
3. Runtime model initialization requires no network and readiness accurately transitions from loading to ready/failed.
4. Every native operation is serialized on one locked OS thread through a bounded queue.
5. Every request resets, initializes, and fully replays without cross-agent state sharing.
6. `/v1/models` and `/v1/chat/completions` use OpenAI-compatible auth, envelopes, errors, and tool calls.
7. Multi-turn user/tool replay works within the configured limit.
8. Dynamic function tools and the strict schema subset work; unsupported schemas fail before CGO.
9. `stream: true` produces valid synthesized SSE ending in `[DONE]`.
10. No-tools, unsupported model, unsupported response format, invalid key, not-ready, queue-full, and engine-error cases map as specified.
11. The general API returns low-confidence calls unchanged with complete `x_needle` signals and never executes them.
12. Real native inference produces `pve_vm_start({"vmid":3052})` for the controlled English probe.
13. Unit, race, vet, real-native, image, non-root, read-only, and offline runtime checks pass.
14. The artifact licensing gate is satisfied before any image containing `libneedle.so` is pushed.
15. `v0.1.1` remains unchanged and available as the last PVE-executing release.
