# Granite Controller v1.0.0 — Design Specification

**Date:** 2026-09-09  
**Status:** Approved in discussion; awaiting written-spec review  
**Target:** New `granite-controller` repository, first release `v1.0.0`

## 1. Purpose

Create a new `granite-controller` project from the complete `needle-controller` Git history, then replace the embedded Needle 2 engine and phrase-to-English normalization with IBM Granite 4.0 350M Q4_K_M running through a managed `llama-server` child process.

Granite receives the original final Chinese user message and five fixed bilingual PVE VM tools. It must return exactly one standard tool call. The Go controller then applies deterministic safety rules, checks current VM state, deduplicates mutation requests, calls one fixed Infrastructure Control endpoint, and returns an OpenAI-compatible Chinese response.

The project is a managed infrastructure controller. It is not a general chat model and does not execute client-provided tools.

## 2. Repository and Product Identity

Create new repositories:

```text
GitHub: aoliyougei/granite-controller
GitLab: aoliyougei/granite-controller
```

Preserve the complete Git history from `needle-controller`, but do not rename, delete, overwrite, or continue Granite development in the original repositories.

New identities:

```text
Project: granite-controller
Go module: github.com/aoliyougei/granite-controller
Image: registry.aoliyougei.com/aoliyougei/granite-controller
Compose service: granite-controller
Kubernetes service: granite-controller
Pi provider: Granite-Controller
OpenAI model ID: granite-4.0-350m
API credential: GRANITE_API_KEY
First formal release: v1.0.0
```

The existing `needle-controller` Git tags and images through `v0.3.2` remain immutable and available.

The current pi-web session workspace is not renamed during implementation. Work occurs in an isolated directory/worktree so the active `needle-controller` session path remains valid. The user can open the new project in pi-web after validation.

## 3. Model and Runtime Selection

Use the official dense instruct model:

```text
Repository: ibm-granite/granite-4.0-350m-GGUF
Artifact: granite-4.0-350m-Q4_K_M.gguf
Architecture: Granite dense 350M
Model context capability: 32768 tokens
Configured context: 4096 tokens
License: Apache-2.0
```

Do not use `granite-4.0-h-350m`; the hybrid Mamba2 architecture adds unnecessary llama.cpp compatibility constraints.

Use a pinned llama.cpp commit that has verified Granite 4 tool-calling support. Build and bundle `llama-server` from that exact commit.

Before image publication, record and verify:

- Hugging Face repository revision;
- GGUF SHA-256;
- llama.cpp Git commit;
- built `llama-server` SHA-256;
- Granite Apache-2.0 license and artifact metadata;
- llama.cpp MIT license and notices.

Runtime performs no model or binary downloads.

## 4. Architecture

```text
Pi / OpenAI client
        |
        | POST /v1/chat/completions
        v
granite-controller (Go PID 1, :8080)
├── OpenAI envelope parser
│   └── only final user text is retained
├── pre-model generic safety checks
├── Granite client
│   └── HTTP 127.0.0.1:18080
├── tool-call parser and safety checks
├── action deduplicator
├── Infrastructure Control client
│   ├── GET VM state
│   └── one optional mutation POST
├── fixed Chinese response/SSE renderer
└── llama-server child process
    └── Granite 4.0 350M Q4_K_M
```

One image and one Pod contain two processes. The Go controller is PID 1 and owns the child lifecycle.

Remove all Needle-specific components:

- `libneedle.so`;
- Needle artifact fetcher and hashes/HF coordinates;
- Needle licenses/notices;
- CGO ABI;
- native locked-thread Dispatcher;
- Needle confidence, `ungrounded`, and `negation` semantics;
- `NEEDLE_*` configuration;
- Chinese-to-English model input normalization.

## 5. Process Lifecycle

### 5.1 Child startup

The Go controller starts:

```text
/opt/granite/llama-server
  --model /opt/granite/granite-4.0-350m-Q4_K_M.gguf
  --host 127.0.0.1
  --port 18080
  --ctx-size 4096
  --parallel 1
  --threads 4
  --jinja
```

Thread count, context size, and startup/request timeouts are configurable within bounded ranges. Host, port, model path, Jinja mode, and parallelism remain fixed.

### 5.2 States

```text
starting -> granite_loading -> ready -> failed
```

- `/healthz` returns HTTP 200 while the Go process serves.
- `/readyz` returns HTTP 503 while Granite loads.
- `/readyz` returns HTTP 200 only after llama-server reports ready.
- Startup timeout defaults to 120 seconds.
- If startup times out, the Go process exits nonzero.
- If llama-server exits unexpectedly, the Go process exits nonzero.
- The controller does not restart llama-server internally; Kubernetes restarts the Pod.

