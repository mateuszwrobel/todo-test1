```json
{"task": "kanban pivot planning artifacts", "status": "done", "date": "2026-10-06"}
```

- ADR-003 accepted: pivot to a single-board kanban — three fixed columns, cards with
  persistent per-column positions, done as Done-column membership, drag-and-drop as the
  sole movement mechanism, one-time migration of existing todos; consequences cover the
  deleted todo store model, board rendering replacing list rendering, replaced api
  contract, and migration owned by the composition root.
- Todo workplan family (parent, four sub-workplans, user journeys, dependency ledger)
  marked superseded with a blockquote under each H1; kept in the repo as history.
- Parent kanban workplan follows in the next commit.
- Parent kanban workplan authored from the draft; todo family superseded.
