```json
{"task": "KW1 board store — cards board/01, board/02, board/05", "status": "in-progress", "date": "2026-10-06", "workplan": "workplans/workplan_board_store.md", "ledger": "workplans/dependencies_kanban.md#kw1"}
```

New `board` package lands per ADR-003 pivot and the board-store workplan: Open/Create/List slice of the store contract, one card per commit. SQLite conventions mirrored from the superseded `todos` store (same `modernc.org/sqlite` driver, single pooled conn, create-if-not-exists schema on open, error wrapping style). Schema is the workplan Database section: `cards` with autoincrement id, title CHECK (non-blank, ≤500 chars — the full validation behavior is KW2's board/03+04), column CHECK over the three-value enum, position ≥0, index on (column, position). `todos` untouched. archspec spec gains the `board` module (no internal deps) and the sanctioned future api/server imports declared accordingly.