### 5.3 Shutdown

On SIGTERM/SIGINT:

1. stop accepting new controller requests;
2. cancel/wait for active controller work within graceful timeout;
3. terminate llama-server;
4. wait for child exit;
5. exit the Go process.

An expected child exit during controller shutdown must not trigger a second failure path.

## 6. Granite Inference Contract

Controller calls only:

```http
POST http://127.0.0.1:18080/v1/chat/completions
```

with deterministic generation:

```json
{
  "model": "granite-4.0-350m",
  "messages": [{"role":"user","content":"<original Chinese>"}],
  "tools": ["five fixed bilingual functions"],
  "temperature": 0,
  "top_p": 1,
  "seed": 42,
  "max_tokens": 256,
  "stream": false
}
```

The original final user text is sent after trimming and length validation. It is not translated, keyword-replaced, or prefixed with a custom business instruction. Granite's official chat template and fixed tool definitions describe the task.

Granite must return exactly one standard OpenAI function `tool_call`. Free text, no call, malformed call, or multiple calls do not execute anything.

## 7. OpenAI Public API

Expose:

```http
GET  /healthz
GET  /readyz
GET  /v1/models
POST /v1/chat/completions
```

`/healthz` and `/readyz` are public. `/v1/*` requires:

```http
Authorization: Bearer <GRANITE_API_KEY>
```

`GET /v1/models` returns one model:

```json
{
  "object": "list",
  "data": [{
    "id": "granite-4.0-350m",
    "object": "model",
    "created": 0,
    "owned_by": "ibm-granite"
  }]
}
```

Requests naming another model return OpenAI HTTP 404 `model_not_found`.

## 8. Pi and Agent Input Handling

Continue the final-message rule from `needle-controller v0.3.2`:

- inspect only `messages[len(messages)-1]`;
- require role `user`;
- accept content as a string or one or more `{type:"text", text:"..."}` parts;
- join text parts in order;
- reject empty, image, audio, file, unknown, or non-text final content;
- never search backward for an earlier user command.

Accept but ignore:

- system/developer messages;
- all prior user/assistant/tool/function history;
- client-provided tools and tool choice;
- response format and sampling settings;
- unknown top-level fields.

Only the final user text reaches Granite. Pi's system prompt, history, and tools must not enter model context or logs.

The HTTP OpenAI envelope limit remains 8 MiB. Granite user text defaults to at most 2048 Unicode characters.

## 9. Five Fixed Bilingual Tools

Each tool accepts exactly one required positive integer `vmid` and `additionalProperties:false`.

### `pve_vm_get`

```text
Get the current status of a Proxmox VE virtual machine. 查询虚拟机当前状态，不改变虚拟机。
```

### `pve_vm_start`

```text
Start a stopped Proxmox VE virtual machine. 开启或启动已停止的虚拟机。
```

### `pve_vm_shutdown`

```text
Gracefully shut down a running Proxmox VE virtual machine. 正常关闭或优雅关机，不是强制断电。
```

### `pve_vm_stop`

```text
Force stop a running Proxmox VE virtual machine. 强制停止或强制断电，可能导致数据损坏。
```

### `pve_vm_reboot`

```text
Reboot a running Proxmox VE virtual machine. 重启或重新启动正在运行的虚拟机。
```

Client tools never alter this catalog.

## 10. Pre-Model Safety Checks

Granite performs intent selection. The controller does not maintain complete phrase templates, but it rejects generic dangerous structures before model inference.

### 10.1 VM identifier

- Require exactly one distinct ASCII decimal VM ID.
- Require positive signed 64-bit integer.
- Reject no ID, multiple IDs, zero, negative/ambiguous number syntax, and overflow.

### 10.2 Conditional and multi-action structures

Reject condition indicators such as:

```text
如果, 假如, 要是, 确认后, 完成后, 等…再
```

Reject batch/sequence indicators such as:

```text
全部, 所有, 分别, 依次, 然后, 接着
```

Reject multiple explicit VM actions in one message.

These are minimal safety structure words, not a full intent classifier.

### 10.3 Negation

Generic negation markers include:

```text
不要, 别, 禁止, 取消, 无需, 不允许
```

Negated status questions may proceed to Granite. If Granite selects `pve_vm_get`, read-only execution is allowed. If Granite selects a mutation, reject it.

### 10.4 Ambiguity

