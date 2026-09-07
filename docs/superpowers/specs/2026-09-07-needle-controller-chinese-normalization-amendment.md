# Needle Controller Chinese Normalization Amendment

**Date:** 2026-09-07  
**Status:** Approved and implemented

This amendment records the user-approved requirement that the controller's public interaction remain Chinese after live Needle 2 testing showed unsafe behavior for direct Chinese inference.

## Live evidence

With the original tool schema, real `needle-openai` returned five calls and hallucinated VM IDs for `开启 3052 这个 VM`, with confidence `0.0` and non-empty `ungrounded`. The English normalization `Start VM 3052` returned one correct call with confidence `0.9044` and empty `ungrounded`.

## Required behavior

Before calling Needle, the controller:

1. Requires a Chinese start phrase: `开启`, `启动`, `开机`, `打开`, or `开起来`.
2. Requires an explicit `VM` token or `虚拟机` phrase.
3. Requires exactly one ASCII numeric VM ID that parses as a positive `int64`.
4. Rejects negative or conflicting phrases including `不要`, `别`, `禁止`, `无需`, `取消`, `关闭`, `关机`, `停止`, `重启`, and `重新启动`.
5. Rejects English-only input and ambiguous VM-like words such as `VMware`.
6. Sends only `Start VM <vmid>` to Needle.
7. Requires Needle to return exactly one `pve_vm_start` call whose VM ID exactly equals the ID extracted from the original Chinese input.
8. Retains all prior confidence, `ungrounded`, `negation`, tool-name, argument, authentication, and downstream safety gates.

API responses remain Chinese. This normalization does not lower the confidence threshold and does not bypass Needle tool selection.

## Timeout correction

Live controller testing also showed that go-zero's default REST timeout is 3000 milliseconds, while real Needle inference can take more than 20 seconds. The default top-level go-zero `Timeout` is therefore set to `130000` milliseconds, slightly above the configured 120-second Needle client timeout.
