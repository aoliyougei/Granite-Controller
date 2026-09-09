# Granite Controller v1.0.0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a new history-preserving `granite-controller` project that runs Granite 4.0 350M Q4_K_M through a managed llama-server, safely selects and executes five PVE VM operations from natural Chinese, and exposes final OpenAI-compatible responses.

**Architecture:** Go PID 1 accepts only the final user text, applies generic pre-model safety guards, and sends the unchanged Chinese message plus five fixed bilingual tools to a localhost llama-server child. Strict post-model validation, mutation deduplication, VM state preconditions, a narrow Infrastructure Control client, fixed Chinese/Emoji rendering, and sanitized logging prevent model or client output from directly controlling arbitrary HTTP behavior.

**Tech Stack:** Go 1.24, go-zero REST, IBM Granite 4.0 350M Q4_K_M GGUF, pinned llama.cpp/llama-server, standard-library HTTP/process management, Docker Linux AMD64.

**Spec:** `docs/superpowers/specs/2026-09-09-granite-controller-v1.0.0-design.md`

## Global Constraints

- Work in a new `granite-controller` repository/directory with full `needle-controller` history; do not rename or modify the original product repository after the migration commit.
- Module path is exactly `github.com/aoliyougei/granite-controller`.
- First formal release is `v1.0.0`; no reused `needle-controller` version tags are pushed to the new remotes unless explicitly required to preserve history, and release naming must not overwrite original repositories.
- Runtime model is official dense `ibm-granite/granite-4.0-350m-GGUF` Q4_K_M, not the hybrid model.
- Pin and verify GGUF revision/hash, llama.cpp commit, llama-server hash, and both licenses before image publication.
- One image/Pod contains Go PID 1 plus one llama-server child; child is localhost-only, Jinja-enabled, parallel 1, deterministic inference.
- Unexpected child exit or startup timeout exits the Go process; no in-container restart loop.
- Only the final user string/text parts reach Granite; Pi system/history/tools are ignored and never logged.
- Granite receives original trimmed Chinese unchanged and all five fixed bilingual tools.
- Controller pre-model checks only generic unsafe structures; Granite chooses the tool; controller post-validates tool, VM ID, negation, and minimal semantic evidence.
- No calibrated confidence field exists; never use model-generated confidence as an execution gate.
- Five tools only: get, start, shutdown, stop, reboot. Force stop defaults disabled.
- Mutations deduplicate by tool+VM for 30 seconds, check current state before POST, reject redirects, and never retry mutation POSTs.
- Query results whitelist vmid/name/status/uptime and render approved Emoji/readable uptime.
- Disable go-zero request dump logging and add safe metadata-only access/business logs.
- Preserve 8 MiB OpenAI envelope limit; user text defaults to 2048 runes; Granite context defaults 4096.
- All installs, builds, tests, model execution, and image operations use Docker API tools.
- TDD every behavior; commit each independently reviewable task.
- Real Infrastructure mutations require explicit authorization after mock/model safety gates pass.
- Do not publish Git `v1.0.0` or image until explicit release authorization after RC evidence.

## Planned File Map

- `build/granite-model.env`, `build/fetch_granite.py`: pinned GGUF retrieval/hash gate.
- `build/llama-cpp.env`: pinned llama.cpp source commit and expected binary hash.
- `internal/granite/process.go`: child lifecycle/readiness/failure/shutdown.
- `internal/granite/client.go`: deterministic localhost OpenAI tool request and response parsing.
- `internal/managed/input.go`: final user extraction and ignored categories.
- `internal/managed/safety.go`: generic pre-model structure and post-model evidence checks.
- `internal/managed/tools.go`: five fixed bilingual tools and action metadata.
- `internal/managed/service.go`: inference, dedup, state query, mutation, final response.
- `internal/dedup/store.go`: in-flight, success, no-op, and unknown-status caching with a controllable clock.
- `internal/infracontrol/client.go`: GetVM and four fixed mutation methods with safe errors.
- `internal/render/vm.go`: Emoji status/action text and uptime formatting.
- `internal/safelog/`: metadata-only access/business logging.
- Remove `internal/native/`, Needle artifacts/licenses/configuration.