Granite selects the tool, but a mutation needs minimal original-text evidence:

| Tool selected | Required semantic evidence examples |
|---|---|
| `pve_vm_start` | 开启, 启动, 开机, 打开 |
| `pve_vm_shutdown` | 正常关闭, 正常关机, 优雅关机 |
| `pve_vm_stop` | 强制停止, 强制关机, 强制断电 |
| `pve_vm_reboot` | 重启, 重新启动 |
| `pve_vm_get` | 查询, 查看, 状态, 是否运行, 开着吗, equivalent read-only question |

This evidence only confirms that a selected mutation is grounded in the original text; it does not choose the tool. Ambiguous phrases such as `处理一下`, `弄一下`, `重置一下`, and standalone `停止` are rejected for mutation.

## 11. Tool-Call Validation

After Granite returns:

1. require one choice and exactly one tool call;
2. require function type;
3. require name in the five-tool allowlist;
4. require arguments to be one JSON object;
5. require exactly one key `vmid`;
6. require positive JSON integer without coercion;
7. require model VM ID equals the pre-model ID;
8. apply negation and semantic-evidence checks;
9. require `GRANITE_ALLOW_FORCE_STOP=true` for `pve_vm_stop`.

Free text/no call returns OpenAI HTTP 422 `tool_call_required` with Chinese message:

```text
模型未能确定要执行的虚拟机操作，请明确说明查询、开启、正常关机、强制停止或重启。
```

Multiple calls return HTTP 422 `multiple_tool_calls_not_supported`; none are executed.

The controller never parses free text to infer a missing tool call.

Granite does not expose a Needle-style calibrated confidence. Do not accept model-generated confidence or uncalibrated logprobs as an execution gate.

## 12. Infrastructure Control API

Use:

```text
INFRA_CONTROL_API_BASE_URL
INFRA_CONTROL_API_TOKEN
INFRA_CONTROL_TIMEOUT=30s
```

The base URL must be an HTTP(S) origin without userinfo, non-root path, query, or fragment. Token never enters Granite input.

Fixed mappings:

| Tool | Method | Path |
|---|---|---|
| `pve_vm_get` | GET | `/api/v1/pve/vms/{vmid}` |
| `pve_vm_start` | POST | `/api/v1/pve/vms/{vmid}/start` |
| `pve_vm_shutdown` | POST | `/api/v1/pve/vms/{vmid}/shutdown` |
| `pve_vm_stop` | POST | `/api/v1/pve/vms/{vmid}/stop` |
| `pve_vm_reboot` | POST | `/api/v1/pve/vms/{vmid}/reboot` |

No generic arbitrary request method exists. Redirects are rejected. Bodies are bounded. Mutation requests are never automatically retried.

## 13. Pre-Mutation State Check

Every mutation not already deduplicated first performs:

```http
GET /api/v1/pve/vms/{vmid}
```

Allowed states:

| Mutation | Required current state |
|---|---|
| start | `stopped` |
| shutdown | `running` |
| stop | `running` plus force-stop enabled |
| reboot | `running` |

A state mismatch returns OpenAI HTTP 200 with a fixed current-state/no-op message. It sends no mutation POST.

Examples:

```text
🟢 VM 3052 已处于 running 状态，无需重复启动。
🔴 VM 3052 已处于 stopped 状态，无需重复关闭。
```

State no-op results participate in short deduplication.

## 14. Action Deduplication

Configuration:

```text
GRANITE_ACTION_DEDUP_WINDOW=30s
```

Keys:

```text
<tool name>:<vmid>
```

Only mutation tools are deduplicated; `pve_vm_get` always queries current state.

Order:

```text
dedup lookup
-> if miss, state GET
-> optional mutation POST
```

Within the window:

- in-flight duplicate waits for and reuses the same result;
- successful 202 result is reused;
- state-mismatch/no-op result is reused;
- explicit HTTP 4xx/5xx failure is not cached and may be retried by a new request;
- network/response-loss unknown status is cached to prevent potentially duplicate mutation;
- different actions on the same VM use different keys;
- different VMs use different keys.

v1.0.0 supports one controller replica. Process restart clears dedup state. Multi-replica deployment requires a future distributed idempotency store or upstream idempotency key support.

Responses expose:

```json
{
  "executed": true,
  "deduplicated": false
}
```

A reused result reports:

```json
{
  "executed": false,
  "deduplicated": true
}
```

## 15. Upstream Errors and Diagnostics

Parse only bounded JSON fields:

```text
code
message
requestId
```

