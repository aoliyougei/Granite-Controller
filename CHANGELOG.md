# Changelog

## v1.0.1

- Hide `pve_vm_stop` from Granite whenever force stop is disabled, preventing the 350M model from confusing normal shutdown with an unavailable force-stop tool.
- Keep strict post-model action evidence checks; ambiguous model output is still rejected rather than rewritten.

## v1.0.0

- Replace Needle 2 with pinned IBM Granite 4.0 350M Q4_K_M and a Go-managed loopback-only llama-server.
- Add five fixed PVE VM tools: query, start, graceful shutdown, force stop, and reboot.
- Add strict final-user input handling, tool/VM grounding, mutation state checks, and default-disabled force stop.
- Add 30-second in-process mutation deduplication to prevent repeated Infrastructure submissions.
- Add fixed Chinese/Emoji responses, query field whitelisting, and readable uptime.
- Disable request-dump logging and record only safe metadata.
- Bundle verified offline model/runtime artifacts in a non-root, read-only-capable image.
- Real Granite acceptance: 87/100 correct candidates, 13 safe refusals, zero dangerous executable mismatches.

## v0.3.2

- Raise both go-zero and Chat Completions request limits from 1 MiB to 8 MiB so Pi Agent envelopes containing system context and tool schemas reach the managed final-user parser.
- Keep a bounded 8 MiB limit; oversized requests are rejected before Needle inference or Infrastructure execution.

## v0.3.1

- Accept OpenAI final-user content as either a string or one or more plain `{type: "text", text: "..."}` content parts.
- Join multiple plain text parts in order for Pi and other OpenAI-compatible Agent clients.
- Continue to reject empty parts, images, audio, files, unknown part types, and non-text content without falling back to historical user messages.

## v0.3.0

### Breaking changes

- `/v1/chat/completions` is now managed auto-execution rather than general dynamic tool selection.
- Client-provided tools, tool choice, system prompts, and history are accepted but ignored.
- Only the final plain-text user message is processed; no backward search is performed.
- Responses contain final Chinese assistant text and no `tool_calls`.
- Added required `INFRA_CONTROL_API_BASE_URL` and `INFRA_CONTROL_API_TOKEN`.
- Restored server-side Infrastructure Control execution for one built-in tool: `pve_vm_start`.
- Removed dynamic schema validation and multi-turn tool-result replay.

### Safety

- Chinese rules only validate and normalize input; embedded Needle 2 remains a mandatory semantic selector.
- Model VM ID, confidence, grounding, negation, call count, and tool name must all pass before execution.
- Infrastructure requests use a fixed method/path, reject redirects, accept only HTTP 202, and are never retried automatically.
- Use v0.2.0 for general client-provided OpenAI tools without server execution.

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
