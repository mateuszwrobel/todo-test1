```json
{
  "task": "journeys: in-flight control blocking decision",
  "status": "done",
  "date": "2026-10-04"
}
```

UI decision settled during UI design work, recorded in `workplans/user-journeys.md` only — workplan untouched.
- Added "Interaction decision — in-flight control blocking" section after Actor: the control that triggers an operation (Add button, per-row toggle, row Save, Delete) is disabled from request until response; create input/button return to ready per J2's ready-state rule. Consequence stated: the page never issues a second mutation for an operation already in flight from that page. Hook bullets added to J2/J3/J4/J5 and a "Control in-flight" row to the cross-cutting states table.
- Why: user decision closing the same-tab double-click race (e.g. Delete clicked twice → second request hits 404). The page serializes its own operations per control; no app behavior changes — one mutation per page action stays fully inside the existing contract.
- J6 reworded accordingly: same-page double-submit is closed by the decision; reachable staleness paths narrowed to a second tab (per-page snapshots), server restart against a different/emptied data file, and direct data-file edits. 404 handling requirements unchanged; open questions checked — none contradict, all kept verbatim.