Never return arbitrary `details` or raw response bodies.

Map known stable upstream codes to safe Chinese messages. Preserve the stable code and optionally expose sanitized upstream request ID in an extension field.

Classify and log safely:

- HTTP status;
- stable upstream code;
- network error class (`dns`, `connect`, `tls`, `timeout`, `reset`, `unknown`);
- whether mutation acceptance is unknown.

Explicit HTTP failures are known failures. Network errors after request send are treated as unknown acceptance status for deduplication.

## 16. VM Query Result

Whitelist only:

```json
{
  "vmid": 3052,
  "name": "MS-SQL-1",
  "status": "running",
  "uptime": 12588
}
```

Do not forward unknown upstream fields.

Human-readable uptime:

- zero: `0 秒`;
- under minute: seconds;
- under hour: minutes and optional seconds;
- under day: hours and optional minutes;
- one day or more: days, optional hours, optional minutes;
- omit zero intermediate units unless total is zero.

Response text:

```text
🟢 VM 3052（MS-SQL-1）当前状态：running，运行时长 3 小时 29 分钟。
🔴 VM 3052（MS-SQL-1）当前状态：stopped。
🟡 VM 3052（MS-SQL-1）当前状态：paused。
```

If name is empty, omit the parenthesized name.

## 17. Mutation Responses

Fixed Chinese text:

```text
▶️ VM 3052 的启动请求已提交。
⏻ VM 3052 的正常关机请求已提交。
⚠️ VM 3052 的强制停止请求已提交。
🔄 VM 3052 的重启请求已提交。
```

Only HTTP 202 means submitted. Do not claim final device state.

OpenAI assistant responses use `finish_reason: "stop"` and contain no `tool_calls`, preventing client re-execution.

Metadata uses `x_granite`, including:

```text
tool
arguments.vmid
executed
deduplicated
upstream_status
sanitized result for query/no-op
safe upstream code/request ID when relevant
warnings for ignored Agent categories
```

## 18. Synthesized Streaming

For `stream:true`, wait until Granite selection, validation, state check, and any upstream action complete. Then emit:

1. assistant role delta;
2. one complete UTF-8 Chinese content delta;
3. terminal `finish_reason:"stop"` with `x_granite`;
4. optional zero/estimated usage chunk;
5. `data: [DONE]`.

Never emit tool-call deltas because the managed action has already executed. Streaming does not reduce time to first byte.

## 19. Secure Logging

Disable go-zero request-dumping HTTP log middleware:

```yaml
Middlewares:
  Log: false
```

Install a safe access logger that records only:

```text
requestId
method
path
status
durationMs
contentLength
userAgent
```

Business logs may add:

```text
tool
vmid
upstreamStatus
upstreamCode
networkErrorClass
deduplicated
```

Never log:

- Authorization headers;
- API tokens;
- request body;
- Pi system prompt;
- history;
- client tools;
- full user text;
- full Granite prompt/response;
- raw upstream body.

llama-server stdout/stderr is not forwarded verbatim during normal operation. Controller emits only lifecycle states and sanitized exit status.

Existing leaked `NEEDLE_API_KEY` must be rotated; the new project uses a new `GRANITE_API_KEY`.

## 20. Configuration

```text
GRANITE_API_KEY                         required
GRANITE_MODEL_ID=granite-4.0-350m
GRANITE_STARTUP_TIMEOUT=120s
GRANITE_REQUEST_TIMEOUT=60s
GRANITE_THREADS=4                       range 1..16
GRANITE_CONTEXT_SIZE=4096               range 1024..8192
GRANITE_MAX_TOKENS=256                  bounded positive
GRANITE_MAX_MESSAGE_LENGTH=2048
GRANITE_ALLOW_FORCE_STOP=false
GRANITE_ACTION_DEDUP_WINDOW=30s

INFRA_CONTROL_API_BASE_URL              required
INFRA_CONTROL_API_TOKEN                 required
INFRA_CONTROL_TIMEOUT=30s
```

Retain the 8 MiB OpenAI HTTP envelope limit.

Remove all `NEEDLE_*` variables without compatibility fallback.

## 21. Container Build

Use four conceptual stages:

1. `granite-fetcher`: download pinned GGUF and verify SHA-256;
2. `llama-builder`: checkout pinned llama.cpp commit, build llama-server, hash and inspect binary;
3. `go-builder`: build `granite-controller`;
4. runtime: copy only approved binaries/model/config/licenses.

Runtime initially uses `debian:bookworm-slim` unless dependency inspection proves a smaller distroless base is complete and operationally useful.