---

### Task 1: Create the New History-Preserving Repository

**Files:**
- New project directory: sibling `granite-controller/`
- Modify in new project: `go.mod`, README identity, remote configuration

**Interfaces:**
- Produces an independent Git repository with source history through the approved Granite design/plan commit.
- Original `needle-controller` HEAD/tags/remotes remain byte-identical.

- [ ] **Step 1: Capture original invariants**

Record literal original repository root, HEAD, remotes, tags and clean status. Verify `v0.3.2` commit.

- [ ] **Step 2: Clone locally into a sibling project without network**

Use Git clone/shared-object-safe copy into `/app/project/granite-controller` (or a harness-approved isolated location), then remove/replace remotes only in the new repository. Never move the active session directory.

- [ ] **Step 3: Verify history and isolation**

Assert new HEAD equals source HEAD, commit count matches, original worktree remains clean, and writes/branches in the new repo do not affect original refs.

- [ ] **Step 4: Rename module and imports with a compile-failing test checkpoint**

Change `go.mod` to `module github.com/aoliyougei/granite-controller`, update all imports, and run `CGO_ENABLED=0 go test ./...` in Docker. Expected intermediate failures must only identify Needle components scheduled for removal, not stale module imports.

- [ ] **Step 5: Commit repository identity**

```bash
git add go.mod go.sum .
git commit -m "chore: create granite-controller project"
```

### Task 2: Granite and llama.cpp Artifact/License Gate

**Files:**
- Create: `build/granite-model.env`
- Create: `build/fetch_granite.py`, tests
- Create: `build/llama-cpp.env`
- Create: `licenses/granite/`, `licenses/llama.cpp/`, notices

**Interfaces:**
- Records literal HF revision/GGUF SHA-256 and llama.cpp commit/server SHA-256.
- Fetcher emits only a verified GGUF.

- [ ] **Step 1: Probe official GGUF metadata/license**

Inspect HF model card/API, exact Q4_K_M artifact, LICENSE/metadata, size and revision. Stop publication if redistribution evidence is ambiguous.

- [ ] **Step 2: Download twice independently and hash**

Compute identical GGUF hash in two fresh containers. Record no placeholders.

- [ ] **Step 3: Select llama.cpp commit and verify Granite tool calling**

Choose a current pinned commit with Granite/Jinja/OpenAI tools support. Build llama-server and run a no-execution Chinese five-tool probe before adopting the commit.

- [ ] **Step 4: Write RED fetcher tests**

Wrong hash, truncated file, download failure/retry, atomic output, existing output refusal.

- [ ] **Step 5: Implement fetcher and licenses**

Standard library only; limited transient retries; hash before rename.

- [ ] **Step 6: Commit artifact gate**

```bash
git add build licenses
git commit -m "build: pin Granite and llama.cpp artifacts"
```

### Task 3: Granite Configuration

**Files:**
- Rewrite: `internal/config/config.go`, tests, YAML, `.env.example`

**Interfaces:**
- Produces Granite/process/infrastructure/dedup configs exactly from spec section 20.

- [ ] **Step 1: Write RED config table tests**

Required keys, defaults/ranges, URL validation, bool, durations, no NEEDLE fallback, secret-free errors, 8 MiB MaxBytes, safe log disabled.

- [ ] **Step 2: Implement and GREEN**

Remove all NEEDLE config; validate threads 1..16, context 1024..8192, positive tokens/message, startup/request/upstream/dedup durations.

- [ ] **Step 3: Commit**

```bash
git add internal/config etc .env.example
git commit -m "feat: configure Granite managed execution"
```

### Task 4: Fixed Five-Tool Catalogue

**Files:**
- Create: `internal/managed/tools.go`, tests

**Interfaces:**
- `Action` enum and metadata: tool name, method suffix, required state, mutation, evidence phrases.
- `GraniteTools() []OpenAITool` returns exact bilingual schema.

- [ ] **Step 1: Write RED literal catalogue tests**

Assert five unique names, bilingual descriptions, strict vmid schema, action metadata, stop risk.

- [ ] **Step 2: Implement fixed catalogue and GREEN**

No config/client extension mechanism.

