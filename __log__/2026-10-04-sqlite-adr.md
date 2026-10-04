```json
{
  "task": "record SQLite database decision",
  "status": "done",
  "date": "2026-10-04"
}
```

Pinned the workplan's deferred engine choice to embedded SQLite in a single local file (ADR-002): it satisfies the relational single-table store with typed columns and autoincrement id directly and with zero operational surface, unlike an external DB server (operational burden for a single-user local app), a JSON/in-memory store (would reinvent schema and queries, or fails restart persistence), or an embedded KV (not relational); workplan Decisions bullet and Database "Existing Data Store" paragraph updated to point at the ADR.
