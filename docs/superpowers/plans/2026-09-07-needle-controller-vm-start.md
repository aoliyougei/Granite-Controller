# Needle Controller VM Start Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Dockerized go-zero REST controller that turns an authenticated natural-language VM-start instruction into one strictly validated `pve_vm_start` tool call and one Infrastructure Control start request.

**Architecture:** A thin HTTP layer delegates to an application service whose dependencies are a Needle OpenAI client, a fail-closed tool-call validator, and a narrow Infrastructure Control client. The service is stateless; models may select only the compiled-in VM-start tool, and only validated commands reach the mutation client.

**Tech Stack:** Go 1.24, go-zero REST, standard-library `net/http` and `httptest`, Docker multi-stage builds, Docker Compose.

**Spec:** `docs/superpowers/specs/2026-09-07-needle-controller-vm-start-design.md`

## Global Constraints

- The new project lives at `needle-controller/`; the repository root is Git-managed, while `/source/` remains ignored reference material.
- Runtime service framework is go-zero REST; the first release exposes `GET /healthz`, `GET /readyz`, and authenticated `POST /api/v1/chat` only.
- The controller is stateless and supports one model-visible tool: `pve_vm_start`.
- A start request executes directly after all validation gates pass; there is no VM allowlist and no confirmation flow in phase one.
- `NEEDLE_MIN_CONFIDENCE` defaults to `0.6`; missing or malformed safety metadata fails closed.
- Only Infrastructure Control HTTP 202 is success; the public response says the request was submitted, never that VM startup completed.
- No generic HTTP executor, redirects, mutation retries, model-provided URLs, methods, headers, credentials, or configuration overrides are permitted.
- `CONTROLLER_API_TOKEN` and `INFRA_CONTROL_API_TOKEN` are required environment secrets; `NEEDLE_API_KEY` is optional.
- All dependency resolution, generation, tests, race tests, vet, builds, and application execution occur through Docker API tooling, never in the local Kubernetes workspace.
- Use TDD for every behavior: observe a focused failing test before adding its implementation, then run the focused and package-wide tests in Docker.
- Commit after every task; never commit secrets or generated dependency/build artifacts.

## Planned File Map

- `needle-controller/go.mod`, `go.sum`: pinned module and dependency graph.
- `needle-controller/needle-controller.api`: go-zero route/type contract documenting the public API.
- `needle-controller/cmd/needle-controller/main.go`: config loading, validation, dependency construction, routes, and graceful server startup.
- `needle-controller/etc/needle-controller.yaml`: non-secret defaults.
- `needle-controller/internal/config/config.go`: typed config, environment overrides, URL/duration/range validation.
- `needle-controller/internal/apierror/error.go`: stable application error codes, HTTP status mapping, and safe public messages.
- `needle-controller/internal/requestid/requestid.go`: safe inbound request-ID validation and generation.
- `needle-controller/internal/types/types.go`: strict HTTP request/response DTOs.
- `needle-controller/internal/needle/models.go`: exact Needle request/response wire types.
- `needle-controller/internal/needle/client.go`: bounded, no-redirect Needle HTTP client and readiness probe.
- `needle-controller/internal/needle/validator.go`: fail-closed tool-call validation.
- `needle-controller/internal/infracontrol/models.go`: bounded downstream error wire types.
- `needle-controller/internal/infracontrol/client.go`: narrow `StartVM` client and readiness probe.
- `needle-controller/internal/logic/chat.go`: orchestration interfaces and chat use case.
- `needle-controller/internal/middleware/auth.go`: constant-time Bearer authentication.
- `needle-controller/internal/handler/response.go`: strict JSON decode and uniform JSON output.
- `needle-controller/internal/handler/health.go`, `ready.go`, `chat.go`: HTTP adapters.
- `needle-controller/internal/svc/context.go`: production dependency container.
- `needle-controller/**/*_test.go`: unit and HTTP tests beside packages.
- `needle-controller/Dockerfile`, `.dockerignore`, `docker-compose.yml`, `.env.example`, `Makefile`: reproducible build/deployment surface.
- `needle-controller/README.md`: configuration, usage, security, and 202 semantics.

---

### Task 1: Repository Baseline and Configuration Contract

