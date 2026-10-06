# Workplan: Kanban Board Application

## Goal

A single-user web app for running one personal kanban board: a fixed set of three columns — "To Do", "In Progress", "Done" — holding cards that the user creates, edits, deletes, drags between columns, and reorders within a column by dragging. Position within a column is the user's priority; a card is done exactly when it sits in the "Done" column. Board state persists between server restarts. Observable outcome: a browser page shows the three columns with their cards in order, drag-and-drop carries every movement, and the board is unchanged after reloads and restarts. The app migrates the user's existing todo list into the board on first start.

## Acceptance Criteria

### Scenario: Board shows fixed columns
Given the app is running
When the user opens the page
Then the board shows exactly three columns in the order "To Do", "In Progress", "Done"
  And each column lists its cards top-to-bottom in stored position order

### Scenario: Create card
Given the board does not contain a card with the text "Buy milk"
When the user creates a card with the text "Buy milk"
Then the "To Do" column shows a card with the text "Buy milk" at its bottom
  And the card is not done
  And the card has a stable identifier

### Scenario: Reject empty card text
Given the app is running
When the user creates a card with empty or whitespace-only text
Then the card is not created
  And the app states that the text is required

### Scenario: Reject over-long card text
Given the app is running
When the user creates a card with text longer than 500 characters
Then the card is not created
  And the app states the character limit

### Scenario: Drag card between columns
Given a card sits in the "To Do" column
  And the "In Progress" column holds other cards
When the user drags the card onto the "In Progress" column between two of its cards
Then the card appears in "In Progress" at the drop position
  And the card is gone from "To Do"
  And the cards left in "To Do" and the cards around the drop position keep their relative order
  And the card's text and identifier are unchanged

### Scenario: Drag reorder within a column
Given a column holds at least three cards
When the user drags the top card to a position below the second card in the same column
Then the column lists the cards in the new order
  And after reloading the page the new order is still shown

### Scenario: Done is column membership
Given a card sits in the "In Progress" column
When the user drags it into the "Done" column
Then the card is shown as done
When the user drags the same card back into "To Do"
Then the card is shown as not done
  And nowhere on the page exists a separate done control for a card

### Scenario: Edit card text keeps place
Given the board contains a card with an existing text and position
When the user edits that card's text
Then the card shows the new text
  And the card stays in the same column at the same position
  And the card's identifier is unchanged

### Scenario: Delete card
Given the board contains a card
When the user deletes it
Then the board no longer shows that card
  And the cards of its column keep their relative order with no gap between positions

### Scenario: Board survives server restart
Given the board holds cards in a mix of columns and positions
When the server restarts
Then the board shows the same cards with the same texts, columns, and positions

### Scenario: Operation on missing card
Given no card exists with identifier X
When the user edits, moves, or deletes identifier X
Then the operation does not take effect
  And the app states that the card does not exist

### Scenario: Migrate existing todos on first start
Given a todo data store exists holding not-done todos [A, B, C] and done todos [D, E]
  And the kanban board has never been created
When the server starts
Then the board is created with [A, B, C] as cards in "To Do" in that top-to-bottom order
  And [D, E] as cards in "Done" in that top-to-bottom order
  And the cards carry fresh identifiers
When the server restarts again
Then the board is unchanged — the import does not run twice

### Scenario: Start without todo data
Given no todo data store exists
When the server starts
Then the board is created empty with the three fixed columns

## Decisions

