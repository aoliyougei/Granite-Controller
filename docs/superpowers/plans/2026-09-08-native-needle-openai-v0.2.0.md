# Native Needle OpenAI Service v0.2.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the remote `needle-openai` and PVE execution flow with one Linux AMD64 Go process that calls pinned Needle Engine 2.0.3 through CGO and exposes authenticated OpenAI-compatible model/tool-calling endpoints.

**Architecture:** Pure Go protocol code validates OpenAI messages and dynamic function schemas, creates an isolated replay plan, and submits it to a bounded dispatcher whose sole goroutine is locked to one OS thread. The dispatcher owns every native ABI call and maps the final native envelope to ordinary or synthesized-SSE OpenAI responses; the service never executes selected tools.

**Tech Stack:** Go 1.24, go-zero REST, CGO, pinned Needle Engine 2.0.3 `libneedle.so`, Linux AMD64, Docker multi-stage builds, distroless CC Debian 12.

**Spec:** `docs/superpowers/specs/2026-09-08-native-needle-openai-v0.2.0-design.md`

## Global Constraints

- Target release is `v0.2.0`; existing Git/image tag `v0.1.1` remains immutable.
- Production native support is exactly `linux/amd64` with CGO.
- Runtime is one Go process; no Python, FastAPI, Uvicorn, `needle-openai`, Hugging Face client, or remote Needle HTTP dependency.
- The server selects tools but never executes them and holds no Infrastructure Control URL or credential.
- Remove `POST /api/v1/chat`, PVE-specific normalization, `internal/infracontrol/`, and all `INFRA_CONTROL_*`, `CONTROLLER_API_TOKEN`, and `NEEDLE_BASE_URL` configuration.
- `GET /healthz` and `GET /readyz` are public; every `/v1/*` endpoint requires one exact `Authorization: Bearer <NEEDLE_API_KEY>` header.
- Every request resets, initializes, and fully replays independently; no cross-request native session reuse ships in v0.2.0.
- Every native ABI call runs serially on one dedicated goroutine after `runtime.LockOSThread()`.
- Queue waiting is cancelable; a native call already entered is not interrupted.
- Queue capacity defaults to 32; queue full maps to OpenAI HTTP 429 `engine_busy`.
- Model loads in the background; state is `loading -> ready|failed` and `ready -> failed`, with no in-process retry from `failed`.
- Dynamic tools use the strict schema allowlist in the spec; unsupported schemas fail before queueing or CGO.
- `/v1/chat/completions` requires tools, does not support `response_format`, accepts arbitrary text without built-in translation, and never applies confidence filtering.
- `stream: true` is synthesized after complete inference and ends with `data: [DONE]`.
- Engine artifact is exactly `Cactus-Compute/needle2/python/cactus_needle-2.0.3-py3-none-manylinux2014_x86_64.whl`; wheel and extracted library hashes are mandatory and version-controlled.
- Do not push an image containing `libneedle.so` until artifact-level redistribution terms are verified and notices are included.
- All dependency installation, tests, compilation, generation, native execution, image builds, and image inspection run through Docker API tools, never in the local Pi container.
- Follow TDD: observe a focused failing test before adding production behavior, then run focused and package-wide checks in Docker.
- Commit after each task; never commit downloaded wheels, `libneedle.so`, credentials, caches, or binaries.

## Planned File Map

- `build/needle-engine.env`: immutable engine coordinate and verified SHA-256 values.
- `build/fetch-needle.py`: standard-library-only deterministic fetch/extract/hash script used only during image build.
- `native/include/needle.h`: reviewed ABI declaration matching upstream ctypes signatures.
- `internal/native/abi.go`: testable ABI interface and error values.
- `internal/native/abi_linux_amd64.go`: CGO production binding with RPATH.
- `internal/native/abi_stub.go`: unsupported-platform implementation that never becomes ready.
- `internal/native/types.go`: replay request, turn, native envelope, state, and dispatcher result types.
- `internal/native/engine.go`: reset/init/replay/buffer/native-envelope logic, independent of HTTP.
- `internal/native/dispatcher.go`: background initialization, locked thread, queue, cancellation, and state.
- `internal/openai/request.go`: permissive top-level wire types and strict supported-field validation.
- `internal/openai/schema.go`: dynamic tool and JSON Schema allowlist validation.
- `internal/openai/replay.go`: OpenAI history to isolated native replay plan.
- `internal/openai/response.go`: native envelope to OpenAI completion/error models.
- `internal/openai/streaming.go`: synthesized SSE event generation.
- `internal/handler/models.go`, `chat.go`, `ready.go`, `routes.go`: v0.2.0 HTTP endpoints.
- `internal/middleware/auth.go`: reused constant-time Bearer middleware, adapted to OpenAI errors.
- `internal/config/config.go`: new native/OpenAI-only configuration.
- `internal/svc/context.go`: dispatcher and OpenAI service composition.
- `Dockerfile`, `.dockerignore`, `docker-compose.yml`, `.env.example`, `Makefile`: pinned native build and runtime.
- `licenses/Apache-2.0.txt`, `licenses/THIRD_PARTY_NOTICES.md`: redistribution artifacts.
- `README.md`, `CHANGELOG.md`, `needle-controller.api`: v0.2.0 API and migration documentation.