**Files:**
- Create: `.gitignore`
- Create: `needle-controller/go.mod`
- Create: `needle-controller/needle-controller.api`
- Create: `needle-controller/etc/needle-controller.yaml`
- Create: `needle-controller/internal/config/config.go`
- Create: `needle-controller/internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Config`, `config.NeedleConfig`, `config.InfraControlConfig`.
- Produces: `func Load(path string) (Config, error)` and `func (c Config) Validate() error`.
- Later tasks consume effective normalized URLs, durations, confidence, model ID, token/message limits, and secret values from `config.Config`.

- [ ] **Step 1: Create the module manifest and failing config tests**

Use this initial module manifest:

```go
module needle-controller

go 1.24

require github.com/zeromicro/go-zero v1.9.2
```

Create table-driven tests that write temporary YAML files and use `t.Setenv` to verify:

```go
func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
    t.Setenv("CONTROLLER_API_TOKEN", "controller-secret")
    t.Setenv("INFRA_CONTROL_API_TOKEN", "infra-secret")
    t.Setenv("NEEDLE_BASE_URL", "http://needle.example:8000/")
    t.Setenv("NEEDLE_MIN_CONFIDENCE", "0.75")
    t.Setenv("CONTROLLER_MAX_MESSAGE_LENGTH", "300")

    cfg, err := Load(testConfigPath(t))
    require.NoError(t, err)
    assert.Equal(t, "http://needle.example:8000", cfg.Needle.BaseURL)
    assert.Equal(t, 0.75, cfg.Needle.MinConfidence)
    assert.Equal(t, 300, cfg.Needle.MaxMessageLength)
}
```

Also test missing controller/downstream tokens, malformed URLs, URL userinfo/query/fragment rejection, invalid confidence outside `[0,1]`, non-positive duration/token/message limits, and an empty optional `NEEDLE_API_KEY`.

- [ ] **Step 2: Run focused tests in Docker and verify RED**

Create a task-owned Go container from `golang:1.24-alpine`, upload the project directory, and run:

```text
go mod download
go test ./internal/config -run 'TestLoad|TestValidate' -count=1
```

Expected: FAIL because `config.Load` and configuration types do not exist.

- [ ] **Step 3: Implement typed configuration and environment overrides**

Embed go-zero `rest.RestConf` and define:

```go
type Config struct {
    rest.RestConf
    ControllerAPIToken string
    Needle             NeedleConfig
    InfraControl       InfraControlConfig
}

type NeedleConfig struct {
    BaseURL         string
    APIKey          string
    Model           string
    Timeout         time.Duration
    MinConfidence   float64
    MaxTokens       int
    MaxMessageLength int
}

type InfraControlConfig struct {
    BaseURL string
    APIToken string
    Timeout time.Duration
}
```

`Load` must call go-zero `conf.MustLoad` only from `main`; the testable loader should return errors. Apply exact environment names from the spec, trim one or more trailing slashes, parse durations with `time.ParseDuration`, parse numeric overrides strictly, reject base URLs unless scheme is `http` or `https`, host is non-empty, and userinfo/query/fragment are absent. Never include token values in errors.

- [ ] **Step 4: Add the API contract and default YAML**

Define routes and DTOs in `needle-controller.api`:

```text
@server(group: public)
service needle-controller {
  @handler Health
  get /healthz returns (HealthResponse)
  @handler Ready
  get /readyz returns (ReadyResponse)
}

@server(group: protected)
service needle-controller {
  @handler Chat
  post /api/v1/chat (ChatRequest) returns (ChatResponse)
}
```

Store only non-secret defaults in YAML. Do not put token keys with sample secret values into it.

- [ ] **Step 5: Run config tests and module tests in Docker**

Run:

```text
go test ./internal/config -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the baseline**

```bash
git add .gitignore needle-controller/go.mod needle-controller/go.sum \
  needle-controller/needle-controller.api needle-controller/etc \
  needle-controller/internal/config
