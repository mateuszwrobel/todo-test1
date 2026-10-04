```json
{
  "task": "re-cut dependency ledger into feature waves",
  "status": "done",
  "date": "2026-10-04"
}
```

Replaced `workplans/dependencies.md` wholesale with the feature-wave ledger. The old module-lane model — per-module lanes where each module lands whole, consumers coding against contract fakes until an integration checkpoint retires them — is superseded. Ordering now lives in 8 feature waves (W1 foundation+browse through W8 in-flight serialization): each wave delivers one feature vertically through store → contract translation → page, and the wave end is that feature's integration against the real code landed earlier in the same wave, accepted by re-executing the named parent scenarios. No fake-then-replace staging survives the re-cut.

The standing law recorded in the ledger: **a module is never implemented wholesale.** Every increment is a functional/common-scenario slice — the thinnest cut through the modules that makes one feature observable end to end. A module grows one feature per wave it appears in; whole-module waves are forbidden. This retires the checkpoint-based integration story (old CP1..CPn entries): integration is now a wave-end property, never an in-flight card requirement, so a card still never claims an integration result it could not observe.

All 47 scenario cards are assigned to exactly one wave with concrete per-card dependencies (script-verified before commit: todos 14, api 11, ui 16, server 6 — 47 rows, none missing, none duplicated; dependency references inside rows are edges, not assignments). The wave-dependency graph keeps parallelism explicit — after W1 the widest spread is {W2, W3∥W5, W7} — and the lane rules carry over the git discipline: one lane per wave-branch, separate worktrees, orchestrator ff-only merges in wave order.

Source for the replacement: `/tmp/opencode/waves-deps.md`, copied with `cp` and confirmed byte-identical with `cmp`. Cards under `workplans/scenarios/` are untouched — they stay behavior-only; the ledger remains the single owner of all ordering.