---

### Task 1: Artifact and License Gate

**Files:**
- Create: `build/needle-engine.env`
- Create: `build/fetch-needle.py`
- Create: `build/fetch-needle_test.py`
- Create: `licenses/Apache-2.0.txt`
- Create: `licenses/THIRD_PARTY_NOTICES.md`
- Modify: `.gitignore`

**Interfaces:**
- Produces build variables `NEEDLE_ENGINE_VERSION`, `NEEDLE_HF_REPO`, `NEEDLE_WHEEL_PATH`, `NEEDLE_WHEEL_SHA256`, and `NEEDLE_LIB_SHA256`.
- Produces `fetch-needle.py --source URL_OR_FILE --wheel-sha256 HEX --lib-sha256 HEX --out DIR`, writing only verified `libneedle.so` and copied artifact notices.
- Produces a written redistribution decision in `THIRD_PARTY_NOTICES.md`; later image-push work is blocked unless it says the checked artifact permits the intended distribution.

- [ ] **Step 1: Probe the exact Hugging Face artifact without changing source**

Use Docker/fetch tooling to retrieve repository metadata and the exact wheel named in the spec. Record HTTP final URL, content length, repository license metadata, wheel `*.dist-info/METADATA`, and names of `LICENSE*`, `NOTICE*`, and `COPYING*` members. Never infer wheel license solely from the Git repository.

Expected coordinate:

```text
repo: Cactus-Compute/needle2
path: python/cactus_needle-2.0.3-py3-none-manylinux2014_x86_64.whl
```

If the artifact cannot be fetched or redistribution permission remains ambiguous, stop and report the licensing blocker before implementing image publication. Local technical work may proceed only after explicitly recording that image publication remains blocked.

- [ ] **Step 2: Compute hashes independently twice**

In two fresh task-owned containers/downloads, run:

```text
sha256sum cactus_needle-2.0.3-py3-none-manylinux2014_x86_64.whl
unzip -p cactus_needle-2.0.3-py3-none-manylinux2014_x86_64.whl needle/libneedle.so | sha256sum
```

Expected: both downloads yield identical wheel hashes and both extractions yield identical library hashes. Put the literal verified hashes—not placeholders—into `build/needle-engine.env`.

- [ ] **Step 3: Write failing fetch-script tests**

Using Python `unittest`, construct tiny temporary ZIP fixtures and test:

```python
class FetchNeedleTests(unittest.TestCase):
    def test_extracts_only_verified_library_and_notices(self): ...
    def test_rejects_wrong_wheel_hash_before_extraction(self): ...
    def test_rejects_wrong_library_hash(self): ...
    def test_rejects_missing_or_duplicate_library_member(self): ...
    def test_rejects_zip_path_traversal(self): ...
```

The production change each test catches is accepting mutable or malicious build input.

- [ ] **Step 4: Run fetch tests in a Python Docker container and verify RED**

Run:

```text
python -m unittest -v build/fetch-needle_test.py
```

Expected: FAIL because the fetch/extract implementation does not exist.

- [ ] **Step 5: Implement the standard-library fetcher**

Use only `argparse`, `hashlib`, `pathlib`, `urllib.request`, and `zipfile`. Stream the source to a temporary file, verify the wheel SHA-256 before opening it, require exactly one `needle/libneedle.so`, hash extracted bytes, copy only the verified library and recognized notice files, fsync, then rename atomically. Never execute wheel code.

- [ ] **Step 6: Run fetch tests and a real artifact extraction in Docker**

Run the unittest command, then execute the fetcher against the exact artifact and assert:

```text
sha256sum output/libneedle.so == NEEDLE_LIB_SHA256
find output -type f contains only libneedle.so and approved notices
```

Do not copy the output into the Git workspace.

- [ ] **Step 7: Record notices and publication decision**

Copy Apache-2.0 from `source/needle/LICENSE` into `licenses/Apache-2.0.txt`. `THIRD_PARTY_NOTICES.md` must identify Cactus Compute, the source repository, Hugging Face coordinate, engine/wheel versions, both hashes, artifact metadata license, included notice files, and whether redistribution to the intended registry is permitted. Do not claim legal certainty beyond inspected evidence.

- [ ] **Step 8: Commit the gate**

```bash
git add build licenses .gitignore
git commit -m "build: pin Needle engine artifact"
```