git commit -m "feat: establish needle controller configuration"
```

### Task 2: Stable Errors, Request IDs, and Strict HTTP Primitives

**Files:**
- Create: `needle-controller/internal/apierror/error.go`
- Create: `needle-controller/internal/apierror/error_test.go`
- Create: `needle-controller/internal/requestid/requestid.go`
- Create: `needle-controller/internal/requestid/requestid_test.go`
- Create: `needle-controller/internal/types/types.go`
- Create: `needle-controller/internal/handler/response.go`
- Create: `needle-controller/internal/handler/response_test.go`

**Interfaces:**
- Produces: `type apierror.Error struct { Code string; Message string; HTTPStatus int; Cause error }` implementing `error` and `Unwrap`.
- Produces: constructors/constants for every error code in spec section 9.
- Produces: `requestid.FromRequest(*http.Request) string` and `requestid.Valid(string) bool`.
- Produces: strict `handler.DecodeJSON(w, r, dst, maxBytes) *apierror.Error` and JSON writers.
- Produces public DTOs `types.ChatRequest`, `ChatResponse`, `ErrorResponse`, `HealthResponse`, and `ReadyResponse`.

- [ ] **Step 1: Write failing error and request-ID tests**

Verify stable code/status/message values and that wrapped causes are not serialized. Verify request IDs accept only 1–128 visible ASCII characters from `[A-Za-z0-9._:-]`; unsafe, empty, or oversized IDs are replaced with a generated non-empty ID.

- [ ] **Step 2: Write failing strict-JSON tests**

Test valid one-object input and rejection of unknown fields, trailing JSON values, empty body, malformed JSON, and bodies above 8 KiB. Assert errors use `REQUEST_INVALID` and never echo the request body.

- [ ] **Step 3: Run focused tests in Docker and verify RED**

```text
go test ./internal/apierror ./internal/requestid ./internal/handler -count=1
```

Expected: FAIL because packages/functions are missing.

- [ ] **Step 4: Implement errors, IDs, DTOs, and response helpers**

Use `crypto/rand` for 16 random bytes and hex encoding for generated IDs, with a timestamp-free fallback only if randomness fails. `DecodeJSON` must use `http.MaxBytesReader`, `json.Decoder.DisallowUnknownFields`, and a second decode requiring `io.EOF`. JSON writers always set `Content-Type: application/json` and `X-Request-ID`.

- [ ] **Step 5: Run focused and full tests in Docker**

```text
go test ./internal/apierror ./internal/requestid ./internal/handler -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit HTTP primitives**

```bash
git add needle-controller/internal/apierror needle-controller/internal/requestid \
  needle-controller/internal/types needle-controller/internal/handler/response.go \
  needle-controller/internal/handler/response_test.go
git commit -m "feat: add safe controller HTTP primitives"
```

### Task 3: Needle OpenAI Client

**Files:**
- Create: `needle-controller/internal/needle/models.go`
- Create: `needle-controller/internal/needle/client.go`
- Create: `needle-controller/internal/needle/client_test.go`

**Interfaces:**
- Produces: `type needle.Completion struct { ToolCalls []ToolCall; Safety SafetyMetadata }`.
- Produces: `type needle.Service interface { Complete(ctx context.Context, message string) (Completion, error); Ready(ctx context.Context) error }`.
- Produces: `func NewClient(cfg config.NeedleConfig) *Client` implementing `Service`.
- Errors are returned as typed `*apierror.Error` with `NEEDLE_BUSY` or `NEEDLE_UNAVAILABLE`.

- [ ] **Step 1: Write failing request-shape and parsing tests**

Use `httptest.Server` to capture a request and assert:

```go
assert.Equal(t, http.MethodPost, r.Method)
assert.Equal(t, "/v1/chat/completions", r.URL.Path)
assert.Equal(t, "Bearer needle-secret", r.Header.Get("Authorization"))
assert.Len(t, payload.Tools, 1)
assert.Equal(t, "pve_vm_start", payload.Tools[0].Function.Name)
assert.Equal(t, false, payload.Tools[0].Function.Parameters.AdditionalProperties)
```

Return one realistic completion and verify parsing of call ID/name/raw arguments, confidence, empty `ungrounded`, and false `negation`.

- [ ] **Step 2: Write failing resilience tests**

Cover optional absent API key, 429 mapping, other non-2xx mapping, malformed JSON, response larger than 1 MiB, request timeout, rejected redirect, and `/health` readiness requiring HTTP 200 plus `{"status":"ok"}`.

- [ ] **Step 3: Run Needle tests in Docker and verify RED**

```text
go test ./internal/needle -count=1
```

Expected: FAIL because the client does not exist.

- [ ] **Step 4: Implement exact wire models and fixed tool schema**

The request body must be equivalent to:

```json
{
  "model": "needle-2",
  "messages": [{"role":"user","content":"开启 3052 这个 VM"}],
  "tools": [{
    "type":"function",
    "function": {
      "name":"pve_vm_start",
      "description":"Start or power on a Proxmox VE virtual machine. 用于开启、启动或开机一个 PVE 虚拟机。",
      "parameters": {
        "type":"object",
        "properties":{"vmid":{"type":"integer","description":"Proxmox VE 虚拟机的数字 ID，例如 3052"}},
        "required":["vmid"],
        "additionalProperties":false
      }
    }
  }],
  "max_tokens":256
}
```

Represent safety fields with pointer/raw types so missing values remain distinguishable from false/zero.

- [ ] **Step 5: Implement bounded no-redirect HTTP behavior**

Use a dedicated `http.Client` with configured timeout and:

```go
CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
    return http.ErrUseLastResponse
}
```

Read responses through `io.LimitReader(limit+1)`, reject overflow, close bodies, and never include raw response bodies or credentials in public errors.

- [ ] **Step 6: Run Needle and full tests in Docker**

```text
go test ./internal/needle -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the Needle client**

```bash
git add needle-controller/internal/needle
git commit -m "feat: add bounded Needle OpenAI client"
```

### Task 4: Fail-Closed VM Start Tool Validator

**Files:**
- Create: `needle-controller/internal/needle/validator.go`
- Create: `needle-controller/internal/needle/validator_test.go`

**Interfaces:**
- Produces: `type needle.StartVMCommand struct { VMID int64; Confidence float64; ToolCallID string }`.
- Produces: `func ValidateStartVM(c Completion, minConfidence float64) (StartVMCommand, error)`.
- Returns stable `*apierror.Error` values for every validator rejection.

- [ ] **Step 1: Write the valid-case and structural failing tests**

Create table tests for no call, multiple calls, wrong function, malformed argument JSON, missing VM ID, extra fields, and valid `{"vmid":3052}`. Assert every rejection returns the expected code and no command.

- [ ] **Step 2: Add numeric edge-case failing tests**

Test string, boolean, fractional, zero, negative, exponential fractional, larger-than-int64, and duplicate `vmid` keys. Decode arguments with a token-level strict object parser so duplicate keys are rejected rather than silently overwritten.

- [ ] **Step 3: Add safety-metadata failing tests**

Test missing/non-finite/out-of-range/low confidence, missing or non-array `ungrounded`, non-empty `ungrounded`, and missing/non-boolean/true negation. Include a regression fixture representing `不要开启 3052` with `negation=true` and assert `NEEDLE_NEGATION_DETECTED`.

- [ ] **Step 4: Run validator tests in Docker and verify RED**

```text
go test ./internal/needle -run TestValidateStartVM -count=1
```

Expected: FAIL because validator behavior is absent.

- [ ] **Step 5: Implement minimal strict validation**

Use `json.Decoder.UseNumber`, manually iterate the top-level object tokens, reject duplicate/unknown keys, require exactly one `vmid`, parse with `strconv.ParseInt`, and require EOF. Validate safety metadata before returning `StartVMCommand`; do not coerce any values.

- [ ] **Step 6: Run validator and full tests in Docker**

```text
go test ./internal/needle -run TestValidateStartVM -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the validator**

```bash
git add needle-controller/internal/needle/validator.go \
  needle-controller/internal/needle/validator_test.go
git commit -m "feat: validate Needle VM start calls"
```

### Task 5: Narrow Infrastructure Control Client

**Files:**
- Create: `needle-controller/internal/infracontrol/models.go`
- Create: `needle-controller/internal/infracontrol/client.go`
- Create: `needle-controller/internal/infracontrol/client_test.go`

**Interfaces:**
- Produces: `type infracontrol.Service interface { StartVM(ctx context.Context, requestID string, vmid int64) (StartResult, error); Ready(ctx context.Context) error }`.
- Produces: `type StartResult struct { UpstreamStatus int; UpstreamRequestID string }`.
- Produces: `func NewClient(cfg config.InfraControlConfig) *Client` implementing `Service`.

- [ ] **Step 1: Write failing success-contract tests**

With `httptest.Server`, assert exactly one request uses POST, the fixed `/api/v1/pve/vms/3052/start` path, expected Infrastructure Bearer token, `Accept: application/json`, and propagated `X-Request-ID`. Return 202 and assert accepted result.

- [ ] **Step 2: Write failing safety and error tests**

