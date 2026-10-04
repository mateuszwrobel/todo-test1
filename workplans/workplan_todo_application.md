# Workplan: Todo Application

## Goal

A single-user web app for managing a personal todo list: create todos, mark them done, edit their text, delete them. Todos persist between server restarts. Observable outcome: a browser page shows the list with each todo's done/not-done state and offers controls for all four operations; the state survives reloads and restarts.

## Acceptance Criteria

### Scenario: Create todo
Given the user's todo list does not contain "Buy milk"
When the user creates a todo with the text "Buy milk"
Then the list shows a todo with the text "Buy milk"
  And the todo is marked not-done
  And the todo has a stable identifier

### Scenario: Reject empty todo text
Given the app is running
When the user creates a todo with empty or whitespace-only text
Then the todo is not created
  And the app states that the text is required

### Scenario: Mark todo done
Given the list contains a not-done todo
When the user marks it done
Then the list shows that todo as done
  And the todo's text is unchanged

### Scenario: Edit todo text keeps done state
Given the list contains a done todo
When the user edits that todo's text
Then the list shows the new text
  And the todo remains marked done

### Scenario: Delete todo
Given the list contains a todo
When the user deletes it
Then the list no longer shows that todo

### Scenario: Todos survive server restart
Given the list contains todos with a mix of done and not-done states
When the server restarts
Then the list shows the same todos with the same texts and done states

### Scenario: Operation on missing todo
Given no todo exists with identifier X
When the user edits, marks done, or deletes identifier X
Then the operation does not take effect
  And the app states that the todo does not exist

### Scenario: Repeat activation while an operation is in flight
Given the list contains a todo
When the user activates the same row control twice before the first response arrives
Then no additional effect occurs beyond the single operation
  And the list ends in the state produced by exactly one operation

## Decisions

- Single user, no authentication — Rationale: explicitly chosen scope; the app runs locally. Rejected: multi-user accounts, out of scope.
- Web app (browser page + local server) — Rationale: explicitly chosen surface. Rejected: CLI, user picked the browser.
- One local embedded data file; no external database server — Rationale: single-user app, a DB server is an operational burden with no benefit here. Rejected: external DB server; in-memory only (fails the restart-persistence scenario).
- One update operation carries text change, done-state change, or both — Rationale: a single observable "change a todo" contract; the done flag is one field of the todo, not a separate behavior needing its own contract. Rejected: separate contracts per field, duplicate the same behavior.
- Marking done and marking not-done are both plain updates of the done state — Rationale: the update contract already carries the flag; reopening costs no extra surface. Rejected: one-way done-only flow, would forbid a user-visible correction with no benefit.
- The list is ordered by creation order, oldest first — Rationale: stable order the user can rely on after restart; identifier ordering is the natural read of "creation order". Rejected: done-last or mutable ordering, adds drag/reorder behavior nobody asked for.
- No due dates, priorities, projects, or tags — Rationale: build only requested behavior; forecasting abstractions is forbidden by modular design principle 9. Rejected: adding them now.
- The server also serves the browser page — Rationale: one process to start, no separate deploy target for the page. Rejected: separately served frontend, unnecessary boundary for a single-user local app.
- Implementation stack: Go only, htmx for page interactivity, Playwright for integration/e2e, archspec as architecture-test gate — Rationale: user decision; recorded in ADR-001 — see docs/adr/ADR-001-stack-go-htmx-playwright-archspec.md.
- Database engine: embedded SQLite, single local data file — Rationale: satisfies the relational single-table store with zero operational surface; recorded in ADR-002 — see docs/adr/ADR-002-database-sqlite.md.
- Controls for an operation stay disabled from request until response — Rationale: a page never issues a duplicate mutation for an action already in flight; the same-tab double-submit race is closed at the source. Rejected: server-side duplicate suppression, a repeated legitimate request is indistinguishable from a stray double-click.

> Decomposition: this workplan is decomposed into sub-workplans only after all Open Questions below are resolved.

## Assumptions

- Only one browser session edits the list at a time — Depends: the absence of concurrency control. If wrong: last write wins and a concurrent edit can be lost.
- Todo text is plain single-line text — Depends: the validation rule (non-empty after trim) and the storage schema. If wrong: rich or multi-line text changes the data model and validation.
- The server runs on the user's local machine — Depends: running without authentication. If wrong: authentication moves into scope.