### Task 2: v0.2.0 Configuration and OpenAI Error Contract

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/apierror/error.go`
- Modify: `internal/apierror/error_test.go`
- Modify: `internal/types/types.go`
- Modify: `etc/needle-controller.yaml`
- Modify: `.env.example`

**Interfaces:**
- Produces `config.Config` with `APIKey string` and `Needle NativeConfig`.
- Produces `NativeConfig { ModelID string; MaxNewTokens, MaxQueueDepth, MaxReplaySteps, BufferSize int; ToolIndexPath string; SlowCall time.Duration }`.
- Produces OpenAI error body `types.OpenAIErrorResponse` and constructor `apierror.OpenAI(code, message, param string, status int, cause error)`.
- Removes every Infrastructure Control and remote Needle client configuration field.

- [ ] **Step 1: Replace old config tests with failing v0.2.0 tests**

Test required `NEEDLE_API_KEY`, defaults, all environment overrides, buffer range `[65536, 8388608]`, positive generation/queue/replay values, positive slow-call duration, non-empty model ID, and rejection of every removed variable as ineffective—not accepted fallback configuration. Assert error strings never contain API key values.

- [ ] **Step 2: Write failing OpenAI error-envelope tests**

Assert literal output shape:

```json
{"error":{"message":"needle-2 requires at least one function tool","type":"invalid_request_error","param":"tools","code":"tools_required"}}
```

Verify wrapped causes remain available through `errors.Is` but never enter JSON.

- [ ] **Step 3: Run focused tests in Docker and verify RED**

```text
go test ./internal/config ./internal/apierror -count=1
```

Expected: FAIL because current configuration requires Controller/Infrastructure/remote Needle fields and current errors use the custom v0.1.x envelope.

- [ ] **Step 4: Implement minimal config and error migration**

Keep `rest.RestConf`; remove `InfraControlConfig` and remote `BaseURL/APIKey/Timeout`. Parse only variables listed in spec section 19. Default top-level REST timeout must allow queue plus native execution and be documented in milliseconds.

- [ ] **Step 5: Update YAML and environment example**

`.env.example` contains only:

```text
NEEDLE_API_KEY=replace-with-a-long-random-api-key
NEEDLE_MODEL_ID=needle-2
NEEDLE_MAX_NEW_TOKENS=256
NEEDLE_MAX_QUEUE_DEPTH=32
NEEDLE_MAX_REPLAY_STEPS=32
NEEDLE_BUFFER_SIZE=1048576
NEEDLE_TOOL_INDEX_PATH=
NEEDLE_ENGINE_SLOW_CALL=30s
```

No Infrastructure or old Controller credential remains.

- [ ] **Step 6: Run focused and full tests in Docker**

```text
go test ./internal/config ./internal/apierror -count=1
go test ./... -count=1
```

Tests outside the migrated boundary may still fail to compile because old service wiring consumes removed fields; record those expected failures for later removal, but focused packages must pass.

- [ ] **Step 7: Commit configuration migration**

```bash
git add internal/config internal/apierror internal/types etc/needle-controller.yaml .env.example
git commit -m "feat: define native OpenAI service configuration"
```

### Task 3: Strict Dynamic Tool Schema Validation

**Files:**
- Create: `internal/openai/request.go`
- Create: `internal/openai/schema.go`
- Create: `internal/openai/schema_test.go`

**Interfaces:**
- Produces permissive `openai.ChatCompletionRequest` wire type with raw/optional fields needed to distinguish absence from zero.
- Produces `openai.FunctionTool`, `FunctionDefinition`, and validated native schema types.
- Produces `func ValidateAndSelectTools(req ChatCompletionRequest, limits SchemaLimits) ([]NativeTool, []string, *apierror.Error)`.
- Produces `SchemaLimits { MaxTools, MaxCatalogBytes, MaxDescriptionRunes, MaxDepth, MaxProperties int }` with spec defaults.

- [ ] **Step 1: Write failing valid-schema table tests**

Cover every allowed type and keyword with hand-written literals: object/properties/required, array/items, string constraints, integer/number constraints, boolean, enum, const, and `additionalProperties:false`.

- [ ] **Step 2: Write failing rejection table tests**

Cover zero tools, over 64 tools, non-function type, invalid/duplicate names, overlong descriptions, over-256-KiB catalog, depth over 8, over 64 properties, unknown keywords, `$ref`, `$defs`, `definitions`, combinators, conditionals, missing item schemas, malformed required lists, `additionalProperties:true/object`, and mismatched keyword/type values.

- [ ] **Step 3: Write failing `tool_choice` tests**

Assert absent/`auto` keeps all tools, named choice narrows to exactly one existing tool, unknown named choice is 400, `required` keeps all and returns a literal warning, and `none` returns `tools_required`.

- [ ] **Step 4: Run schema tests in Docker and verify RED**

```text
go test ./internal/openai -run 'TestSchema|TestToolChoice' -count=1
```

Expected: FAIL because package behavior is absent.

- [ ] **Step 5: Implement recursive allowlist validation**

Decode schemas with `json.Decoder.UseNumber`, require exactly one JSON value, iterate keys explicitly, enforce keyword types and depth/property limits, reject duplicates where token-level parsing is required, preserve approved schemas without lossy rewriting, then serialize the Needle flat tool form `{name,description,parameters}`.

- [ ] **Step 6: Run focused tests and mutation review**

Run tests and mentally mutate each limit, allowed-key set, duplicate-name check, and `additionalProperties` branch; ensure at least one test fails for each realistic mutation.

- [ ] **Step 7: Commit schema validation**

```bash
git add internal/openai/request.go internal/openai/schema.go internal/openai/schema_test.go
git commit -m "feat: validate dynamic OpenAI function tools"
```

### Task 4: Message Validation and Native Replay Planning

**Files:**
- Create: `internal/openai/replay.go`
- Create: `internal/openai/replay_test.go`
- Modify: `internal/openai/request.go`

**Interfaces:**
- Produces `native.Request { System string; ToolsJSON []byte; Turns []native.Turn; MaxNewTokens int; ToolIndexPath string }`.
- Produces `native.Turn { Kind native.TurnKind; Text string }`, with `TurnUser` and `TurnToolResults`.
- Produces `func BuildReplay(req ChatCompletionRequest, effectiveTools []NativeTool, cfg config.NativeConfig) (native.Request, []string, *apierror.Error)`.

- [ ] **Step 1: Write failing role/content tests**

Test ordered joining of system/developer text, user turns, assistant omission, string and text-part flattening, rejected image/audio/file/unknown parts, rejected non-text tool results, empty history, and history with no replayable turn.

- [ ] **Step 2: Write failing tool-result grouping tests**

For consecutive tool messages with complete realistic OpenAI fields, assert one literal JSON-array turn in source order. Verify a following user message flushes pending tool results and starts a new turn.

- [ ] **Step 3: Write failing replay-limit and option tests**

Assert more than configured complete-call steps returns `replay_limit_exceeded` before native submission; validate model exact match, positive/max-bounded token fields, conflict between `max_tokens` and `max_completion_tokens`, `n > 1`, `response_format`, and ignored sampling warnings.

- [ ] **Step 4: Run replay tests in Docker and verify RED**

```text
go test ./internal/openai -run 'TestBuildReplay|TestMessage' -count=1
```

- [ ] **Step 5: Implement pure replay construction**

Never mutate the request. Join system/developer messages with newline, drop assistant messages, group tool results as JSON strings/objects without including authorization data, apply effective tools, and ensure every error happens before queueing.

- [ ] **Step 6: Run all OpenAI package tests**

```text
go test ./internal/openai -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit replay planning**