- [ ] **Step 3: Commit**

### Task 5: Final User Input and Generic Pre-Model Safety

**Files:**
- Adapt: `internal/managed/input.go`, tests
- Create: `internal/managed/safety.go`, tests

**Interfaces:**
- `Input{Original, VMID, HasNegation, Warnings}`.
- `ValidatePreModel(Input) *apierror.Error`.

- [ ] **Step 1: Preserve RED/GREEN Pi final-message tests**

String/text parts, no fallback, ignored contexts, 8 MiB.

- [ ] **Step 2: Write RED safety tables**

Single ID, condition, batch, sequence, multiple explicit actions, ambiguous stop/reset, negated query allowed, negated mutations undecided until post-model.

- [ ] **Step 3: Implement generic guards**

No full phrase-to-action normalization; original text unchanged.

- [ ] **Step 4: Commit**

### Task 6: llama-server Process Manager

**Files:**
- Create: `internal/granite/process.go`, tests

**Interfaces:**
- `Manager.Start(ctx)`, `Ready`, `Wait`, `Stop`; injectable command/probe/exit hooks.

- [ ] **Step 1: RED process tests**

Exact arguments, startup readiness, timeout, unexpected exit, graceful stop, no restart, no raw stdout/stderr forwarding.

- [ ] **Step 2: Implement child lifecycle**

Use `exec.CommandContext` carefully so parent request cancellation does not kill shared child; process-owned context, sanitized lifecycle logs.

- [ ] **Step 3: Race tests and commit**

### Task 7: Granite OpenAI Client and Tool Parser

**Files:**
- Create: `internal/granite/client.go`, models/tests

**Interfaces:**
- `Select(ctx,text,tools) (ToolCall,*apierror.Error)`.

- [ ] **Step 1: RED exact-request test**

Localhost URL, unchanged Chinese, five tools, deterministic params, stream false, timeout, no Pi history.

- [ ] **Step 2: RED response/error tests**

One call success; free text/no call Chinese 422; multiple call 422; malformed args; timeout/connect/HTTP errors; bounded body; no retry.

- [ ] **Step 3: Implement and commit**

### Task 8: Post-Model Validation

**Files:**
- Create: `internal/managed/validate.go`, tests

**Interfaces:**
- `ValidateToolCall(Input, granite.ToolCall, allowForceStop) (ActionCall,*apierror.Error)`.

- [ ] **Step 1: RED exhaustive tests**

Allowlist, one vmid, positive integer, ID match, negated query vs mutation, evidence matrix, ambiguous mutation, force-stop gate.

- [ ] **Step 2: Implement deterministic validation**

No confidence/logprob/self-reported fields.

- [ ] **Step 3: Commit**

### Task 9: Infrastructure Client, Safe Errors, VM Result

**Files:**
- Rewrite: `internal/infracontrol/client.go`, tests

**Interfaces:**
- `GetVM`, `Mutate(action, vmid)` fixed methods; `VM{VMID,Name,Status,Uptime}`; typed known/explicit/unknown errors.

- [ ] **Step 1: RED GET/POST/path/auth tests**

All five mappings, one request, request ID, redirects, no retry.

- [ ] **Step 2: RED safe error tests**

Bounded code/message/requestId, details dropped, Chinese code mapping, network class, unknown acceptance for transport errors.

- [ ] **Step 3: Implement and commit**

### Task 10: Mutation State Preconditions

**Files:**
- Create: `internal/managed/state.go`, tests

**Interfaces:**
- Determines execute vs no-op from action/current VM.

- [ ] **Step 1: RED action/state matrix**

start stopped; shutdown/stop/reboot running; query; all mismatch texts.

- [ ] **Step 2: Implement and commit**

### Task 11: Concurrent Dedup Store

**Files:**
- Create: `internal/dedup/store.go`, tests

**Interfaces:**
- `Do(ctx,key,fn) (Result,deduplicated,error)` with controllable clock and error cache classification.

- [ ] **Step 1: RED concurrency/expiry tests**

In-flight one execution, success/no-op cache, explicit failure no cache, unknown cache, action/VM isolation, query bypass, cancellation, expiry.

