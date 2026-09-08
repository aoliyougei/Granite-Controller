# OpenAI Managed VM Start v0.3.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `/v1/chat/completions` accept Agent-style OpenAI requests, use only the final plain-text Chinese user message, require embedded Needle 2 to select the fixed `pve_vm_start` tool, execute one authenticated Infrastructure Control start request, and return a final Chinese OpenAI assistant response.

**Architecture:** A managed-input layer ignores Agent history/tools, strictly validates and normalizes the final user command, then submits one fixed tool schema to the existing locked-thread native Dispatcher. A managed safety validator cross-checks Needle's single call against the original VM ID before a narrow no-retry Infrastructure Control client executes it; ordinary and SSE responders synthesize final text and never return an already-executed tool call.

**Tech Stack:** Go 1.24, go-zero REST, existing CGO Needle Engine 2.0.3 integration, standard-library HTTP client, Docker Linux AMD64, distroless CC Debian 12.

**Spec:** `docs/superpowers/specs/2026-09-08-openai-managed-vm-start-v0.3.0-design.md`

## Global Constraints

- Target release is `v0.3.0`; immutable `v0.2.0` remains the general client-provided tool-calling release and `v0.1.1` remains the legacy `/api/v1/chat` release.
- Public routes remain `GET /healthz`, `GET /readyz`, `GET /v1/models`, and `POST /v1/chat/completions`.
- `/v1/*` uses the existing required `NEEDLE_API_KEY` Bearer authentication.
- Only `messages[len(messages)-1]` may influence execution; it must be a non-empty plain JSON string with role `user`.
- Never search backward for an earlier user message.
- Ignore earlier messages, client `tools`, `tool_choice`, `response_format`, sampling fields, and unknown top-level fields; report stable category warnings without logging or echoing ignored content.
- The only native tool is fixed `pve_vm_start`; clients cannot add, replace, or weaken it.
- Chinese rules validate and normalize but never execute. Embedded Needle 2 must be called and must select the same VM ID.
- Automatic execution requires one `pve_vm_start` call, exactly one positive integer `vmid`, exact original/model ID match, confidence at least `NEEDLE_MIN_CONFIDENCE` (default `0.6`), empty `ungrounded`, and `negation:false`.
- Upstream configuration names are exactly `INFRA_CONTROL_API_BASE_URL`, `INFRA_CONTROL_API_TOKEN`, and `INFRA_CONTROL_TIMEOUT` (default `30s`).
- Upstream method/path/auth are fixed; redirects and automatic retries are forbidden; only HTTP 202 is success.
- Success response is final assistant content `VM <id> 的启动请求已提交。`, finish reason `stop`, no `tool_calls`, and `x_needle.executed:true`.
- SSE is synthesized only after model validation and upstream execution; it emits role, content, terminal metadata, optional usage, and `[DONE]`, never tool-call deltas.
- All dependency resolution, tests, builds, native execution, image operations, and live verification run through Docker API tools.
- Follow TDD and commit each independently reviewable task.
- Do not create/push `v0.3.0` or its image without a separate explicit release request after release-candidate evidence.

## Planned File Map

- `internal/managed/input.go`: final-user extraction and ignored-category warnings.
- `internal/managed/normalize.go`: strict Chinese VM-start recognition and normalized text.
- `internal/managed/tool.go`: immutable Needle-native `pve_vm_start` schema and request builder.
- `internal/managed/validate.go`: model-result execution safety gate.
- `internal/managed/service.go`: model -> validate -> upstream -> final completion orchestration.
- `internal/managed/response.go`: final OpenAI response and managed `x_needle` shape.
- `internal/managed/streaming.go`: post-execution content SSE.
- `internal/infracontrol/client.go`: narrow `StartVM` no-retry client.
- `internal/config/config.go`: confidence/message/upstream settings.
- `internal/handler/chat.go`: managed request handler.
- `internal/svc/context.go`: managed service composition.
- Delete active generic flow: `internal/openai/schema.go`, `replay.go`, generic service/response/streaming and their tests where no longer shared.
- `README.md`, `CHANGELOG.md`, `.env.example`, `docker-compose.yml`, `needle-controller.api`: v0.3.0 contract.