```bash
git add internal/openai/request.go internal/openai/replay.go internal/openai/replay_test.go
git commit -m "feat: build isolated Needle replay plans"
```

### Task 5: Native ABI Boundary and Engine Core

**Files:**
- Create: `native/include/needle.h`
- Create: `internal/native/abi.go`
- Create: `internal/native/abi_linux_amd64.go`
- Create: `internal/native/abi_stub.go`
- Create: `internal/native/types.go`
- Create: `internal/native/engine.go`
- Create: `internal/native/engine_test.go`

**Interfaces:**
- Produces `native.ABI { Init(system, toolsJSON, toolIndexPath []byte) int; Complete(text []byte, maxNewTokens int, output []byte) int; Reset() }`.
- Produces `native.Engine.Execute(request Request) (Envelope, error)`; this method is called only by dispatcher loop.
- Produces typed `native.Envelope`, `FunctionCall`, `Validation`, and metrics matching the raw engine contract.
- Production ABI constructor is `native.NewABI() (ABI, error)`.

- [ ] **Step 1: Add the reviewed C header**

Copy exactly the four signatures from spec section 10, including `<stdint.h>`. Add an attribution comment pointing to pinned `source/needle/needle/__init__.py` ctypes declarations. Do not invent extra ABI functions.

- [ ] **Step 2: Write failing fake-ABI engine tests**

A recording fake returns literal envelopes. Test exact call order:

```text
Reset -> Init -> Complete(turn1) -> Complete(turn2)
```

Also test tools/system/index bytes, final-envelope selection, multiple calls, confidence/validation/metrics parsing, negative init, negative complete, malformed JSON, missing NUL, and reset after failure.

- [ ] **Step 3: Run native engine tests without CGO and verify RED**

```text
CGO_ENABLED=0 go test ./internal/native -run TestEngine -count=1
```

Expected: FAIL because engine types do not exist; tests use fake ABI and must remain runnable without the real library.

- [ ] **Step 4: Implement engine core and bounded buffer behavior**

Allocate the configured buffer once per Engine, clear it before every call, locate the first NUL, reject no-NUL output, parse only the prefix, validate returned function names against request tools, and never include raw buffer content in errors.

