# Absorb scenario-card and dependency-ledger conventions into skills

```json
{
  "task": "absorb scenario-card, verbatim-split and dependency-ledger conventions into skills",
  "status": "done",
  "date": "2026-10-04"
}
```

The decomposition session invented scenario cards (`workplans/scenarios/`, 47 cards) and a dependency ledger (`workplans/dependencies.md`) ad-hoc; this makes them durable skill conventions so future parallel work starts from the format instead of reinventing it. The `tdd-workplan` skill gained a "Decomposing for parallel implementation" section covering card format, the verbatim-split proof rule, the ledger's contract-vs-code edges and integration checkpoints, and parent-scenario traceability. The `modular-planner` skill gained Step 8 pointing at both artifacts. Ordering stays out of workplans — workplans keep exactly the six core sections plus partials — it lives in the ledger only, and cards stay behavior-only per the unchanged behavior/implementation dividing line.
