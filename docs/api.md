# External Infrastructure Control REST API

> **v0.3.0 wires exactly one operation from this catalogue:** `POST /api/v1/pve/vms/{vmid}/start`, exposed as the internally managed `pve_vm_start` tool. `needle-controller` does not support or execute any other operation listed here yet. Client-provided tools are ignored; future managed operations require explicit code, safety policy, tests, and release work.

Infrastructure Control exposes a go-zero REST API on `INFRA_CONTROL_HTTP_HOST:INFRA_CONTROL_HTTP_PORT`. All responses are JSON except raw JSONL task-log downloads and SSE streams.

## Authentication

`GET /healthz` and `GET /readyz` are public. Every `/api/v1/*` request requires the exact static token:

```http
Authorization: Bearer <INFRA_CONTROL_API_TOKEN>
```

Missing or incorrect credentials return `401`. Never put the token in a URL, workflow, log, or image. Rotate it by changing the injected environment/Secret and restarting the single application replica.

## Health and system

| Method | Path | Meaning |
|---|---|---|
| GET | `/healthz` | Process is serving HTTP. |
| GET | `/readyz` | Configuration, writable workflow/log paths, miIO runtime, and watcher are ready. External devices may remain offline. |
| GET | `/api/v1/system/info` | Name, version, commit, and build time. |

## Direct device APIs

Read operations complete synchronously. Mutation endpoints submit one external action and return `202`; they do **not** wait for the final device state. Use an asynchronous workflow for ordered actions and waits.

### Proxmox VE

```text
GET  /api/v1/pve/vms
GET  /api/v1/pve/vms/{vmid}
POST /api/v1/pve/vms/{vmid}/start
POST /api/v1/pve/vms/{vmid}/shutdown
POST /api/v1/pve/vms/{vmid}/stop
POST /api/v1/pve/vms/{vmid}/reboot
GET  /api/v1/pve/nodes
GET  /api/v1/pve/nodes/{node}
POST /api/v1/pve/nodes/{node}/shutdown
GET  /api/v1/pve/backup-jobs
GET  /api/v1/pve/tasks/{upid}
GET  /api/v1/pbs/status
```

`stop` and node shutdown are dangerous. They must be deliberately requested and are never performed by health checks.

### Configured physical hosts

```text
GET  /api/v1/hosts
GET  /api/v1/hosts/{alias}/status
POST /api/v1/hosts/{alias}/start
POST /api/v1/hosts/{alias}/shutdown
```

`start` sends WOL using startup configuration. `shutdown` uses only the PVE node or SSH command loaded at startup; API callers cannot provide arbitrary commands.

### Configured miIO powers

```text
GET  /api/v1/powers
GET  /api/v1/powers/{alias}/status
POST /api/v1/powers/{alias}/on
POST /api/v1/powers/{alias}/off
POST /api/v1/powers/{alias}/toggle
GET  /api/v1/powers/{alias}/diagnostics
```

Power tokens are never returned. `off` can cause data loss if a workload is still running; use an explicitly ordered workflow with status checks.

## Workflow-step discovery

No fixed business workflows are installed. Discover supported controlled steps and build your own:

```text
GET /api/v1/workflow-step-categories
GET /api/v1/workflow-steps
GET /api/v1/workflow-steps?category=pve.vm
GET /api/v1/workflow-steps?search=shutdown
GET /api/v1/workflow-steps/{uses}
```

A step detail contains its risk, input/output schemas, retry/cancellation metadata, defaults, and one minimal YAML fragment. This registry is also the executor and validator source of truth.

## Workflow management

```text
GET    /api/v1/workflows
GET    /api/v1/workflows/{name}
POST   /api/v1/workflows
PUT    /api/v1/workflows/{name}
DELETE /api/v1/workflows/{name}
POST   /api/v1/workflows/validate
POST   /api/v1/workflows/reload
POST   /api/v1/workflows/{name}/reload
POST   /api/v1/workflows/{name}/run
```

Create/update/validate accept JSON or YAML. Successful writes use a temporary same-directory file, sync, and atomic rename, then publish an immutable in-memory snapshot. `ETag` and optional `If-Match` protect against overwriting a manually changed file. API-created or manually edited valid files are immediately usable; an invalid edit leaves the prior valid snapshot active while exposing validation errors.

Run returns `202`:

```json
{"taskId":"01...","workflow":"nas-startup","workflowVersion":"sha256:...","status":"pending","createdAt":"..."}
```

## Tasks, logs, and events

```text
GET  /api/v1/tasks
GET  /api/v1/tasks/{taskId}
POST /api/v1/tasks/{taskId}/cancel
GET  /api/v1/tasks/{taskId}/log
GET  /api/v1/tasks/{taskId}/log?format=jsonl
GET  /api/v1/tasks/{taskId}/events
```

Tasks are process-local. A restart intentionally removes task state and interrupts running work; mounted workflow YAML and JSONL files remain. Cancellation stops local waiting/process work but cannot reverse an action already accepted by PVE, SSH, WOL, or miIO.

The SSE endpoint emits a connected comment, JSON `data:` events, and heartbeat comments. Task errors, outputs, log records, and SSE events pass through the public sanitizer.

## Errors

Errors use stable codes:

```json
{"code":"PVE_VM_API_NOT_READY","message":"target VM API is not ready","requestId":"...","details":{"vmid":1006,"node":"pve-core"}}
```

Families include `AUTH_*`, `CONFIG_*`, `WORKFLOW_*`, `TASK_*`, `PVE_*`, `PBS_*`, `HOST_*`, `SSH_*`, `WOL_*`, `MIIO_*`, and `INTERNAL_*`. Credentials and raw authorization headers are excluded.