- [ ] **Step 5: Implement production and stub ABI files**

Production CGO directives use the checked library at build time and:

```text
-L... -lneedle -Wl,-rpath,/opt/needle
```

Convert Go bytes to NUL-terminated C strings while rejecting embedded NUL. Stub constructor returns the literal unsupported-platform error. No test imports `C` directly.

- [ ] **Step 6: Run fake-ABI tests and compile the stub path**

```text
CGO_ENABLED=0 go test ./internal/native -count=1
```

Expected: PASS, with state unavailable through stub only when production constructor is used.

- [ ] **Step 7: Compile/link a production probe in Docker with verified library**

Use the Task 1 verified extraction as an ephemeral build input, run `go test`/a narrow integration binary with `CGO_ENABLED=1`, and inspect with:

```text
ldd /out/needle-controller
readelf -d /out/needle-controller
```

Verify `libneedle.so` and RPATH `/opt/needle`; record all other dynamic dependencies for Docker runtime validation.

- [ ] **Step 8: Commit ABI and engine core**

```bash
git add native/include internal/native
git commit -m "feat: bind the native Needle engine"
```

### Task 6: Locked-Thread Dispatcher and Lifecycle

**Files:**
- Create: `internal/native/state.go`
- Create: `internal/native/dispatcher.go`
- Create: `internal/native/dispatcher_test.go`

**Interfaces:**
- Produces `native.State` constants `loading`, `ready`, `failed`.
- Produces `Dispatcher.Submit(ctx context.Context, request Request) (Envelope, error)`.
- Produces `Dispatcher.State() State`, `Dispatcher.FailureSummary() string`, and `Dispatcher.Close()`.
- Produces typed errors `ErrNotReady`, `ErrQueueFull`, and engine-operation errors.

- [ ] **Step 1: Write failing lifecycle tests**

Use a controllable fake engine to verify asynchronous `loading -> ready`, `loading -> failed`, no reinitialization after failed load, and safe concurrent `State()` access.

- [ ] **Step 2: Write failing queue and cancellation tests**

With queue size one and blocking fake operations, verify queue full, FIFO serialization, canceled waiting job never reaches engine, cancellation after execution begins lets engine finish, and the next request starts only after completion.

- [ ] **Step 3: Write failing failure-state tests**

Verify init-fatal errors mark failed, ordinary complete errors fail only that request when engine reports recovery, sanitized failure summary excludes request text/tools/native buffers, and `Close` drains/stops without racing submitters.

- [ ] **Step 4: Run dispatcher tests and verify RED**

```text
go test ./internal/native -run TestDispatcher -count=1
```

- [ ] **Step 5: Implement the single worker**

Call `runtime.LockOSThread()` at the first line of the worker goroutine before constructing/initializing the production engine. Use a nonblocking queue send for capacity, a response channel buffered to one so canceled callers cannot block the worker, and check context immediately before execution.

- [ ] **Step 6: Run dispatcher tests under race detector**

In a Docker container with a C toolchain:

```text
CGO_ENABLED=1 go test -race ./internal/native -run TestDispatcher -count=1
```

Expected: PASS with no races or overlapping fake calls.

- [ ] **Step 7: Commit dispatcher**

```bash
git add internal/native/state.go internal/native/dispatcher.go internal/native/dispatcher_test.go
git commit -m "feat: serialize Needle calls on one native thread"
```

### Task 7: OpenAI Completion and Model Response Mapping

**Files:**
- Create: `internal/openai/response.go`
- Create: `internal/openai/response_test.go`
- Create: `internal/openai/service.go`
- Create: `internal/openai/service_test.go`

**Interfaces:**
- Produces `openai.Service.Complete(ctx, ChatCompletionRequest) (ChatCompletionResponse, error)`.
- Produces `openai.ModelList(modelID string)`.
- Produces response structs for completion, choice, assistant message, tool call, zero/estimated usage, and `x_needle`.
- Consumes a narrow `NativeCompleter { Submit(context.Context, native.Request) (native.Envelope, error); State() native.State }`.

- [ ] **Step 1: Write failing model-list and tool-call mapping tests**

Use literal expected structures to assert one model, ownership, one/multiple tool calls, JSON-string arguments, generated IDs with correct prefixes, `finish_reason:tool_calls`, zero estimated usage, and exact native safety/metrics preservation.

- [ ] **Step 2: Write failing no-call and low-confidence tests**

Assert no-call yields `content:null`, omitted calls, `finish_reason:stop`, reasoning only under `x_needle`; low confidence, non-empty ungrounded, and negation are returned unchanged rather than filtered.

- [ ] **Step 3: Write failing service error-mapping tests**

Assert not-ready -> 503 `model_not_ready`, queue full -> 429 `engine_busy`, engine failure -> 502 `engine_error`, and every request/schema/replay error retains its OpenAI parameter/code.

