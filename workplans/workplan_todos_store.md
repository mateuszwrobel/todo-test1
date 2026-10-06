# Workplan: Todos Module — Durable Store and Operations

> **Status: superseded** by [workplan_kanban_application.md](workplan_kanban_application.md) — see [ADR-003](../docs/adr/ADR-003-kanban-pivot.md). Kept as history.

## Goal

The todo application needs exactly one owner of todo data and its durable state. The todos module lets the api module and the composition root create, list, change, and delete todos through one in-process contract, and guarantees every completed operation survives a process restart. Observable outcome for the user (via the other modules): a todo list that is always correct after restarts.

## Acceptance Criteria

### Scenario: Create assigns fresh identifier
Given the store contains no todo with the text "Buy milk"
When Create is called with that text
Then the result is a todo with that text, done false, and an identifier never used before
  And the next List includes it

### Scenario: Create rejects invalid text
Given the store is open
When Create is called with empty, whitespace-only, or longer-than-500-character text
Then the result is an invalid-text outcome
  And no state changes

### Scenario: List is creation order
Given the store contains several todos
When List is called
Then todos are returned in ascending identifier order, oldest first

### Scenario: Change title of a not-done todo keeps done state
Given the store contains a not-done todo
When Change is called for its identifier with a new title only
Then the result is the todo with the new title and still not-done

### Scenario: Change done state keeps title
Given the store contains a todo with text "Buy milk"
When Change is called for its identifier with done false only
Then the result is the todo with text unchanged and done false

### Scenario: Change title on a done todo is refused
Given the store contains a done todo
When Change is called for its identifier with a title
Then the result is a done-frozen outcome
  And the todo's text and done state are unchanged

### Scenario: Reopen a done todo by done-only change
Given the store contains a done todo
When Change is called for its identifier with done false only
Then the result is the todo, not-done, text unchanged

### Scenario: Change missing identifier
Given no todo exists with identifier X
When Change is called for X
Then the result is a not-found outcome

### Scenario: Change with no fields is invalid
Given the store is open
When Change is called with neither title nor done supplied
Then the result is an invalid outcome
  And no state changes

### Scenario: Delete removes and identifier is never reused
Given the store contains a todo
When Delete is called for its identifier and then Create is called
Then the deleted todo never appears in List again
  And the new todo's identifier differs from the deleted one

### Scenario: Delete missing identifier
Given no todo exists with identifier X
When Delete is called for X
Then the result is a not-found outcome

### Scenario: State survives reopen
Given todos exist with mixed done states
When the store is closed and a new store instance is opened on the same file
Then List returns the same todos with the same texts and done states

### Scenario: Fresh file opens as empty valid store
Given no data file exists at the given path
When the store is opened at that path
Then the store is empty and all operations work

### Scenario: Completed operation survives process death
Given Create returns successfully
When the process is killed immediately after and restarted on the same file
Then the todo is present

## Decisions
- The module owns all validation and state rules (non-empty after trim, 500-char max, at-least-one-field change, done-text-frozen) — Rationale: one source of truth for every caller; api and ui never re-validate, and a stale page cannot outvote the server. Rejected: per-caller validation, rules drift.
- A done todo's text is frozen; only a done-only change (reopen) unlocks editing — Rationale: the parent workplan states text edits and done-state toggles as independent observable operations; the refusal must be server truth. Rejected: free edits on done todos (superseded by the parent decision), page-only enforcement.
- Outcomes are typed results (todo value | not-found | invalid | done-frozen) with no HTTP semantics — Rationale: status-code mapping is api's decision, not data's. Rejected: returning status codes here, couples storage to transport.
- SQLite in a single local file per ADR-002; the file path is supplied by the composition root at open — Rationale: the store chooses nothing about deployment location. Rejected: env reading inside the module, hidden configuration.
- Identifiers are auto-increment rowids, never reused even after delete — Rationale: satisfies the never-reuse guarantee directly. Rejected: min-free-id reuse, extra logic no behavior asks for.
- Schema is created on open (create-if-not-exists), no migration framework — Rationale: single table, greenfield; forecasting migrations violates principle 9. Rejected: migration tooling.
- A completed operation means the write is committed before the result returns; no extra concurrency machinery beyond SQLite's own guarantees — Rationale: single-user single-process app. Rejected: app-level locking, an in-memory cache.

## Assumptions
- Exactly one process opens the data file at a time — Depends: absence of multi-process coordination. If wrong: SQLite busy errors surface as operation failures.
- Text is plain single-line — Depends: the validation rule and storage. If wrong: schema and validation change.

## Risks
- Data file deleted or corrupted — Impact: todos lost. Mitigation: durable committed writes mean at most nothing is lost after a completed operation; backups out of scope (parent workplan).

## API

In-process contract (this module's whole public surface):

### Operations
| Operation | Accepts | Returns | Fails |
|-----------|---------|---------|-------|
| Open | path to data file | store handle; file created when absent | path unusable |
| Create | title | Todo (done false, fresh id) | invalid-text |
| List | nothing | todos ascending by id | — |
| Change | id, optional title, optional done (at least one) | updated Todo | not-found; invalid-title; invalid-no-fields; done-frozen |
| Delete | id | success | not-found |

### Data Models / DTOs
| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | integer | server-assigned, stable, never reused | todo identifier |
| title | string | non-empty after trim, max 500 chars | the todo text |
| done | boolean | required | done state |

Error cases are exactly: not-found, invalid-text, invalid-no-fields, done-frozen. No other failure mode is part of the contract.

## Database

### Existing Data Store
None — greenfield; this module introduces it: embedded SQLite, single local file (ADR-002). This module is the sole owner of the file; no other module opens it.

### Proposed Tables
#### todos
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | integer | primary key, autoincrement | identifier and creation-order key |
| title | text | not null, non-blank | the todo text |
| done | integer (boolean) | not null, default 0 | done state |

### Relationships
Single table; no relationships.

### Data Flow
Create inserts and returns the created row; List selects all ordered by id; Change updates only supplied columns for the matching id — a title on a done row is refused before any write, affected-row count distinguishes not-found; Delete removes by id. The row shape satisfies the API-model Todo exactly — nothing more is stored.

## Modularity

### Behavior Analysis
- Owning the todo collection's durable state and the rules that keep it valid — the single reason this module changes.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Durable todo state + validation/state rules | none (greenfield) | todos | hides one decision: how todo state is stored and kept valid |

### Internal Architecture
Flat CRUD over one table. No domain model ceremony — the domain is one record type with three fields.

### Boundaries
- New module; depends on nothing internal, only the SQLite driver dependency (ADR-002).
- Owns the data file lifecycle; other modules receive operations through the store handle (composition root wires).
- api consumes this module's contract through a minimal interface api itself declares (consumer-defined port); the concrete store is injected — no module depends on another's concrete type.
