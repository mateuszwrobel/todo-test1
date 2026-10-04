```json
{
  "task": "user journeys document for UI design",
  "status": "done",
  "date": "2026-10-04"
}
```

User journeys authored from `workplans/workplan_todo_application.md` as input for UI design.
- added `workplans/user-journeys.md`: six journeys (browse, create, toggle done, edit, delete, stale-id operation), each with goal, entry/exit states, observable main/alternate flows, API calls, and behavior-level UI hooks — no behavior beyond the workplan's 7 acceptance scenarios and API contract; page-necessity hooks (empty list, load failure) annotated as derived, not scenarios.
- added cross-cutting UI-states table and journey→scenario→endpoint traceability so UI design can prove full scenario coverage and no scope creep; due dates/undo/confirm-delete etc. stay out, surfaced only as open UI questions.
- J6 notes the single-user last-write-wins edge once, citing workplan Assumptions/Risks.