---

### Task 1: Configuration for Managed Execution

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `etc/needle-controller.yaml`
- Modify: `.env.example`

**Interfaces:**
- Extends `config.NativeConfig` with `MinConfidence float64` and `MaxMessageLength int`.
- Adds `config.InfraControlConfig { BaseURL string; APIToken string; Timeout time.Duration }`.
- `config.Config` gains `InfraControl InfraControlConfig`.

- [ ] **Step 1: Write failing configuration tests**

Test exact environment names, defaults `0.6`, `512`, and `30s`; required base URL/token; HTTP(S)-origin validation; trailing-slash normalization; rejected userinfo/path/query/fragment; finite confidence `[0,1]`; positive message length/timeout; no fallback to `INFRA_CONTROL_BASE_URL`; errors never contain tokens.

- [ ] **Step 2: Run focused tests in Docker and verify RED**

```text
CGO_ENABLED=0 go test ./internal/config -count=1
```

Expected: FAIL because managed/upstream fields do not exist.

- [ ] **Step 3: Implement minimal config parsing and validation**

Use `url.Parse`, `math.IsNaN/IsInf`, strict integer/duration parsing, and trim only trailing root slashes. Do not perform an upstream network probe during startup.

- [ ] **Step 4: Update YAML and environment example**

Add only:

```text
NEEDLE_MIN_CONFIDENCE=0.6
NEEDLE_MAX_MESSAGE_LENGTH=512
INFRA_CONTROL_API_BASE_URL=http://infrastructure-control.example:8080
INFRA_CONTROL_API_TOKEN=replace-with-upstream-token
INFRA_CONTROL_TIMEOUT=30s
```

Remove `NEEDLE_MAX_REPLAY_STEPS` because managed mode executes exactly one turn.

- [ ] **Step 5: Run focused tests**

```text
CGO_ENABLED=0 go test ./internal/config -count=1
```

- [ ] **Step 6: Commit**

```bash
git add internal/config etc/needle-controller.yaml .env.example
git commit -m "feat: configure managed VM execution"
```

### Task 2: Agent Final-User Extraction

**Files:**
- Create: `internal/managed/input.go`
- Create: `internal/managed/input_test.go`
- Modify: `internal/openai/request.go` only if raw fields need distinguishing presence.

**Interfaces:**
- Produces `managed.Input { Original string; Warnings []string }`.
- Produces `func ExtractInput(req openai.ChatCompletionRequest, maxRunes int) (Input, *apierror.Error)`.

- [ ] **Step 1: Write failing accepted Agent-request tests**

Use complete request fixtures containing earlier system/developer/user/assistant/tool/function messages, client tools, tool choice, response format, sampling parameters, and unknown top-level fields. Assert only final user text is returned and warnings contain categories—not contents.

- [ ] **Step 2: Write failing final-message rejection tests**

Cover empty messages, final non-user role, empty/whitespace/null/object/array/content-parts/image/audio final content, and overlong Unicode text. Assert `user_message_required`, parameter `messages`, and no backward fallback to an earlier valid user command.

- [ ] **Step 3: Run focused tests and verify RED**

```text
CGO_ENABLED=0 go test ./internal/managed -run TestExtractInput -count=1
```

- [ ] **Step 4: Implement final-only extraction**

Index the final message directly, require role and JSON string, trim it, count runes, and derive stable warnings from field presence/count. Never parse or validate client tool schemas.

- [ ] **Step 5: Run tests and commit**

```bash
CGO_ENABLED=0 go test ./internal/managed -count=1
git add internal/managed/input* internal/openai/request.go
git commit -m "feat: extract only the final Agent user message"
```

### Task 3: Strict Chinese VM Start Normalization

**Files:**
- Create: `internal/managed/normalize.go`
- Create: `internal/managed/normalize_test.go`

**Interfaces:**
- Produces `managed.Command { Original string; Normalized string; VMID int64 }`.
- Produces `func NormalizeVMStart(input Input) (Command, *apierror.Error)`.

- [ ] **Step 1: Write failing acceptance table**

Literal accepted phrases:

```text
开启 3052 这个 VM
启动虚拟机 3052
把 VM 3052 开起来
给 3052 号虚拟机开机
打开 VM 3052
```

Assert exact `Normalized == "Start VM 3052"` and VM ID.

- [ ] **Step 2: Write failing rejection table**

Cover every negative/conflicting phrase in the spec, multiple/no/zero/negative/overflow IDs, English-only input, missing VM marker, and `VMware` boundary. Assert OpenAI 400 `unsupported_managed_command`.

- [ ] **Step 3: Run tests and verify RED**

```text
CGO_ENABLED=0 go test ./internal/managed -run TestNormalize -count=1
```

- [ ] **Step 4: Implement the smallest strict parser**

Use explicit phrase lists, Unicode Han detection, ASCII digit regex, case-insensitive standalone VM regex, and `strconv.ParseInt`. Do not add general NLP or translation.

- [ ] **Step 5: Run tests and commit**

```bash
git add internal/managed/normalize*
git commit -m "feat: normalize managed Chinese VM starts"
```

### Task 4: Fixed Native Tool and Mandatory Needle Request

**Files:**
- Create: `internal/managed/tool.go`
- Create: `internal/managed/tool_test.go`

**Interfaces:**
- Produces `func NativeRequest(command Command, cfg config.NativeConfig) native.Request`.
- Produces one immutable serialized `pve_vm_start` tool schema.

- [ ] **Step 1: Write failing fixed-schema/request tests**

Assert exact tool name, description, object schema, integer `vmid`, required list, `additionalProperties:false`, one normalized user turn, configured token budget/index path, and no client-derived data.

- [ ] **Step 2: Run test and verify RED**

```text
CGO_ENABLED=0 go test ./internal/managed -run TestNativeRequest -count=1
```

- [ ] **Step 3: Implement fixed request builder**

Use a version-controlled Go literal or `go:embed` JSON; validate once in a test, not dynamically from client input. `ToolNames` contains exactly `pve_vm_start`.

- [ ] **Step 4: Run tests and commit**

```bash
git add internal/managed/tool*
git commit -m "feat: declare the managed Needle VM tool"
```

### Task 5: Needle Result Safety Gate

**Files:**
- Create: `internal/managed/validate.go`
- Create: `internal/managed/validate_test.go`

**Interfaces:**
- Produces `managed.ValidatedCall { VMID int64; Confidence float64; Validation native.Validation }`.
- Produces `func ValidateCall(command Command, envelope native.Envelope, minConfidence float64) (ValidatedCall, *apierror.Error)`.

- [ ] **Step 1: Write valid-case test**

Assert exact single `pve_vm_start({"vmid":3052})`, confidence, empty grounding, and no negation passes.

- [ ] **Step 2: Write exhaustive failing table**

Cover `success:false`, wrong type/no/multiple/wrong calls, nil/extra/missing/string/fractional/boolean/zero/negative/overflow VM ID, original/model mismatch, missing/NaN/Inf/out-of-range/low confidence, non-empty grounding, and negation. Assert exact codes and zero usable call.

- [ ] **Step 3: Run test and verify RED**

```text
CGO_ENABLED=0 go test ./internal/managed -run TestValidateCall -count=1
```

- [ ] **Step 4: Implement fail-closed validation**

Use native RawMessage arguments, require exactly one key, parse integer without coercion, and map errors to the spec's 422 codes.

- [ ] **Step 5: Run tests and commit**

```bash
git add internal/managed/validate*
git commit -m "feat: gate managed Needle tool execution"
```

### Task 6: Narrow No-Retry Infrastructure Client

**Files:**
- Create: `internal/infracontrol/client.go`
- Create: `internal/infracontrol/client_test.go`

**Interfaces:**
- Produces `infracontrol.Service { StartVM(context.Context, requestID string, vmid int64) (Result, *apierror.Error) }`.
- Produces `Result { Status int; UpstreamRequestID string }`.

- [ ] **Step 1: Write failing request-contract test**

Using `httptest.Server`, assert one POST to `/api/v1/pve/vms/3052/start`, exact upstream Bearer token, Accept and request-ID headers, empty/no user-provided body, and 202 result.