Final image contains:

```text
/granite-controller
/opt/granite/llama-server
/opt/granite/granite-4.0-350m-Q4_K_M.gguf
/etc/granite-controller/config.yaml
/licenses/granite/*
/licenses/llama.cpp/*
```

It excludes Go, Git, C/C++ compilers, Python, Hugging Face client, Needle 2, and `libneedle.so`.

Runtime supports Linux AMD64, runs non-root, supports read-only root with `/tmp` tmpfs, and starts without external network.

## 22. Resource Baseline

Initial Kubernetes baseline:

```yaml
requests:
  cpu: "1"
  memory: "1Gi"
limits:
  cpu: "4"
  memory: "2Gi"
```

Run `llama-server` with one parallel slot and four threads. Validate actual RSS, startup time, request latency, and CPU before production sizing.

## 23. Test Strategy

### 23.1 Input and safety tests

- final user string/text parts;
- ignored Pi context/tools;
- one VM ID;
- condition/batch/sequence rejection;
- negated query allowed only when model chooses get;
- negated mutation rejected;
- ambiguous mutation rejected;
- semantic-evidence matrix.

### 23.2 Granite client/tool tests

- exact deterministic request;
- fixed bilingual tools;
- no/free-text/malformed/multiple call rejection;
- allowlisted name and strict VM ID;
- original/model ID mismatch;
- llama timeout/error/exit handling.

### 23.3 Process tests

- startup to ready;
- startup timeout exits;
- unexpected child exit exits controller;
- SIGTERM clean shutdown;
- no internal child restart;
- no raw child output leakage.

### 23.4 Infrastructure tests

- fixed GET/POST paths;
- state preconditions;
- force-stop disabled by default;
- redirects rejected;
- no mutation retries;
- upstream error parsing and Chinese mapping;
- field whitelist and uptime formatting.

### 23.5 Dedup tests

- concurrent duplicate waits once;
- success/no-op reused;
- explicit failure not cached;
- unknown network status cached;
- action and VM key isolation;
- query never deduplicated;
- expiry uses controllable clock.

### 23.6 Logging tests

Use sentinel secrets and bodies. Capture logs and assert absence of Authorization, Bearer value, system prompt, history, client tools, and full user content. Verify allowed metadata remains.

### 23.7 Granite Chinese acceptance suite

At least 20 natural variants per operation plus:

- negation;
- shutdown vs. force stop;
- ambiguity;
- multiple VM IDs;
- conditions/sequences;
- irrelevant requests;
- prompt injection;
- Pi Agent envelopes.

No wrong mutation may reach mock Infrastructure. Quality must be measured with real Granite Q4_K_M, not only fake responses.

### 23.8 Real authorized integration

With explicit authorization:

- query a VM;
- perform start on stopped VM;
- verify state;
- perform graceful shutdown on running VM if separately authorized;
- verify state transition asynchronously;
- never run force stop unless `GRANITE_ALLOW_FORCE_STOP=true` and explicitly authorized;
- clean credentials and containers.

## 24. Acceptance Criteria

`granite-controller v1.0.0` is ready when:

1. New GitHub/GitLab repositories preserve history and original `needle-controller` remains unchanged.
2. Runtime contains Granite Q4_K_M and pinned llama-server, with no Needle runtime.
3. Go manages child startup/readiness/failure/shutdown as specified.
4. Pi/OpenAI requests use only final user text; original Chinese reaches Granite unchanged.
5. Granite selects from five fixed bilingual tools through official llama.cpp OpenAI/Jinja tool calling.
6. Exactly one strict tool call and matching VM ID are required.
7. Generic condition/batch/negation/ambiguity safety policies prevent unsafe execution.
8. Query/start/shutdown/reboot work under state preconditions; force stop is disabled by default.
9. Mutation dedup prevents Pi retries from producing duplicate upstream calls.
10. Upstream errors are diagnosable without logging secrets or request bodies.
11. Query output uses whitelisted fields, readable uptime, and approved Emoji text.
12. Ordinary and SSE responses are OpenAI-compatible final assistant output without tool calls.
13. Real Granite Chinese acceptance tests produce zero wrong mock mutations.
14. Image runs Linux AMD64, non-root, read-only, offline, and within the 2 GiB baseline.
15. Unit, race, vet, process, model, integration, logging, image, and real authorized tests pass.
16. Artifact licenses/hashes/notices are verified before image publication.
17. Publishing Git `v1.0.0` and image requires explicit user authorization after release-candidate evidence.