- [ ] **Step 4: Run response/service tests and verify RED**

```text
go test ./internal/openai -run 'TestResponse|TestService|TestModelList' -count=1
```

- [ ] **Step 5: Implement minimal mapping and service orchestration**

Flow: validate/select tools -> build replay -> dispatcher submit -> map final envelope. IDs use cryptographic randomness with safe fallback; arguments use deterministic `json.Marshal` of native objects; no business-specific validation or execution is added.

- [ ] **Step 6: Run all OpenAI/native tests**

```text
go test ./internal/openai ./internal/native -count=1
```

- [ ] **Step 7: Commit protocol mapping**

```bash
git add internal/openai/response.go internal/openai/response_test.go internal/openai/service.go internal/openai/service_test.go
git commit -m "feat: map native Needle calls to OpenAI responses"
```

### Task 8: Synthesized SSE

**Files:**
- Create: `internal/openai/streaming.go`
- Create: `internal/openai/streaming_test.go`

**Interfaces:**
- Produces `func StreamEvents(response ChatCompletionResponse, includeUsage bool) []SSEEvent`.
- Produces `SSEEvent.Data []byte`; final event data is exactly `[DONE]`.

- [ ] **Step 1: Write failing single/multiple-call SSE tests**

Assert literal event order: role, each tool-call identity/name, argument fragments, finish reason carrying `x_needle`, optional usage, `[DONE]`. Reassemble fragments and compare exact argument JSON. Assert all JSON chunks share response ID/model/created values.

- [ ] **Step 2: Write failing no-call and usage-option tests**

No-call emits role then terminal `stop`; usage chunk exists only when requested; no event exposes reasoning as content.

- [ ] **Step 3: Run streaming tests and verify RED**

```text
go test ./internal/openai -run TestStreamEvents -count=1
```

- [ ] **Step 4: Implement deterministic event synthesis**

Split argument strings on byte-safe boundaries without breaking UTF-8, or emit one argument delta when fragmentation adds no compatibility value. Each SSE payload is compact JSON and the renderer adds exactly `data: <payload>\n\n`.

- [ ] **Step 5: Run OpenAI tests**

```text
go test ./internal/openai -count=1
```

- [ ] **Step 6: Commit SSE support**

```bash
git add internal/openai/streaming.go internal/openai/streaming_test.go
git commit -m "feat: synthesize OpenAI tool-call streams"
```

### Task 9: HTTP Endpoints, Authentication, and Wiring

**Files:**
- Modify: `internal/middleware/auth.go`
- Modify: `internal/middleware/auth_test.go`
- Delete: `internal/handler/chat.go`
- Delete: old `internal/handler/handler_test.go`
- Create: `internal/handler/models.go`
- Create: `internal/handler/chat.go`
- Create: `internal/handler/ready.go`
- Create: `internal/handler/openai_test.go`
- Modify: `internal/handler/routes.go`
- Modify: `internal/svc/context.go`
- Modify: `internal/svc/context_test.go`
- Modify: `cmd/needle-controller/main.go`
- Modify: `needle-controller.api`

**Interfaces:**
- Public handlers: `Health()`, `Ready(stateProvider)`.
- Protected handlers: `Models(openai.Service)`, `ChatCompletions(openai.Service)`.
- `svc.ServiceContext` owns config, dispatcher, and OpenAI service only.
- `handler.Register` exposes exactly `/healthz`, `/readyz`, `/v1/models`, and `/v1/chat/completions`.

- [ ] **Step 1: Write failing auth tests for OpenAI envelopes**

Keep exact-token constant-time coverage, but assert missing/wrong keys return OpenAI 401 `invalid_api_key`. Public probes bypass auth; both `/v1/*` routes require it.

- [ ] **Step 2: Write failing endpoint tests**

Cover model loading/ready/failed readiness statuses, authenticated model list, strict malformed JSON handling with permissive unknown top-level fields, ordinary completion JSON, SSE headers/events, unsupported request errors, queue errors, and no `/api/v1/chat` route.

- [ ] **Step 3: Run handler/middleware tests and verify RED**

```text
go test ./internal/handler ./internal/middleware ./internal/svc -count=1
```

Expected: FAIL because current endpoints and service context are PVE-specific.

- [ ] **Step 4: Implement handlers and production wiring**

Start HTTP before background initialization. Construct ABI/engine/dispatcher through testable factory functions; if ABI construction fails, expose `failed` readiness without crashing before `/healthz` can serve. Chat handler computes complete response first, then writes JSON or synthesized SSE; canceled requests do not write fallback bodies.

- [ ] **Step 5: Update go-zero API declaration**

Document only the four routes and OpenAI request/response surface. The `.api` file may use permissive/raw JSON notes where goctl types cannot faithfully express dynamic schemas; runtime Go structs remain authoritative.

- [ ] **Step 6: Run focused and full tests**

