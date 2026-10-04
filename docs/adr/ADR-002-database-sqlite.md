# ADR-002: Database — embedded SQLite

## Status

Accepted — 2026-10-04

## Context

The workplan (`workplans/workplan_todo_application.md`) fixed the storage *shape* — one
embedded local relational-style store in a single data file, one `todos` table with typed
columns and an auto-increment id, durable writes per operation, no external server — but
deliberately left the engine choice deferred. The app is a single-user local web app served
by one Go binary (ADR-001); the restart-persistence scenario requires durable state on the
local disk. The decision now pins the engine so implementation has no open choice to make.

## Decision

- **Embedded SQLite database in a single local data file.** The server process opens one
  file; no DB server, no separate deploy target, no network hop.
- **Schema: the workplan's `todos` table** — `id` integer primary key auto-increment,
  `title` text not null non-blank, `done` integer (boolean) not null default 0. Nothing
  else is stored.
- **Durable write per operation.** Every create/change/delete commits before the operation
  reports success, so nothing is lost after a completed operation.
- **No ORM ceremony.** Plain SQL against the store directly; the `todos` module is flat CRUD.
- **Implementation note, not a constraint:** driver selection (e.g. a cgo-free pure-Go
  driver) is left to the implementation, as long as the store stays single-file embedded
  SQLite.

## Consequences

- The restart-persistence scenario is satisfied by one file on local disk; the composition
  root owns its open/creation (one owner, principle 8).
- Zero operational surface: nothing to install, run, or administer besides the app binary.
- The relational schema maps 1:1 onto the API Todo fields, and the workplan's Data Flow
  queries (insert, select ordered by id, column-level update, delete) are direct SQL.
- The driver choice stays swappable behind the `todos` module boundary without changing any
  observable contract.
- SQLite's file locking comfortably covers the single-writer assumption in the workplan.

## Alternatives

- **External DB server (Postgres/MySQL)** — rejected: operational burden; a single-user
  local app gains nothing from a server process, ports, or administration.
- **In-memory or JSON-file store** — rejected: the workplan's database partial specifies a
  relational-style single table with typed columns and an auto-increment id; SQLite
  satisfies that directly, a JSON store would reinvent schema and querying. In-memory also
  fails the restart-persistence scenario outright.
- **Embedded KV (bbolt et al)** — rejected: not relational; the schema, id ordering, and
  per-column updates would all be hand-rebuilt on a byte store.
