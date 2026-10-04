# Workplan save-path convention → workplans/ folder

```json
{
  "status": "done",
  "links": {
    "task": "workplan save-path convention → workplans/ folder",
    "example": "workplans/workplan_todo_application.md @ 309ac60"
  }
}
```

Updated the tdd-workplan skill's save-path line so future plans land in `workplans/` instead of the repo root. Why: the first real workplan already moved there at 309ac60, and the skill text still instructed root-level saves — the convention and the example disagreed. Grep across `.agents/` and `AGENTS.md` found this as the only save-path reference, so no other files needed the change.