```text
go test ./internal/handler ./internal/middleware ./internal/svc ./cmd/needle-controller -count=1
go test ./... -count=1
```

- [ ] **Step 7: Commit HTTP migration**

```bash
git add -A internal/handler internal/middleware internal/svc cmd/needle-controller needle-controller.api
git commit -m "feat: expose native OpenAI tool-calling endpoints"
```

### Task 10: Remove PVE Execution and Obsolete Code

**Files:**
- Delete: `internal/infracontrol/`
- Delete: PVE-specific files under `internal/logic/`
- Delete: obsolete remote-Needle client/model/validator files under `internal/needle/`
- Delete or migrate: obsolete `internal/types/` DTOs
- Modify: tests and imports throughout repository

**Interfaces:**
- Removes every code path capable of calling Infrastructure Control.
- Removes fixed `pve_vm_start`, Chinese normalization, confidence execution gate, and remote HTTP Needle service.
- Leaves dynamic tools solely in `internal/openai` and native execution solely in `internal/native`.

- [ ] **Step 1: Add a failing black-box route/capability test before deletion**

Test that the production router returns 404 for `/api/v1/chat` and that a dynamic `pve_vm_start` completion returns a tool call while an external test server receives zero requests. This proves “select, never execute” behavior rather than merely grepping source.

- [ ] **Step 2: Run the black-box test and verify RED against old route wiring if still present**

```text
go test ./internal/e2e -run TestServerNeverExecutesTools -count=1
```

- [ ] **Step 3: Delete obsolete production and test code**

Remove packages only after Tasks 2–9 have replacements. Do not retain compatibility aliases for old environment variables or endpoint paths.

- [ ] **Step 4: Run all tests and static source checks**

```text
go test ./... -count=1
go vet ./...
```

Additionally run non-executing checks that no production file references `INFRA_CONTROL_`, `CONTROLLER_API_TOKEN`, `NEEDLE_BASE_URL`, `/api/v1/chat`, or the old fixed-tool client, except migration/release documentation.

- [ ] **Step 5: Commit breaking removal**

```bash
git add -A
git commit -m "refactor: remove infrastructure tool execution"
```

### Task 11: Native Docker Image and Offline Runtime

**Files:**
- Modify: `Dockerfile`
- Modify: `.dockerignore`
- Modify: `docker-compose.yml`
- Modify: `Makefile`
- Create: `internal/native/integration_test.go` with an explicit integration build tag

**Interfaces:**
- Produces one `linux/amd64` image running non-root with `/opt/needle/libneedle.so` and no language toolchains.
- Produces `make test`, `race`, `vet`, `native-test`, and `image` commands aligned to v0.2.0.

- [ ] **Step 1: Write the real-native integration test**

Under an explicit build tag, initialize one `pve_vm_start` schema and assert `Start VM 3052` returns exactly one call named `pve_vm_start` with integer `vmid` 3052 and non-nil confidence. Add a second independent request with a different ID and assert no state leakage.

- [ ] **Step 2: Run native integration test before image changes and verify RED**

In Linux AMD64 Docker with CGO:

```text
go test -tags=needle_native ./internal/native -run TestRealNeedle -count=1
```

Expected: FAIL to link/find the pinned library in the current image flow.

- [ ] **Step 3: Implement the three-stage Docker build**

Fetcher reads immutable `build/needle-engine.env`, downloads and verifies the wheel/library, and emits notices. Go builder copies verified library for linking, builds with CGO, and runs `ldd`/`readelf` assertions. Runtime is pinned `distroless/cc-debian12:nonroot`, copies only approved files, exposes 8080, and retains the binary healthcheck subcommand.

- [ ] **Step 4: Update Compose hardening and config**

Use target version `0.2.0`, only `NEEDLE_*` environment fields, read-only root, `/tmp` tmpfs, no-new-privileges, all capabilities dropped, and healthcheck. Remove cache volumes and Infrastructure/remote-Needle values.

- [ ] **Step 5: Build and run native tests through Docker API**

Build the image and a test target, run real-native integration, inspect architecture/user/files/dynamic dependencies, and verify no Python/pip/go/huggingface executable/module exists in runtime.

- [ ] **Step 6: Verify offline startup**

Create the final container with network mode `none`, a test API key, read-only root, tmpfs, and dropped capabilities. Confirm `/needle-controller healthcheck`, model loading to ready, and a local in-container/native probe without external network. If HTTP smoke requires a client, use a separate same-network test arrangement before switching to `none`, or add a tested binary self-check command rather than installing curl.

- [ ] **Step 7: Commit image migration**

```bash
git add Dockerfile .dockerignore docker-compose.yml Makefile internal/native/integration_test.go
git commit -m "build: embed the pinned Needle engine"
```

### Task 12: End-to-End OpenAI Agent Loop