- [ ] **Step 2: Write failure/no-retry table**

Cover redirects, 200, 400, 401/403, 429, 500, malformed/oversized bodies, connection failure, timeout, and caller cancellation. Count requests and assert exactly one or zero when canceled before send; never retry.

- [ ] **Step 3: Run test and verify RED**

```text
CGO_ENABLED=0 go test ./internal/infracontrol -count=1
```

- [ ] **Step 4: Implement fixed client**

Use a dedicated no-redirect `http.Client`, fixed path from validated int64, bounded response reads, and safe error messages that tell callers to query status before repeating when acceptance is uncertain.

- [ ] **Step 5: Run tests and commit**

```bash
git add internal/infracontrol
git commit -m "feat: execute one managed VM start request"
```

### Task 7: Managed OpenAI Final Response and SSE

**Files:**
- Create: `internal/managed/response.go`
- Create: `internal/managed/response_test.go`
- Create: `internal/managed/streaming.go`
- Create: `internal/managed/streaming_test.go`

**Interfaces:**
- Produces final `openai.ChatCompletionResponse` or a managed equivalent with text content and extended managed `x_needle`.
- Produces `StreamEvents` with role/content/stop/optional usage/DONE and no tool-call deltas.

- [ ] **Step 1: Write failing non-streaming response test**

Assert exact Chinese content, `finish_reason:stop`, omitted tool calls, zero estimated usage, tool/arguments/confidence/validation/executed/upstream status, warnings, and stable response identity.

- [ ] **Step 2: Write failing SSE tests**

Assert role, one complete UTF-8 Chinese content delta, terminal stop and metadata, optional usage, DONE, one shared identity, and absence of `tool_calls` in every event.

- [ ] **Step 3: Run tests and verify RED**

```text
CGO_ENABLED=0 go test ./internal/managed -run 'TestResponse|TestStream' -count=1
```

- [ ] **Step 4: Implement response and stream synthesis**

Reuse OpenAI model/usage/ID helpers only where they remain semantically correct. Do not reuse generic tool-call mapping. Ensure empty warnings serialize as `[]`.

- [ ] **Step 5: Run tests and commit**

```bash
git add internal/managed/response* internal/managed/streaming*
git commit -m "feat: synthesize managed OpenAI results"
```

### Task 8: Managed Service Orchestration

**Files:**
- Create: `internal/managed/service.go`
- Create: `internal/managed/service_test.go`

**Interfaces:**
- Consumes `NativeCompleter.Submit`, `infracontrol.Service.StartVM`, configuration, extraction/normalization/validation/response builders.
- Produces `managed.Service.Complete(ctx, requestID, openai.ChatCompletionRequest) (Response, *apierror.Error)`.

- [ ] **Step 1: Write failing ordered happy-path test**

Recording fakes assert exact order `Needle -> Infrastructure`, normalized text reaches Needle, original text does not, validated ID reaches upstream, exactly one call occurs, and final response contains no tool calls.

- [ ] **Step 2: Write failing short-circuit table**

For input, normalization, model not ready, queue full, native error, every safety-gate error, and upstream failure, assert expected OpenAI code and exact call counts. Every pre-upstream failure must make zero upstream calls.

- [ ] **Step 3: Write cancellation/no-retry tests**

Cancellation before native queue prevents upstream. Cancellation after upstream send does not trigger retry. One service invocation can call StartVM at most once.

- [ ] **Step 4: Run tests and verify RED**

```text
CGO_ENABLED=0 go test ./internal/managed -run TestService -count=1
```

- [ ] **Step 5: Implement orchestration without alternate paths**

Flow is exactly extract -> normalize -> native submit -> validate -> upstream start -> response. Append ignored-category warnings. There is no direct rule-to-upstream branch.

- [ ] **Step 6: Run package/race tests and commit**

```bash
CGO_ENABLED=0 go test ./internal/managed -count=1
git add internal/managed/service*
git commit -m "feat: orchestrate managed Needle VM starts"
```

### Task 9: HTTP Handler and Production Wiring