## Risks

- A second browser tab edits the same todo concurrently — Impact: last write wins, one edit silently lost. Mitigation: acceptable for a single-user local app; each update addresses one identifier and replaces only the fields it carries.
- The data file is corrupted or deleted — Impact: all todos are lost. Mitigation: every operation commits durably so at most nothing is lost after a completed operation; backups are out of scope.

## Open Questions

- None.

## API

### Endpoints
| Method | Path | Purpose |
|--------|------|---------|
| GET | /todos | List all todos, creation order (oldest first) |
| POST | /todos | Create a todo |
| PATCH | /todos/{id} | Change a todo's text and/or done state |
| DELETE | /todos/{id} | Delete a todo |

### Contracts

#### GET /todos
**Success response**
- Status: 200
- Body: array of Todo, ordered by identifier ascending

#### POST /todos
**Request**
- Body: `{ "title": string }`

**Success response**
- Status: 201
- Body: the created Todo with `done` false

**Error responses**
- Status: 422 — Body: `{ "error": "title is required" }` when `title` is missing, empty, or whitespace-only; or when its length exceeds 500 characters (the error states the limit)

#### PATCH /todos/{id}
**Request**
- Body: `{ "title"?: string, "done"?: boolean }` — at least one field

**Success response**
- Status: 200
- Body: the updated Todo

**Error responses**
- Status: 404 — Body: `{ "error": "no such todo" }` when the identifier does not exist
- Status: 422 — Body: `{ "error": "title is required" }` when `title` is present but empty or whitespace-only; when an empty body is sent, the same 422 states that at least one field is required

#### DELETE /todos/{id}
**Success response**
- Status: 204
- Body: none

**Error responses**
- Status: 404 — Body: `{ "error": "no such todo" }` when the identifier does not exist

### Data Models / DTOs
| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | integer | server-assigned, stable, never reused | todo identifier |
| title | string | non-empty after trim, max 500 chars | the todo text |
| done | boolean | required | done/not-done state |

## Database

### Existing Data Store
None — greenfield. Decision: embedded SQLite database in a single local data file; no external server — recorded in ADR-002 — see docs/adr/ADR-002-database-sqlite.md.

### Proposed Tables
#### todos
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | integer | primary key, auto-increment | identifier; ordering key for creation order |
| title | text | not null, non-blank | the todo text |
| done | integer (boolean) | not null, default 0 | done state |

### Relationships
None — single table. Deleting a todo removes its only row.

### Data Flow
- POST /todos inserts a row (title only), assigns id, returns the created Todo.
- GET /todos selects all rows ordered by id ascending, maps rows to Todo.
- PATCH /todos/{id} updates only the columns present in the request for the matching id; a no-match update is reported as 404.
- DELETE /todos/{id} removes the row matching id; no match reports 404.
- The schema must satisfy the API Todo fields exactly: id, title, done — nothing more is stored.

## Modularity

### Behavior Analysis
- Todo data and its mutation rules (create, list, change, delete, persistence) — changes when the todo model changes.
- Wire contract (HTTP paths, status codes, request/response shapes) — changes when the interface contract changes, independent of storage or page.
- Page interaction (render the list, issue the operations, block the triggered control while its operation is in flight, show error messages) — changes when the user-facing presentation changes.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Todo data and mutation | none (greenfield) | todos | one behavior: owns the todo collection and its durable state |
| Wire contract | none (greenfield) | api | hides the HTTP contract; translates requests into todo operations |
| Page interaction | none (greenfield) | ui | hides the browser-side rendering and controls |

### Internal Architecture
- todos — flat CRUD: a record store with create/list/update/delete operations and durable writes. No domain model ceremony; the domain is one record type.
- api — thin procedural translation layer: parse request, call todo operation, map result to status/body.
- ui — plain procedural page: renders the current list, sends the four operations to the contract, shows the stated errors. No framework ceremony.

### Boundaries
- All modules are new; no existing module gains behavior; nothing must be extracted first.
- api depends on todos through its in-process contract (create/list/update/delete) only.
- ui depends on the API contract over HTTP only; it never reaches the todos module or the data file.
- The server startup (composition root) owns the store's lifecycle — one owner for the data file's open/creation (principle 8).
