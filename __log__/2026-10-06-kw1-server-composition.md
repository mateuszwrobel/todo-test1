```json
{"task": "KW1 server composition — cards server/01, server/03, server/08", "status": "in-progress", "date": "2026-10-06", "workplan": "workplans/workplan_server_board_composition.md", "ledger": "workplans/dependencies_kanban.md#kw1"}
```

Server-composition lane of KW1: the stale `cmd/todo` tests left red by the api/01 retirement of `GET /todos` (start-serves test, restart test, sqlite import pin) are rewritten onto the board contract — one commit per scenario card (server/01, server/03, server/08). The harness grows a `--board-db` path so spawns never write `kanban.db` into the checkout, the dependency pin becomes per-owner data-file opening (board; todos transitional until KW4), and the todo-flavored restart assertion becomes the minimal board restart that is real today. Plus one ledger cell fix: the KW1 server/01 note claimed the todos package deleted at wave end — reality is retirement with the last endpoint (KW4).