Assert HTTP 200 is rejected, 401/403 map to `INFRA_CONTROL_AUTH_FAILED`, other statuses map to `INFRA_CONTROL_REQUEST_FAILED`, documented error fields are bounded/sanitized, malformed and oversized bodies do not crash, redirects are rejected, timeouts map to `INFRA_CONTROL_TIMEOUT`, and every case makes exactly one request with no retry.

- [ ] **Step 3: Run client tests in Docker and verify RED**

```text
go test ./internal/infracontrol -count=1
```

Expected: FAIL because the client does not exist.

- [ ] **Step 4: Implement `StartVM` and readiness**

Construct the path only from the validated positive integer using `strconv.FormatInt`. Use a no-redirect, timeout-bound client and bounded response reads. `Ready` calls public `/readyz`, requires a 2xx response, and sends no Infrastructure authorization token unless the documented endpoint later requires it; phase one follows `docs/api.md` where `/readyz` is public.

- [ ] **Step 5: Run client and full tests in Docker**

```text
go test ./internal/infracontrol -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the Infrastructure Control client**

```bash
git add needle-controller/internal/infracontrol
git commit -m "feat: add PVE VM start client"
```

### Task 6: Chat Orchestration

**Files:**
- Create: `needle-controller/internal/logic/chat.go`
- Create: `needle-controller/internal/logic/chat_test.go`

**Interfaces:**
- Consumes: `needle.Service`, `needle.ValidateStartVM`, and `infracontrol.Service`.
- Produces: `type logic.ChatService interface { Execute(ctx context.Context, requestID, message string) (types.ChatResponse, error) }`.
- Produces: `func NewChatService(needle needle.Service, infra infracontrol.Service, minConfidence float64, maxMessageLength int) ChatService`.

- [ ] **Step 1: Write failing orchestration tests**

Use fakes recording call order. Test a successful path returns:

```go
types.ChatResponse{
    Status: "accepted",
    Message: "VM 3052 的启动请求已提交",
    Tool: "pve_vm_start",
    Arguments: types.StartVMArguments{VMID: 3052},
    Confidence: 0.92,
    UpstreamStatus: http.StatusAccepted,
    RequestID: "req-1",
}
```

Assert call order `needle -> infrastructure`, exactly one start call, and exact VM/request IDs.

- [ ] **Step 2: Write failing short-circuit tests**

Verify trimmed empty and overlong Unicode messages fail before Needle, Needle errors stop execution, every validator error prevents Infrastructure Control, and downstream failures are returned unchanged. Add a panic-free unexpected-error mapping test at the HTTP boundary in Task 8, not by hiding typed dependency errors here.

- [ ] **Step 3: Run logic tests in Docker and verify RED**

```text
go test ./internal/logic -count=1
```

Expected: FAIL because orchestration is absent.

- [ ] **Step 4: Implement the minimal orchestration service**

Count message length with `utf8.RuneCountInString`, trim only for validation and send the trimmed value to Needle, call the validator, then call `StartVM` exactly once. Do not retry either mutation or model call in this layer.

- [ ] **Step 5: Run logic and full tests in Docker**

```text
go test ./internal/logic -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit orchestration**

```bash
git add needle-controller/internal/logic
git commit -m "feat: orchestrate validated VM start requests"
```

### Task 7: Bearer Authentication Middleware

**Files:**
- Create: `needle-controller/internal/middleware/auth.go`
- Create: `needle-controller/internal/middleware/auth_test.go`

**Interfaces:**
- Produces: `func NewBearerAuth(token string) func(http.Handler) http.Handler`.
- Authentication failure writes `types.ErrorResponse` with code `AUTH_UNAUTHORIZED`, status 401, and the effective request ID.

- [ ] **Step 1: Write failing middleware tests**

Test absent header, wrong scheme, empty token, wrong token, multiple authorization header values, malformed whitespace, and correct exact token. Verify protected handlers are never called on failure and error bodies are identical for missing/wrong credentials.

- [ ] **Step 2: Run middleware tests in Docker and verify RED**

```text
go test ./internal/middleware -count=1
```

Expected: FAIL because middleware is absent.

- [ ] **Step 3: Implement constant-time exact authentication**

Require one `Authorization` header matching `Bearer <token>` with no surrounding ambiguity. Compare SHA-256 digests using `subtle.ConstantTimeCompare` so differing token lengths do not short-circuit. Never log or return either token.

