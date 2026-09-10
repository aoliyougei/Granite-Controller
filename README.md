# granite-controller

OpenAI-compatible Chinese controller for five Proxmox VE VM operations. IBM Granite 4.0 350M Q4_K_M selects one fixed tool; Go validates and executes it through Infrastructure Control.

## Safety boundary

- Only the final `user` text is processed; history, system prompts, and client tools are ignored.
- Exactly one VM ID and one Granite tool call are required.
- Conditional, batch, sequential, negated mutation, and ambiguous mutation requests are rejected.
- Mutations query current VM state first and are deduplicated for 30 seconds.
- Force stop is disabled unless `GRANITE_ALLOW_FORCE_STOP=true`.
- Mutation POSTs are never automatically retried.
- Run one replica in v1.0.0; deduplication is process-local.

## API

```text
GET  /healthz
GET  /readyz
GET  /v1/models
POST /v1/chat/completions
```

`/v1/*` requires `Authorization: Bearer $GRANITE_API_KEY`.

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $GRANITE_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"granite-4.0-350m","messages":[{"role":"user","content":"查询 VM 3052 当前状态"}]}'
```

Five managed tools:

```text
pve_vm_get
pve_vm_start
pve_vm_shutdown
pve_vm_stop
pve_vm_reboot
```

## Runtime

One image contains Go PID 1, a loopback-only llama-server child, and the pinned GGUF. Runtime performs no downloads. If llama-server fails readiness within 120 seconds or exits, the controller exits so Kubernetes can restart the Pod.

Copy `.env.example` to `.env` and provide new secrets. The leaked legacy `NEEDLE_API_KEY` must not be reused.

Recommended baseline: request 1 CPU/1 GiB, limit 4 CPU/2 GiB. Run non-root with read-only root and `/tmp` tmpfs.

## Verification

```bash
go test ./...
go test -race ./...
go vet ./...
docker build -t granite-controller:rc .
```

Builds pin Granite revision/hash and llama.cpp commit in `build/`. See `docs/superpowers/specs/2026-09-09-granite-controller-v1.0.0-design.md` for the complete execution and error contract.