**Files:**
- Modify: `internal/handler/chat.go`
- Modify: `internal/handler/openai_test.go`
- Modify: `internal/svc/context.go`
- Modify: `internal/svc/context_test.go`
- Modify: `needle-controller.api`
- Modify: `cmd/needle-controller/main.go` only if shutdown dependencies change.

**Interfaces:**
- `ChatCompletions` calls managed service with request ID.
- `ServiceContext` owns native Dispatcher, Infrastructure client, and managed service.
- Routes remain unchanged.

- [ ] **Step 1: Write failing Agent-style HTTP test**

Send full history, client tools, tool choice, and final Chinese user text. Assert 200 final content, no tool calls, managed warnings, and one fake upstream call for only the final VM ID.

- [ ] **Step 2: Write failing final-message and error tests**

Assert final assistant/tool/non-text user errors, authentication/model/not-ready/queue/upstream mappings, request ID propagation, and no old `/api/v1/chat` route.

- [ ] **Step 3: Write failing managed SSE HTTP test**

Assert post-execution role/content/stop/metadata/usage/DONE and no tool-call events.

- [ ] **Step 4: Run focused tests and verify RED**

```text
CGO_ENABLED=0 go test ./internal/handler ./internal/svc ./internal/e2e -count=1
```

- [ ] **Step 5: Implement handler/wiring migration**

Keep request JSON permissive. Handler passes request ID, ordinary/SSE behavior, and cancellation to managed service. `/readyz` remains model state plus startup-config validity; no downstream active probe.

- [ ] **Step 6: Update `.api` contract and run full tests**

```text
CGO_ENABLED=0 go test ./... -count=1
CGO_ENABLED=0 go vet ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/handler internal/svc cmd needle-controller.api
git commit -m "feat: expose managed VM start through OpenAI"
```

### Task 10: Delete Generic Dynamic Tool Flow

**Files:**
- Delete: unused generic `internal/openai/schema.go`, `replay.go`, generic service/response/streaming files and tests.
- Retain or relocate: shared request/error/model/ID/usage primitives actually used by managed mode.
- Rewrite: `internal/e2e/` tests for managed behavior.

**Interfaces:**
- Removes dynamic tools from active production behavior.
- Preserves OpenAI wire compatibility for Agent-added fields by ignoring them.

- [ ] **Step 1: Add black-box ignored-tools test before deletion**

Provide a malformed client tool schema that v0.2.0 would reject. Assert v0.3.0 ignores it, invokes only fixed `pve_vm_start`, and executes the final user command. This catches accidental reuse of dynamic schema validation.

- [ ] **Step 2: Run test and verify RED before migration is complete**

```text
CGO_ENABLED=0 go test ./internal/e2e -run TestMalformedClientToolsAreIgnored -count=1
```

- [ ] **Step 3: Delete unused generic code**

Use compiler/search results to remove only dead generic paths; do not keep dual-mode factories or config flags.

- [ ] **Step 4: Run full tests and source checks**

```text
CGO_ENABLED=0 go test ./... -count=1
CGO_ENABLED=0 go vet ./...
```

Check that client tools are referenced only as ignored input, Infrastructure calls exist only in the narrow client, and no direct normalization-to-upstream call bypasses Needle.

- [ ] **Step 5: Commit**

```bash
git add -A internal/openai internal/e2e
git commit -m "refactor: remove dynamic client tool execution flow"
```

### Task 11: Container and Deployment Migration

**Files:**
- Modify: `docker-compose.yml`
- Modify: `Dockerfile` only if file inclusion changes.
- Modify: `.dockerignore`
- Modify: `Makefile`

**Interfaces:**
- Produces `needle-controller:0.3.0` with embedded Needle Engine and managed upstream configuration.

- [ ] **Step 1: Add container-config behavior test or validation fixture**

Verify Compose requires exactly the new upstream variables and confidence/message settings while retaining native settings. No old dynamic-mode/replay variable remains.

- [ ] **Step 2: Update Compose and Makefile**

Use image `0.3.0`, inject exact environment names, retain Linux AMD64/nonroot/read-only/tmpfs/cap-drop/healthcheck. Dockerfile still bundles only binary, native library, config, and licenses.

