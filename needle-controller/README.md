# needle-controller

A stateless go-zero REST controller that asks Needle 2 to select a narrowly scoped infrastructure tool, validates the result, and submits a Proxmox VE VM start request.

Phase one supports only:

```text
Natural language -> pve_vm_start({"vmid": ...}) -> POST /api/v1/pve/vms/{vmid}/start
```

## Safety boundary

`POST /api/v1/chat` performs the action immediately after validation. Phase one has **no confirmation flow and no VM allowlist**. Restrict network exposure and possession of `CONTROLLER_API_TOKEN`.

The controller rejects execution unless Needle returns exactly one `pve_vm_start` call, a positive integer VM ID, confidence at or above the configured threshold, an empty `ungrounded` list, and `negation: false`. It never accepts model-provided URLs, methods, headers, or credentials.

## Configuration

Copy `.env.example` to `.env`. Required secrets:

- `CONTROLLER_API_TOKEN`: authenticates callers of the controller.
- `INFRA_CONTROL_API_TOKEN`: authenticates the controller to Infrastructure Control.
- `NEEDLE_API_KEY`: optional; set it when `needle-openai` requires authentication.

Required service origins:

- `NEEDLE_BASE_URL`, for example `http://needle-openai.example:8000` (without `/v1`).
- `INFRA_CONTROL_BASE_URL`, for example `http://infrastructure-control.example:8080`.

`NEEDLE_MIN_CONFIDENCE` defaults to `0.6`. Calibrate it with your own Chinese instructions before production use.

## Run with Docker Compose

```bash
cp .env.example .env
# Edit .env, then:
docker compose up -d --build
docker compose ps
```

The image runs as a non-root distroless user with a read-only filesystem, no Linux capabilities, and an executable health check built into the controller binary.

## Health and readiness

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s http://127.0.0.1:8080/readyz
```

- `/healthz` checks only the controller process.
- `/readyz` checks Needle `/health` and Infrastructure Control `/readyz`.
- Both endpoints are public for container/Kubernetes probes.

## Start a VM

```bash
curl -i http://127.0.0.1:8080/api/v1/chat \
  -H "Authorization: Bearer ${CONTROLLER_API_TOKEN}" \
  -H 'Content-Type: application/json' \
  -d '{"message":"开启 3052 这个 VM"}'
```

A successful submission returns HTTP 202:

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

**HTTP 202 means the start request was accepted by Infrastructure Control. It does not mean the VM has finished starting.** Phase one does not poll VM state.

## Development

Run project tooling in an appropriate Go or Docker environment:

```bash
make test
make race
make vet
make build
```

The tests use `httptest.Server`; they do not contact or mutate real infrastructure.

## Current limitations

No status query, shutdown, stop, reboot, physical-host control, workflow execution, multi-turn conversation, confirmation, VM allowlist, or Kubernetes manifests are included in phase one.
