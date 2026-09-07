# Needle Controller VM Start — Design Specification

**Date:** 2026-09-07  
**Status:** Approved for implementation planning  
**Project:** `needle-controller`

## 1. Purpose

Build a stateless HTTP controller that accepts a short natural-language instruction such as `开启 3052 这个 VM`, asks the deployed `needle-openai` service to select a narrowly defined tool, validates the model output, and submits the corresponding VM start request to Infrastructure Control.

The controller will be implemented in Go using go-zero, packaged as a Docker image, and designed so it can later be deployed to Kubernetes. The first release supports only starting a Proxmox VE virtual machine.

## 2. Scope

### Included

- A new Go project in `needle-controller/`.
- go-zero REST service.
- `GET /healthz` liveness endpoint.
- `GET /readyz` dependency-readiness endpoint.
- Authenticated `POST /api/v1/chat` endpoint.
- Static Bearer-token authentication for controller clients.
- OpenAI Chat Completions client for `needle-openai`.
- A single model-visible tool: `pve_vm_start`.
- Strict validation of tool name, count, arguments, confidence, grounding, and negation.
- Fixed Infrastructure Control call: `POST /api/v1/pve/vms/{vmid}/start`.
- Unified JSON errors and request IDs.
- Unit and HTTP integration tests with mock HTTP servers.
- Multi-stage Docker build, Docker Compose example, configuration example, and usage documentation.

### Excluded

- VM status lookup, shutdown, stop, or reboot.
- PVE node, physical host, PBS, miIO power, workflow, or task operations.
- Multi-turn conversations or server-side session storage.
- Polling until a VM reaches its final running state.
- Generic or model-generated HTTP methods, URLs, headers, or credentials.
- VM allowlists.
- Interactive or two-phase confirmation.
- Web UI, Kubernetes manifests, databases, user/role authorization, and audit-log persistence.
- Executing multiple model-selected tools in one request.

## 3. External API

### 3.1 Liveness

```http
GET /healthz
```

This endpoint is public and checks only whether the controller process can serve HTTP. It performs no dependency calls.

Successful response:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{"status":"ok"}
```

### 3.2 Readiness

```http
GET /readyz
```

This endpoint is public. It checks that required configuration is loaded, Needle reports `status: "ok"` from `/health`, and Infrastructure Control `/readyz` is reachable and successful.

Successful response:

```http
HTTP/1.1 200 OK
```

```json
{
  "status": "ready",
  "dependencies": {
    "needle": "ready",
    "infrastructureControl": "ready"
  }
}
```

If either dependency is unavailable or not ready, it returns HTTP 503 with sanitized dependency states. A readiness failure does not terminate the process.

### 3.3 Natural-language action

```http
POST /api/v1/chat
Authorization: Bearer <CONTROLLER_API_TOKEN>
Content-Type: application/json
```

Request:

```json
{"message":"开启 3052 这个 VM"}
```

The JSON object must contain exactly one field, `message`. The trimmed message must be non-empty and no longer than the configured character limit, which defaults to 512 Unicode characters. Unknown fields are rejected.

Successful response:

```http
HTTP/1.1 202 Accepted
Content-Type: application/json
X-Request-ID: <request-id>
```

```json
{
  "status": "accepted",
  "message": "VM 3052 的启动请求已提交",
  "tool": "pve_vm_start",
  "arguments": {"vmid": 3052},
  "confidence": 0.92,
  "upstreamStatus": 202,
  "requestId": "..."
}
```

HTTP 202 means Infrastructure Control accepted the action. It does not mean that the VM has completed startup.

## 4. Architecture

Use a layered go-zero REST service:

```text
HTTP handler
  -> application logic
     -> Needle client
     -> tool-call validator
     -> Infrastructure Control client
