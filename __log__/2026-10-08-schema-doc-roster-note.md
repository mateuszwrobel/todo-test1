```json
{"task": "schema-doc roster note — cmd/db-diagram emits a mermaid note on the assignee column naming the simulated cast from users.Names(), so docs/db-schema.md says who an assignee is; generated doc regenerated", "status": "done", "date": "2026-10-08", "base": "17e3908"}
```

What changed and why:

- **cmd/db-diagram:** the generator renders each table's notes; an assignee
  column now adds one more note line after that table's other notes:
  `note for cards "assignee: one of the built-in simulated users (users
  module, not stored): Ada, Grace, Alan, Barbara, Linus"`. Who an assignee is
  cannot be introspected — the cast is users-module data, never stored — so
  the note's names are read from `users.Names()` at generation time and joined
  verbatim; the generator types no roster name. The line rides the table whose
  introspected columns include an assignee (found via the same case-folded
  column lookup the rest of the renderer uses), so it stays schema-driven
  rather than pinned to a table-name string, and it keeps the exact
  `note for <table> "<text>"` shape the index/check notes already emit —
  parentheses and colons inside the quoted string are what those lines already
  carry, so the mermaid block stays grammatically unchanged (mermaid CLI still
  not installed; argued from the grammar shape, the docs-canonical precedent,
  same caveat as the original diagram lane).
- **cmd/db-diagram test:** none existed; added `main_test.go` — drives `run`
  end-to-end against the live schema (board.Open, like `make db-diagram`) and
  asserts the note line once, byte-quoted, built as a join over
  `users.Names()` (the test spells no roster name either, so a roster change
  moves expectation and doc together), and that it sits above no other cards
  note — i.e. it lands after the cards index/check notes.
- **architecture.spec.toml:** declared the db-diagram→users edge
  (`depend_on = ["board", "users"]`), per the spec-first convention of
  declaring an edge at the wave that first imports it. `verify --strict` green
  over 6 modules.
- **docs/db-schema.md:** regenerated via `make db-diagram`; diff is exactly
  the one added note line. Determinism re-checked: back-to-back generation is
  byte-identical (`go run ./cmd/db-diagram -out tmp && cmp` green), so the
  pre-commit diagram gate passes.
- **Verification:** gofmt/vet clean; `go test -count=1 ./...` fully green
  (cmd/todo and tooling included); archspec `verify --strict` ok.