- [ ] **Step 2: Implement minimal mutex/channel map**

No cleanup goroutine unless necessary; prune on access.

- [ ] **Step 3: Race and commit**

### Task 12: Rendering and OpenAI/SSE Responses

**Files:**
- Create: `internal/render/vm.go`, tests
- Adapt response/SSE models

- [ ] **Step 1: RED uptime/Emoji/name tests**

All time boundaries and approved text.

- [ ] **Step 2: RED metadata/SSE tests**

x_granite, executed/dedup, whitelisted result, no tool_calls, role/content/stop/usage/DONE.

- [ ] **Step 3: Implement and commit**

### Task 13: Managed Service Orchestration

**Files:**
- Rewrite: `internal/managed/service.go`, tests

**Interfaces:**
- Exact flow final input -> precheck -> Granite -> postcheck -> query/dedup/state/mutation -> response.

- [ ] **Step 1: RED call-order and bypass tests**

Original Chinese reaches Granite, all five tools; no rule-direct API; every reject zero upstream mutation.

- [ ] **Step 2: RED state/dedup/error tests**

Query; mutation no-op; mutation success; duplicate; explicit vs unknown failure.

- [ ] **Step 3: Implement and race-test**

- [ ] **Step 4: Commit**

### Task 14: Secure HTTP Logging and Wiring

**Files:**
- Create: `internal/safelog/` and tests
- Modify: handlers/routes/service context/main/config
- Remove native/Needle packages.

- [ ] **Step 1: RED sentinel leakage tests**

5xx request with Authorization/system/history/tool/user sentinels; captured logs contain none; allowed metadata present.

- [ ] **Step 2: Disable go-zero request dump and install safe logger**

Ensure no duplicate middleware logs requests.

- [ ] **Step 3: Wire Granite manager/client/dedup/upstream/service**

HTTP starts before Granite ready; child exit shuts process down through testable lifecycle signal.

- [ ] **Step 4: Delete Needle runtime/artifacts/config/licenses**

No `libneedle`, CGO, native dispatcher, NEEDLE names outside historical docs/changelog.

- [ ] **Step 5: Full tests/race/vet and commit**

### Task 15: Four-Stage Offline Image

**Files:**
- Rewrite Dockerfile, Compose, Makefile, ignores

- [ ] **Step 1: RED image build before artifacts wired**

- [ ] **Step 2: Implement pinned GGUF/llama/go/runtime stages**

- [ ] **Step 3: Inspect dynamic dependencies and final contents**

No compiler/git/python/Needle; nonroot/read-only/offline startup.

- [ ] **Step 4: Measure startup/RSS/latency under 2 GiB**

- [ ] **Step 5: Commit**

### Task 16: Real Granite Chinese Safety Acceptance

**Files:**
- Create versioned JSONL cases and evaluator

- [ ] **Step 1: At least 20 variants per action plus negative classes**

Literal expected action/id/reject classification.

- [ ] **Step 2: Run real Q4_K_M against all cases with mock upstream**

Report confusion matrix, rejection, wrong tool, wrong ID, and actual mock mutation counts.

- [ ] **Step 3: Hard gate**

Any wrong mutation reaching mock upstream blocks release. Do not add broad phrase mappings to hide model failure; adjust tool descriptions/safety or reconsider model.

- [ ] **Step 4: Commit suite/results policy**

### Task 17: Documentation, New Remotes, and RC

**Files:**
- Rewrite README/CHANGELOG/API/module identity docs

- [ ] **Step 1: Document model, five tools, risks, dedup, state, logs, resources, Pi config**

- [ ] **Step 2: Validate every documented request against RC/mocks**

- [ ] **Step 3: Create empty/new GitHub and GitLab remote URLs only if they exist; never push to needle-controller remotes**

- [ ] **Step 4: Independent review focused on model safety/process/logging/dedup**

- [ ] **Step 5: Fresh unit/race/vet/model/image/offline/security verification**

- [ ] **Step 6: Authorized real integration**

Query/start/shutdown only with explicit current authorization; force stop requires config and explicit authorization.

- [ ] **Step 7: Report RC commit/digest/metrics without publishing**

- [ ] **Step 8: Release only on explicit user request**