```

### 4.1 Handler layer

The handler parses strict JSON, obtains the request ID, calls application logic, and renders the success or error envelope. It contains no model-selection or downstream-HTTP business logic.

### 4.2 Application logic

The logic layer orchestrates one stateless request:

1. Validate the user message.
2. Ask Needle to select from the single declared tool.
3. Validate the model output.
4. Call Infrastructure Control with the validated VM ID.
5. Return an accepted response.

Dependency interfaces will permit isolated tests without real model or infrastructure services.

### 4.3 Needle client

The Needle client only communicates with `needle-openai` through:

```http
POST /v1/chat/completions
```

For every request it sends:

- Model ID `needle-2` by default, configurable for endpoint naming only.
- One user message.
- Exactly one tool definition, `pve_vm_start`.
- A bounded `max_tokens` value.
- An optional Needle Bearer token.

It parses standard OpenAI `tool_calls` and the Needle-specific `x_needle` fields used for safety gating.

### 4.4 Tool-call validator

The validator accepts a parsed Needle response and returns a validated VM-start command only if all conditions hold:

- Exactly one tool call exists.
- The function name is exactly `pve_vm_start`.
- Arguments contain valid JSON.
- The argument object contains exactly `vmid` and no unknown fields.
- `vmid` is a JSON integer, is not a boolean or fractional value, and is greater than zero.
- Confidence is present, finite, in the range 0–1, and greater than or equal to `NEEDLE_MIN_CONFIDENCE`.
- `x_needle.validation.ungrounded` is present as an empty array.
- `x_needle.validation.negation` is present and false.

Missing safety metadata fails closed. Multiple calls are rejected rather than partially executed.

### 4.5 Infrastructure Control client

The client exposes the narrow business method:

```go
StartVM(ctx context.Context, vmid int64) error
```

It maps only to:

```http
POST /api/v1/pve/vms/{vmid}/start
Authorization: Bearer <INFRA_CONTROL_API_TOKEN>
Accept: application/json
```

The client does not expose a generic request method. The model and public request cannot set the destination, path, method, headers, or token.

Only HTTP 202 is treated as a successful start submission. Other statuses are mapped to controlled errors. For documented Infrastructure Control errors, only `code`, `message`, and `requestId` may be retained; arbitrary `details` are not returned in phase one.

## 5. Project Structure

```text
needle-controller/
├── Dockerfile
├── .dockerignore
├── docker-compose.yml
├── .env.example
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── needle-controller.api
├── etc/
│   └── needle-controller.yaml
├── cmd/
│   └── needle-controller/
│       └── main.go
└── internal/
    ├── config/
    │   └── config.go
    ├── handler/
    │   ├── healthhandler.go
    │   ├── readyhandler.go
    │   └── chathandler.go
    ├── logic/
    │   └── chatlogic.go
    ├── middleware/
    │   └── bearer_auth.go
    ├── svc/
    │   └── servicecontext.go
    ├── types/
    │   └── types.go
    ├── needle/
    │   ├── client.go
    │   ├── models.go
    │   └── validator.go
    └── infracontrol/
        ├── client.go
        └── models.go
```

Generated go-zero conventions may adjust filenames, but responsibilities and package boundaries must remain as described.

## 6. Configuration

The base go-zero configuration file is `etc/needle-controller.yaml`:

```yaml
Name: needle-controller
Host: 0.0.0.0
Port: 8080
Mode: pro

Needle:
  BaseURL: http://needle-openai:8000
  Model: needle-2
  Timeout: 120s
  MinConfidence: 0.6
  MaxTokens: 256
  MaxMessageLength: 512

InfraControl:
  BaseURL: http://infrastructure-control:8080
  Timeout: 30s
