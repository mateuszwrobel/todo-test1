```json
{"task": "db-diagram generator — cmd/db-diagram renders docs/db-schema.md (Mermaid erDiagram) from the live board schema via board.Open + PRAGMA introspection; make db-diagram target; committed diagram", "status": "done", "date": "2026-10-07", "base": "00ed598"}
```

What changed and why:

- **cmd/db-diagram** (new dev-tool module, package main): materializes the
  current schema by `board.Open` on a throwaway temp file — board stays the
  schema's single owner, the tool copies no SQL — closes that handle, and
  reopens the file read-side for introspection. It names the `"sqlite"`
  driver without importing it: registration rides the transitive board import,
  keeping board the driver's one home (server/08 pin; the pin's static import
  scan stays green). Facts come from sqlite_master plus the table_info,
  index_list, index_info and foreign_key_list pragmas. The two facts pragmas
  cannot report per column — CHECK expressions and the AUTOINCREMENT keyword —
  are read from the stored DDL in sqlite_master by a small depth/string-aware
  scanner, not by restating the schema.
- **Rendering:** Mermaid `erDiagram`; attribute key slots PK/FK/UK derive from
  introspection (PK from table_info, FK from the child side of a foreign key,
  UK from a unique index beyond the primary key's implicit one); the quoted
  attribute comment carries not null / autoincrement / DEFAULT / check
  verbatim from pragma or DDL text, with SQLite's reserved-word double quotes
  scrubbed (mermaid comments cannot nest them). Explicit CREATE INDEX gets a
  `note for <table>` line; constraint-implicit indexes deliberately do not —
  they speak through the UK slot instead. Relation arrows render only when
  foreign_key_list yields rows (none today → zero relations today); the FK
  path was exercised out-of-repo against a scratch schema (references,
  composite unique, table-level CHECK) and proved: one harness iteration
  caught a real bug there — foreign_key_list answers 8 columns, not 5, which
  only surfaces once an FK row exists — fixed before commit. Output ordering
  is by table name / declaration order / pragma order, so regeneration is
  byte-stable; verified byte-identical across back-to-back runs.
- **docs/db-schema.md:** committed generated output — one H1, one fenced
  mermaid block, nothing else. cards: id PK "autoincrement", title/column/
  position with their CHECK comments; meta: key PK, value "not null"; one
  index note; zero relations. No prose, no legacy todos mention.
- **Makefile:** standalone `db-diagram` target (`go run ./cmd/db-diagram`),
  not wired into any other target or hook.
- **architecture.spec.toml:** declared the new `db-diagram` module (units
  `*/cmd/db-diagram`, depend_on board alone, forbidden api/ui/server) and
  added it to no_cycles — archspec `verify --strict` green at 5 modules, and
  green through cmd/todo's TestArchspecGateGreen as part of `go test ./...`.
- **Verification:** go build + go vet clean; `go test -count=1 ./...` fully
  green (no app behavior touched); make target idempotent byte-for-byte.
  Mermaid CLI not installed locally — render validity argued from the grammar
  shape (docs-canonical `type name PK "comment"` / `note for` lines), not
  machine-checked; nothing was added as a dependency for it.
