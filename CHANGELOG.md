# Changelog

## v0.2.0

### Breaking changes

- Removed `POST /api/v1/chat`.
- Removed Infrastructure Control execution and all `INFRA_CONTROL_*` configuration.
- Removed `CONTROLLER_API_TOKEN` and the PVE-specific Chinese normalization flow.
- Removed the remote `NEEDLE_BASE_URL` / separate `needle-openai` runtime dependency.
- Added required `NEEDLE_API_KEY` Bearer authentication for `/v1/*`.
- Added OpenAI-compatible `GET /v1/models` and `POST /v1/chat/completions`.
- Embedded pinned Needle Engine 2.0.3 for Linux AMD64 and invoked it directly through CGO.
- Tools are selected but never executed by the server.

### Added

- Dynamic OpenAI function tools with a strict JSON Schema subset.
- Standard user/tool history replay with request-level native isolation.
- Safe `tool_choice` subset.
- Synthesized OpenAI SSE streaming and optional usage chunks.
- Bounded single-thread native dispatcher and asynchronous model readiness.
- Offline runtime image with verified engine hashes and bundled attribution.

### Migration

External agents now own business-language normalization, tool execution, authorization, argument validation, confidence policy, and confirmation for risky actions. They should pass tool definitions to `/v1/chat/completions`, execute returned calls, and send tool results back using standard OpenAI messages.

The immutable `v0.1.1` Git and image tags remain the final release of the PVE-specific automatic execution service.

## v0.1.1

- Moved controller source files to the repository root.

## v0.1.0

- Initial PVE VM-start controller with remote Needle OpenAI inference and Infrastructure Control execution.