```

Environment variables override deploy-time values:

| Variable | Required | Default | Purpose |
|---|---:|---|---|
| `CONTROLLER_API_TOKEN` | yes | none | Authenticate controller clients. |
| `NEEDLE_BASE_URL` | yes | YAML value | Needle service origin, without `/v1`. |
| `NEEDLE_API_KEY` | no | empty | Optional Needle Bearer token. |
| `NEEDLE_MODEL_ID` | no | `needle-2` | Model identifier sent to Needle. |
| `NEEDLE_MIN_CONFIDENCE` | no | `0.6` | Minimum accepted model confidence. |
| `NEEDLE_TIMEOUT` | no | `120s` | Needle request timeout. |
| `NEEDLE_MAX_TOKENS` | no | `256` | Requested generation limit. |
| `CONTROLLER_MAX_MESSAGE_LENGTH` | no | `512` | Maximum user-message characters. |
| `INFRA_CONTROL_BASE_URL` | yes | YAML value | Infrastructure Control origin. |
| `INFRA_CONTROL_API_TOKEN` | yes | none | Downstream Bearer token. |
| `INFRA_CONTROL_TIMEOUT` | no | `30s` | Infrastructure request timeout. |

`NEEDLE_BASE_URL` and `INFRA_CONTROL_BASE_URL` are considered present when supplied either by valid YAML or environment override. Both API tokens required for their respective enabled authentication paths must never be written into YAML, source control, images, URLs, logs, model prompts, or responses.

Startup fails if the controller token, Infrastructure Control token, or either effective base URL is missing or invalid. Needle's API key may be empty.

## 7. Authentication and Security

- `/healthz` and `/readyz` are public.
- `/api/v1/chat` requires the exact `Authorization: Bearer <CONTROLLER_API_TOKEN>` value.
- Token comparison uses constant-time comparison.
- Authentication failures return the same generic response whether the token is missing or incorrect.
- The public request cannot provide tools, URLs, methods, headers, credentials, model IDs, confidence thresholds, or execution options.
- Needle receives only the user's message and the fixed tool declaration; it never receives infrastructure credentials.
- Logs must not include authorization headers or tokens.
- HTTP clients must not automatically send credentials to redirected hosts. Redirects are rejected or disabled.
- Response bodies and downstream failures are bounded before parsing/logging to avoid unbounded memory use.
- Infrastructure-changing actions are directly executed after all gates pass, as explicitly selected for phase one. There is no confirmation step and no VM allowlist.

Because there is no allowlist or confirmation, deployment must restrict possession of `CONTROLLER_API_TOKEN` and network exposure of the controller.

## 8. Tool Definition

Only this tool is declared to Needle:

```json
{
  "type": "function",
  "function": {
    "name": "pve_vm_start",
    "description": "Start or power on a Proxmox VE virtual machine. 用于开启、启动或开机一个 PVE 虚拟机。",
    "parameters": {
      "type": "object",
      "properties": {
        "vmid": {
          "type": "integer",
          "description": "Proxmox VE 虚拟机的数字 ID，例如 3052"
        }
      },
      "required": ["vmid"],
      "additionalProperties": false
    }
  }
}
```

The controller does not set `tool_choice: "required"`, because Needle cannot enforce it. A model response with no tool call is safely rejected.

## 9. Request and Error Handling

Errors use a stable envelope:

```json
{
  "code": "NEEDLE_LOW_CONFIDENCE",
  "message": "模型置信度不足，未执行操作",
  "requestId": "..."
}
```

| Condition | HTTP | Code |
|---|---:|---|
| Invalid JSON or unknown request field | 400 | `REQUEST_INVALID` |
| Empty or overlong message | 400 | `MESSAGE_INVALID` |
| Missing or incorrect controller token | 401 | `AUTH_UNAUTHORIZED` |
| Needle returns no tool call | 422 | `NEEDLE_NO_TOOL_CALL` |
| Needle returns multiple calls | 422 | `NEEDLE_MULTIPLE_TOOL_CALLS` |
| Tool is not allowed | 422 | `NEEDLE_TOOL_NOT_ALLOWED` |
| Arguments are malformed or have extra fields | 422 | `NEEDLE_ARGUMENTS_INVALID` |
| VM ID is not a positive integer | 422 | `NEEDLE_VMID_INVALID` |
| Safety metadata is missing/invalid | 422 | `NEEDLE_SAFETY_METADATA_INVALID` |
| Confidence is below threshold | 422 | `NEEDLE_LOW_CONFIDENCE` |
| Arguments are ungrounded | 422 | `NEEDLE_ARGUMENTS_UNGROUNDED` |
| Negation is detected | 422 | `NEEDLE_NEGATION_DETECTED` |
| Needle is unreachable, times out, or returns invalid data | 502 | `NEEDLE_UNAVAILABLE` |
| Needle reports a full queue/rate limit | 503 | `NEEDLE_BUSY` |
| Infrastructure Control returns 401/403 | 502 | `INFRA_CONTROL_AUTH_FAILED` |
| Infrastructure Control returns another non-202 response | 502 | `INFRA_CONTROL_REQUEST_FAILED` |
| Infrastructure Control request times out | 504 | `INFRA_CONTROL_TIMEOUT` |
| Unexpected internal failure | 500 | `INTERNAL_ERROR` |

Error messages must distinguish rejection from execution. Validation failures guarantee that Infrastructure Control was not called.

## 10. Request IDs and Logging

- Accept a syntactically safe inbound `X-Request-ID`; otherwise generate one.
- Return the effective ID in `X-Request-ID` and every JSON response.
- Propagate the effective ID to Infrastructure Control as `X-Request-ID` for correlation.
- Emit structured logs for request ID, route, result code, selected tool, VM ID after validation, confidence, upstream status, and duration.
- Do not log tokens, authorization headers, full model prompts, unrestricted upstream bodies, or raw confidential errors.

## 11. Concurrency and Statelessness

The controller stores no conversation history and no pending confirmations. Each call is independent and suitable for horizontal scaling.

Needle serializes inference internally. The controller does not add an unbounded queue or retries for action requests. A Needle 429 is returned as a retryable service-unavailable error. The controller must not automatically retry Infrastructure Control start requests because acceptance may have occurred even when the response is lost.

## 12. Docker Deployment

Use a multi-stage build:

- Build with a pinned Go Alpine image compatible with the chosen go-zero version.
- Run `CGO_ENABLED=0` and produce a trimmed static binary.
- Use a pinned distroless non-root runtime image.
- Copy only the binary, CA certificates supplied by the runtime, and non-secret config.
- Run as non-root.
- Expose port 8080.
- Handle `SIGTERM` through go-zero's graceful server shutdown.
- Do not bake secrets into any image layer.

The Compose service will:

- Publish `${CONTROLLER_PORT:-8080}:8080`.
- Inject all secrets and endpoint overrides through environment variables.
- Use `read_only: true`, a `/tmp` tmpfs, `no-new-privileges`, and drop all Linux capabilities.
- Restart unless stopped.
- Define a health check against `/healthz` using a mechanism available in the selected runtime image or Docker's external health handling; the Dockerfile must not claim an unavailable in-image utility.

The deployment may connect to externally hosted Needle and Infrastructure Control services; they need not be in the same Compose project.

## 13. Testing Strategy

Implementation follows test-driven development. All tests run in a Docker execution environment, never in the local Pi workspace.

### 13.1 Needle client tests

Using `httptest.Server`, verify:

- POST path `/v1/chat/completions`.
- Optional Bearer authentication.
- One user message and exactly one fixed tool.
- Bounded `max_tokens`.
- Parsing of one tool call and `x_needle` safety metadata.
- Mapping of non-2xx responses, 429, timeouts, oversized bodies, malformed JSON, and redirects.

### 13.2 Validator table tests

Cover:

- Valid `pve_vm_start({"vmid":3052})` with passing safety metadata.
- No call, multiple calls, and an unknown function.
- Malformed JSON, missing `vmid`, string/fractional/boolean/zero/negative/overflowing `vmid`, duplicate JSON keys if the parser permits detection, and extra fields.
- Missing, NaN/non-finite, out-of-range, and low confidence.
- Missing/non-array/non-empty `ungrounded`.
- Missing/non-boolean/true `negation`.

### 13.3 Infrastructure Control client tests

Using `httptest.Server`, verify:

- Fixed POST path for the validated VM ID.
- Correct downstream Bearer token and propagated request ID.
- Absence of Needle and controller credentials.
- HTTP 202 is accepted; HTTP 200 is not treated as an accepted asynchronous action.
- Mapping of 401/403, documented errors, non-JSON errors, timeouts, oversized bodies, and redirects.
- No automatic retry of the mutation request.

### 13.4 Logic tests

With fake interfaces, verify:

- Exact sequence: Needle, validator, Infrastructure Control.
- Needle failures and validation failures never invoke Infrastructure Control.
- Only a fully validated command invokes `StartVM` once.
- The validated VM ID and request ID are propagated unchanged.
- Downstream 202 maps to the public accepted result.

### 13.5 HTTP tests

Verify:

- Public liveness and readiness behavior.
- Missing/wrong controller token returns 401.
- Correct token permits access.
- Strict JSON rejects unknown fields and trailing JSON values.
- Empty and overlong messages return 400.
- Successful processing returns 202 and request IDs in header/body.
- Error mappings and JSON content type.
- Tokens never appear in response bodies or captured logs.

### 13.6 Docker verification

In Docker, run and inspect:

```text
go test ./...
go test -race ./...
go vet ./...
```

Then build the production image and verify:

- It runs as non-root.
- `/healthz` succeeds.
- `/readyz` reflects mocked or configured dependencies.
- No secret exists in image configuration or layers introduced by the project.
- A real test-stack request `开启 3052 这个 VM` produces one accepted Infrastructure Control request and a controller HTTP 202.
- Negative, low-confidence, malformed, and ungrounded cases make zero Infrastructure Control requests.

## 14. Acceptance Criteria

Phase one is complete when:

1. An authenticated `POST /api/v1/chat` with `开启 3052 这个 VM` causes Needle to select `pve_vm_start` with VM ID 3052 under a passing confidence/safety result.
2. The controller validates that result and submits exactly one `POST /api/v1/pve/vms/3052/start` request with the downstream Bearer token.
3. Infrastructure Control HTTP 202 produces controller HTTP 202 with wording that the request was submitted, not that startup completed.
4. Authentication, malformed requests, missing/no/multiple/wrong tool calls, invalid VM IDs, low confidence, missing safety metadata, ungrounded arguments, and negation cannot invoke Infrastructure Control.
5. Health and readiness endpoints behave as specified.
6. Automated tests, race tests, vet, production image build, and container smoke tests pass in Docker.
7. Tokens are absent from source-controlled configuration, model prompts, normal responses, and captured logs.
8. README documents configuration, Compose deployment, API use, 202 semantics, and the intentionally limited first-phase scope.
