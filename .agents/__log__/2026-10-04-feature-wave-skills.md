```json
{
  "task": "skills teach feature-wave ordering, module-lane model superseded",
  "status": "done",
  "date": "2026-10-04"
}
```

The skills previously taught the module-lane model: contract-vs-code edges, contract fakes retired at integration checkpoints `CP1..CPn`, and per-module waves in the dependency ledger. The user ruled that ordering proceeds by feature waves — vertical slices of one feature through the modules — because a module is never implemented wholesale; the ledger re-cut at c8f7ae1 landed that model first. The `tdd-workplan` and `modular-planner` skills now teach the wave model matching the landed ledger: whole-module waves forbidden, per-card dependencies inside each wave, integration as the wave end against real code with no fake-then-replace staging. Scenario-card conventions, the verbatim-split rule, and parent-scenario traceability are unchanged.