- [ ] **Step 3: Build image and run controlled mock-upstream smoke**

Use real embedded Needle plus a mock Infrastructure server. Send an Agent-style Chinese request and assert one POST, final Chinese assistant text, no returned tool calls, SSE behavior, and rejected unsafe inputs make zero POSTs.

- [ ] **Step 4: Run offline/model-only boundary check**

Because managed success requires an upstream network, `NetworkMode:none` should still start and become model-ready, but execution should fail once with upstream error and never retry. Health/readiness must remain correct.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile .dockerignore docker-compose.yml Makefile
git commit -m "build: package managed VM execution service"
```

### Task 12: Documentation and Breaking Migration

**Files:**
- Rewrite: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `docs/api.md`

**Interfaces:**
- Documents v0.3.0 as managed auto-execution and directs general dynamic-tool users to v0.2.0.

- [ ] **Step 1: Rewrite usage examples**

Show minimal and Agent-style requests without requiring tools, final Chinese response, SSE, exact credentials, confidence gate, no-retry semantics, and the mandatory Needle selection path.

- [ ] **Step 2: Add v0.3.0 changelog**

List breaking ignored client tools/history behavior, restored Infrastructure credentials/execution, one managed tool only, and version fallback guidance.

- [ ] **Step 3: Update API catalogue context**

Mark only PVE VM start as wired in v0.3.0; other documented APIs remain future managed tools. Do not imply the model can execute them.

- [ ] **Step 4: Validate documented examples against a real image with mock upstream**

Run every documented HTTP payload and assert status/shape/one upstream call. Documentation testing must exercise behavior, not grep prose.

- [ ] **Step 5: Commit**

```bash
git add README.md CHANGELOG.md docs/api.md
git commit -m "docs: describe managed OpenAI VM execution"
```

### Task 13: Real Authorized End-to-End and Release Candidate

**Files:**
- Modify only when a verified failure requires a regression test/fix.

**Interfaces:**
- Produces evidence for real embedded Needle selection and real Infrastructure execution; does not publish automatically.

- [ ] **Step 1: Run fresh complete verification**

In Docker:

```text
python -m unittest -v build/fetch-needle_test.py
CGO_ENABLED=0 go test ./... -count=1
CGO_ENABLED=0 go vet ./...
go test -race ./... -count=1
go vet ./...
go test -tags=needle_native ./internal/native -count=1
```

- [ ] **Step 2: Build `needle-controller:0.3.0-rc` without cache**

Require pinned artifact double-hash validation, native test during build, CGO dependency inspection, nonroot/read-only startup, and image inspection.

- [ ] **Step 3: Run mock-upstream safety matrix**

Verify accepted Agent history/tools are ignored, final-user only, valid start one POST, all input/model/safety/upstream failure branches, no retries, ordinary response, and SSE.

- [ ] **Step 4: Run real authorized Chinese test**

After explicit authorization and safe temporary credential injection:

1. Query VM 3052 status.
2. Send OpenAI request with final user `打开 VM 3052` and distracting history/tools.
3. Confirm embedded Needle call and safety data.
4. Confirm exactly one real upstream POST returning 202.
5. Query VM status after a short wait.
6. Report stopped/running state accurately.
7. Delete temporary credentials/containers; do not stop the VM unless separately authorized.

- [ ] **Step 5: Request independent review or record tool unavailability**

Review from `v0.2.0` to feature HEAD for model bypass, stale-history execution, client-tool influence, duplicate execution, retries, token leakage, cancellation, SSE semantics, and native races. Fix Critical/Important findings with RED/GREEN tests.

- [ ] **Step 6: Run repository safety checks**

Confirm clean diff, no secrets/binaries, no old endpoint, no general dynamic execution path, and exact environment names. Verify Git/Image `v0.2.0` remains untouched.

- [ ] **Step 7: Build final release candidate after all fixes**

Repeat all tests and a no-cache image build; run one final real-model/mock-upstream completion. Record commit and digest.

- [ ] **Step 8: Finish branch**

Use finishing-a-development-branch. Merge/tag/push `v0.3.0` and Registry image only after the user explicitly requests formal release based on the reported RC evidence.