- [ ] **Step 4: Run middleware and full tests in Docker**

```text
go test ./internal/middleware -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit authentication**

```bash
git add needle-controller/internal/middleware
git commit -m "feat: protect controller actions with bearer auth"
```

### Task 8: HTTP Handlers, Readiness, and Service Wiring

**Files:**
- Create: `needle-controller/internal/svc/context.go`
- Create: `needle-controller/internal/handler/health.go`
- Create: `needle-controller/internal/handler/ready.go`
- Create: `needle-controller/internal/handler/chat.go`
- Create: `needle-controller/internal/handler/handler_test.go`
- Create: `needle-controller/cmd/needle-controller/main.go`

**Interfaces:**
- Consumes: all clients/services from Tasks 1–7.
- Produces: `svc.ServiceContext` holding config, chat service, and readiness dependencies.
- Produces: `handler.Register(server *rest.Server, ctx *svc.ServiceContext)` registering public and protected routes.
- Produces executable `cmd/needle-controller` accepting go-zero `-f` config path.

- [ ] **Step 1: Write failing endpoint tests**

Build an in-memory router with fakes and verify:

- `/healthz` returns public HTTP 200.
- `/readyz` returns 200 only when both fakes are ready and 503 otherwise.
- `/api/v1/chat` returns 401 without/with wrong token.
- Strict JSON rejects unknown fields and trailing values.
- Empty/overlong messages return 400.
- A successful fake chat returns 202.
- Typed errors map to their statuses/codes.
- Unexpected errors map to `INTERNAL_ERROR` without leaking the cause.
- Every response includes matching body/header request IDs where the body schema includes it.
- Captured logs and response bodies do not contain any of three test tokens.

- [ ] **Step 2: Run handler tests in Docker and verify RED**

```text
go test ./internal/handler -run 'TestHealth|TestReady|TestChat' -count=1
```

Expected: FAIL because endpoints and wiring are absent.

- [ ] **Step 3: Implement handlers and readiness checks**

Run dependency readiness checks concurrently with a short child context no longer than the lower configured dependency timeout. Return only `ready` or `unavailable` states. Chat must use strict decode, attach request ID before authentication/logic, and map errors through the stable error package.

- [ ] **Step 4: Implement production service context and main**

`main` parses `-f`, loads and validates config, constructs clients/services, installs go-zero structured logging, registers routes with authentication only on `/api/v1/chat`, and starts `rest.Server`. Startup errors must name configuration fields but never values of secrets.

- [ ] **Step 5: Run endpoint and full tests in Docker**

```text
go test ./internal/handler -count=1
go test ./cmd/needle-controller ./... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the REST service**

```bash
git add needle-controller/internal/svc needle-controller/internal/handler \
  needle-controller/cmd
git commit -m "feat: expose authenticated VM start API"
```

### Task 9: Container Packaging and Operator Documentation

**Files:**
- Create: `needle-controller/Dockerfile`
- Create: `needle-controller/.dockerignore`
- Create: `needle-controller/docker-compose.yml`
- Create: `needle-controller/.env.example`
- Create: `needle-controller/Makefile`
- Create: `needle-controller/README.md`

**Interfaces:**
- Produces image `needle-controller:0.1.0` listening on port 8080 as non-root.
- Produces Compose deployment configured entirely by environment/Secret values.

- [ ] **Step 1: Write a failing container smoke-check script as a Docker validation command**

Before writing the Dockerfile, attempt:

```text
docker image build equivalent for needle-controller:0.1.0
```

Expected: FAIL because `needle-controller/Dockerfile` does not exist. Record this RED result in task evidence rather than adding a permanent shell script.

- [ ] **Step 2: Implement the multi-stage Dockerfile and ignore rules**

Pin explicit image tags/digests available in the configured registry at implementation time. Builder runs `go mod download`, then:

```text
CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/needle-controller ./cmd/needle-controller
```

Runtime uses a distroless non-root image and copies the binary plus YAML. Do not add a Dockerfile `HEALTHCHECK` that relies on shell/curl in distroless; Compose health checking may use a sidecar/external probe or the binary may gain an explicit `healthcheck` subcommand only if tests specify it first.

- [ ] **Step 3: Add hardened Compose and environment example**