- Single fixed board — Rationale: the user asked for one board to work through; a board switcher is a second product. Rejected: multiple boards. (ADR-003) — see docs/adr/ADR-003-kanban-pivot.md
- Three fixed columns "To Do" / "In Progress" / "Done" as a server-side enumeration — Rationale: the flow is the user's decision already made; column management is behavior nobody asked for. Rejected: user-added/renamed/reordered columns.
- Drag-and-drop is the sole movement mechanism, between and within columns — Rationale: explicit user choice; it makes position first-class state the user manipulates directly. Rejected: buttons or a menu as fallback, duplicating the same behavior on a second surface.
- A card carries a stored position within its column; the server keeps positions of every column contiguous (0..n-1) after each mutation — Rationale: observable rule — the order on screen is the order stored, no gaps, stable across restart. Rejected: fractional/gap positions, order-by-timestamp; both leak implementation into observable ordering.
- Done equals membership of the "Done" column — consequently the page offers no done checkbox or toggle anywhere; reopening a card means dragging it back out of Done — Rationale: one source of truth for done. Rejected: a done flag beside the column, two truths that can disagree (carried over from the todo app, whose model the pivot deletes per principle 2). (ADR-003)
- Existing todos import once, at first start, inside a single transaction — Rationale: the user's list becomes the board without a manual step; transactional import means a crash yields either a fully imported or a clean empty board, never half. Rejected: a manual migration command, reading both stores on every start.
- The board lives in its own SQLite file; the todo file stays untouched as the migration source — Rationale: clean model separation, the old file remains evidence/backup. Rejected: extending the todos database in place, mixing the superseded model into the new store. SQLite engine choice unchanged (ADR-002), stack unchanged (ADR-001).
- Card text keeps the todo app's rules: non-empty after trim, at most 500 characters — Rationale: proven rules, no reason to change. Rejected: new validation schemes.
- Editing, moving and reordering all travel through one update operation carrying text, column, position, or any combination — Rationale: one observable "change a card" contract, same shape the todo app proved. Rejected: separate endpoints per field.

> Decomposition: this workplan is decomposed into sub-workplans only after all Open Questions below are resolved.

## Assumptions

- Only one browser session edits the board at a time — Depends: the absence of concurrency control. If wrong: last write wins and a concurrent drag can be lost.
- The user's browser supports HTML5 drag-and-drop (or the drag mechanism the page uses) — Depends: DnD-only movement. If wrong: the board cannot be operated; a keyboard/button fallback would move back into scope.
- The todo data store is readable in its documented schema at migration start — Depends: the import mapping. If wrong: first start imports nothing or fails; behavior then falls back to the empty-board scenario.
- Card text is plain single-line text — Depends: validation rules and storage schema. If wrong: the data model and validation change.

## Risks

- Two tabs drag the same card concurrently — Impact: last write wins, one drop lost. Mitigation: acceptable for a single-user local app; each update addresses one card identifier and replaces only the fields it carries.
- Server crashes mid-migration — Impact: half-imported board. Mitigation: the import is a single transaction; rollback leaves no kanban schema, so the next start retries cleanly.
- A stale page drags a card that another action deleted — Impact: operation fails. Mitigation: 404 states "no such card"; a reload re-renders current truth.
- A drag drop targets an inconsistent position (stale board between tabs) — Impact: position normalizes to a valid contiguous order anyway. Mitigation: server normalization on every mutation keeps the invariant regardless of the request's guess.

## Open Questions

- None.

## API

### Endpoints
| Method | Path | Purpose |
|--------|------|---------|
| GET | /board | List the board: three fixed columns, each with its cards in position order |
| POST | /cards | Create a card (appended to bottom of "To Do") |
| PATCH | /cards/{id} | Change a card's text, column, and/or position |
| DELETE | /cards/{id} | Delete a card |

### Contracts

#### GET /board
**Success response**
- Status: 200
- Body: object with `columns`: array of exactly three entries in fixed order `todo`, `in_progress`, `done`; each entry carries `title` ("To Do", "In Progress", "Done") and `cards`: array of Card in position order (ascending position)

#### POST /cards
**Request**
- Body: `{ "title": string }`

**Success response**
- Status: 201
- Body: the created Card with `column` "todo" and `position` one greater than the previous bottom of that column

**Error responses**
- Status: 422 — Body: `{ "error": "title is required" }` when `title` is missing, empty, or whitespace-only; or when its length exceeds 500 characters (the error states the limit)

#### PATCH /cards/{id}
**Request**
- Body: `{ "title"?: string, "column"?: "todo"|"in_progress"|"done", "position"?: integer }` — at least one field; `position` is the desired index within the target column (0-based)

**Success response**
- Status: 200
- Body: the updated Card
- Note: after the update the server renormalizes the affected column(s) so their positions are contiguous 0..n-1; cards not named by the request keep their relative order

