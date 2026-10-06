```json
{"task": "KW4 ui lane — card ui/07 (delete drops one card)", "status": "in-progress", "date": "2026-10-06", "workplan": "workplans/workplan_ui_board.md", "ledger": "workplans/dependencies_kanban.md#kw4"}
```

What this lane changes and why. (Prose completed at hand-back.)

Wiring the delete affordance: the inert `.card__delete` placeholder on every card
becomes a real htmx `hx-delete` control; new `DELETE /ui/cards/{id}` fragment route
on the ui mux mirrors the edit pattern; 404 reuses edit's stale arm (banner + truth).
Card text carries no confirmation clause — no confirm dialog added.