Compose includes `read_only`, `/tmp` tmpfs, `no-new-privileges`, all-capability drop, restart policy, port mapping, and environment interpolation. `.env.example` contains obvious non-secret placeholders and warnings, never working credentials.

- [ ] **Step 4: Add Make targets and README**

Document:

```text
make test     -> go test ./...
make race     -> go test -race ./...
make vet      -> go vet ./...
make build    -> go build ./cmd/needle-controller
```

README must include configuration, Docker/Compose launch, health/readiness, authenticated curl example, sample 202 response, no-confirmation/no-allowlist warning, confidence gates, and HTTP 202 semantics.

- [ ] **Step 5: Build and inspect the production image using Docker API**

Build `needle-controller:0.1.0`, inspect image/container config, start with mock dependency URLs and non-secret test tokens, and verify the configured user is non-root and `/healthz` returns 200. Use Docker API lifecycle calls and remove only task-owned containers after evidence is captured.

- [ ] **Step 6: Run source checks in a Go builder container**

```text
gofmt -w $(find . -name '*.go')
go test ./...
go test -race ./...
go vet ./...
```

Copy back only intentional formatted source if formatting changed it; do not copy module caches or binaries into the workspace.

- [ ] **Step 7: Commit packaging and docs**

```bash
git add needle-controller/Dockerfile needle-controller/.dockerignore \
  needle-controller/docker-compose.yml needle-controller/.env.example \
  needle-controller/Makefile needle-controller/README.md
git commit -m "docs: package and document needle controller"
```

### Task 10: End-to-End Safety and VM Start Acceptance

**Files:**
- Create: `needle-controller/internal/e2e/e2e_test.go`
- Modify if required by observed failures: files under `needle-controller/` only.

**Interfaces:**
- Consumes the complete HTTP server with mock Needle and Infrastructure Control servers.
- Produces acceptance evidence for the phase-one flow and safety invariants.

- [ ] **Step 1: Write the failing end-to-end happy-path test**

Start two `httptest.Server` dependencies and the controller router. Mock Needle returns:

```json
{
  "choices":[{"message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"pve_vm_start","arguments":"{\"vmid\":3052}"}}]}}],
  "x_needle":{"confidence":0.92,"validation":{"ungrounded":[],"negation":false}}
}
```

Assert authenticated `{"message":"开启 3052 这个 VM"}` yields controller 202 and exactly one downstream `POST /api/v1/pve/vms/3052/start`.

- [ ] **Step 2: Write failing end-to-end rejection tests**

For no call, low confidence, `ungrounded`, `negation=true`, malformed arguments, and wrong tool, assert the documented 422 code and zero Infrastructure Control requests. Add authentication failure asserting Needle also receives zero requests.

- [ ] **Step 3: Run end-to-end tests in Docker and verify RED where integration hooks are incomplete**

```text
go test ./internal/e2e -count=1
```

Expected: FAIL before final test-server composition is exposed, or PASS only if existing public constructors already satisfy all cases; if it passes immediately, preserve the tests and proceed without manufacturing a failure.

- [ ] **Step 4: Make the smallest integration adjustments needed**

Expose a testable router/server constructor without weakening production encapsulation. Do not add alternate execution paths or generic dependency injection visible through the public HTTP API.

- [ ] **Step 5: Run complete verification in Docker with fresh output**

```text
go test ./...
go test -race ./...
go vet ./...
```

Build the production image again, run it as non-root, and perform `/healthz` and `/readyz` smoke checks against controlled dependencies. If network access to the user's real Needle/Infrastructure services is unavailable from the Docker execution environment, report real-server integration as pending rather than claiming it passed; mock end-to-end coverage remains required.

- [ ] **Step 6: Inspect repository safety and diff**

Run non-executing local checks:

```bash
git status --short
git diff --check
git grep -nE '(controller-secret|infra-secret|sk-[A-Za-z0-9])' -- . ':!docs/superpowers/plans/*'
```

Expected: no real secrets, no whitespace errors, and only intended files changed.

- [ ] **Step 7: Commit acceptance tests and final fixes**

```bash
git add needle-controller
git commit -m "test: verify VM start controller end to end"
```

- [ ] **Step 8: Request final code review before declaring completion**

Apply the requesting-code-review workflow against the complete diff. Address only technically verified findings, rerun affected Docker checks, and then apply verification-before-completion before reporting the result.