**Files:**
- Rewrite: `internal/e2e/e2e_test.go`
- Create: `internal/e2e/stream_test.go`

**Interfaces:**
- Tests complete production router/service composition with fake ABI for exhaustive cases and real ABI under integration tag for acceptance.

- [ ] **Step 1: Write failing ordinary agent-loop test**

First request declares `get_weather`, receives an OpenAI tool call, then a second request supplies full history plus a `tool` result. Assert replay omits assistant input, groups tool results, and returns the fake native final no-call response without executing any Go tool.

- [ ] **Step 2: Write failing multi-agent isolation and concurrency tests**

Interleave two distinct toolsets/messages. Assert each request performs its own reset/init and never sees the other's schema/history. Block fake ABI to prove HTTP concurrency does not cause native overlap and queue overflow produces 429.

- [ ] **Step 3: Write failing SSE black-box test**

Use `httptest.Server` and an OpenAI-shaped streaming request; parse SSE lines, reassemble calls, assert headers, final reason, optional usage, and `[DONE]`.

- [ ] **Step 4: Run E2E tests and verify RED where integration is incomplete**

```text
go test ./internal/e2e -count=1
```

If they pass immediately through existing public constructors, preserve them and do not manufacture a failure.

- [ ] **Step 5: Make only integration-boundary adjustments**

Expose testable constructors/factories without adding test-only production methods or alternate public execution paths.

- [ ] **Step 6: Run full race/static verification**

In Docker with CGO toolchain and fake ABI path:

```text
go test ./... -count=1
CGO_ENABLED=1 go test -race ./... -count=1
go vet ./...
```

Then run real-native integration separately with pinned library.

- [ ] **Step 7: Commit E2E acceptance tests**

```bash
git add internal/e2e
git commit -m "test: verify native OpenAI agent loops"
```

### Task 13: Documentation, Migration, and v0.2.0 Release Candidate

**Files:**
- Rewrite: `README.md`
- Create or modify: `CHANGELOG.md`
- Modify: `docs/api.md` to clarify it is an external-agent tool source, not server-executed routes
- Modify: `.env.example`
- Modify: `docker-compose.yml`

**Interfaces:**
- Documents the exact public contract and breaking migration from v0.1.1.
- Does not publish or tag until all verification and licensing gates pass.

- [ ] **Step 1: Rewrite README around the actual service**

Include: tool-calling-not-chat warning, dynamic tool example, OpenAI SDK/curl examples, multi-turn tool loop, synthesized streaming, `x_needle` policy, Chinese-quality caveat, schema compatibility table, authentication, loading/readiness, Linux AMD64, offline runtime, engine hashes, and explicit “server never executes tools.”

- [ ] **Step 2: Add exact breaking release notes**

Include the spec's BREAKING block, upgrade examples, removed variables/routes, the immutable v0.1.1 fallback, and operator action to move Infrastructure execution into the external agent.

- [ ] **Step 3: Validate documentation examples against a running image**

Run every documented curl/OpenAI payload against controlled fake/native test deployments. Assert status and shape; never validate prose by grep alone.

- [ ] **Step 4: Run final repository safety review**

```text
git status --short
git diff --check
git grep -nE 'INFRA_CONTROL_|CONTROLLER_API_TOKEN|NEEDLE_BASE_URL|/api/v1/chat' -- ':!docs/superpowers/**' ':!CHANGELOG.md'
git grep -nE '(sk-[A-Za-z0-9]{16,}|Bearer [A-Za-z0-9_-]{16,})' -- .
```

Expected: no obsolete production reference and no credentials. Confirm no wheel or `.so` is tracked.

- [ ] **Step 5: Commit documentation**

```bash
git add README.md CHANGELOG.md docs/api.md .env.example docker-compose.yml
git commit -m "docs: describe the v0.2.0 OpenAI migration"
```

- [ ] **Step 6: Request independent code review**

Use the requesting-code-review workflow on the complete range from the v0.1.1 commit `ae1b2c1` to feature HEAD. Fix Critical/Important findings with regression tests and rerun affected Docker checks.

- [ ] **Step 7: Perform fresh release-candidate verification**

Apply verification-before-completion. Freshly run unit, race, vet, real-native, no-network runtime, non-root/read-only, schema, SSE, and agent-loop checks. Build `needle-controller:0.2.0` with no cache and record image digest.

- [ ] **Step 8: Enforce publication gate**

Only if `licenses/THIRD_PARTY_NOTICES.md` records sufficient artifact redistribution evidence may the image be tagged/pushed. Otherwise report: Git implementation ready, image publication blocked by license verification. Do not override this gate based on prior source Apache-2.0 alone.

- [ ] **Step 9: Finish branch without assuming release authorization**

Use finishing-a-development-branch. Present local merge / PR / keep options. Creating `v0.2.0`, pushing Git, or pushing the container image requires the user's explicit release request after release-candidate results are reported.