**Error responses**
- Status: 404 — Body: `{ "error": "no such card" }` when the identifier does not exist
- Status: 422 — Body: `{ "error": "title is required" }` when `title` is present but empty or whitespace-only or over the limit; `{ "error": "invalid column" }` when `column` is not one of the three; an empty body returns 422 stating that at least one field is required

#### DELETE /cards/{id}
**Success response**
- Status: 204
- Body: none
- Note: the column's remaining positions are renormalized contiguously

**Error responses**
- Status: 404 — Body: `{ "error": "no such card" }` when the identifier does not exist

### Data Models / DTOs
| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | integer | server-assigned, stable, never reused | card identifier |
| title | string | non-empty after trim, max 500 chars | the card text |
| column | string enum | one of `todo`, `in_progress`, `done` | which column holds the card |
| position | integer | ≥ 0, contiguous 0..n-1 within its column | priority order, top-to-bottom |

## Database

### Existing Data Store
SQLite file `todos.db` with table `todos(id, title, done)` — in use by the superseded todo application. Decision: keep it untouched as migration source; the board lives in a new SQLite file `kanban.db` (clean model separation; the old file stays as evidence/backup). Engine unchanged (ADR-002).

### Proposed Tables
#### cards
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | integer | primary key, auto-increment | card identifier |
| title | text | not null, non-blank | the card text |
| column | text | not null, CHECK in ('todo','in_progress','done') | column membership |
| position | integer | not null, ≥ 0 | order within the column |

Index on `(column, position)` serving the ordered read per column.

### Relationships
None — single table. Deleting a card removes its only row; positions renormalize.

### Data Flow
- POST /cards inserts at `position = count(cards in 'todo')`, assigns id, returns the Card.
- GET /board selects all rows ordered by column enum order then position ascending, groups into the three-column shape.
- PATCH /cards/{id} updates the present fields for the matching id; when column and/or position change, the server moves the card to the requested index of the target column and renormalizes that column (and the source column when it changed) to contiguous 0..n-1 in one transaction; a no-match update is 404.
- DELETE /cards/{id} removes the row and renormalizes its column; no match is 404.
- Migration (owned by the composition root at startup): if `kanban.db` has no `cards` table AND `todos.db` exists — in a single transaction create the schema and insert: not-done todos → `('todo', position by todos.id order, title)`, done todos → `('done', likewise)`; guard against re-import is the existence of the `cards` table. The todo file is never written.
- The schema must satisfy the API Card fields exactly: id, title, column, position — nothing more is stored.

## Modularity

### Behavior Analysis
- Board data and its mutation rules (create, list, change, delete, move with position normalization, persistence) — changes when the card/column model changes.
- Wire contract (HTTP paths, status codes, request/response shapes) — changes when the interface contract changes, independent of storage or page.
- Page board rendering and drag interaction (render columns, initiate/accept drags, issue the operations, show stated errors) — changes when the presentation changes.
- One-time todo import (read old store, seed board) — changes when the superseded todo store shape changes; dies after first real run.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Board data and ordering | `todos` module is superseded — model deleted, not twisted (principle 2) | board | one behavior: owns the card collection, columns, and position invariants |
| Wire contract | api (existing) | — | same role, new endpoints; hides the HTTP contract |
| Page rendering + drag | ui (existing) | — | same role; list rendering replaced by board rendering with drag mechanics |
| Startup wiring + migration | server (existing composition root) | — | composition root owns data-file lifecycle (principle 8); the import is startup work with a single owner |

### Internal Architecture
- board — flat record store with create/list/update/delete plus move/normalize rules; no domain ceremony: the domain is one record type with an invariant (contiguous per-column positions). The invariant lives in this module, not in callers.
- api — thin procedural translation: parse request, call board operation, map result to status/body.
- ui — plain procedural page: renders the board shape, wires drag events to PATCH calls, shows stated errors; the list-render components retire, the design-token/style layer survives.
- server — composition root: opens/creates kanban.db, runs the guarded import through board's own create operations reading the todo file read-only, serves both surfaces.

### Boundaries
- `todos` module loses its behavior and is deleted once the board replaces it in composition; nothing else imports it afterward.
- api depends on board through its in-process contract only.
- ui depends on the API contract over HTTP only; it never reaches board or the data files.
- Migration is the only place that reads the todo store, owned by server; board and api never see `todos.db`.
