# Workplan: Api Module — HTTP Contract

> **Status: superseded** by [workplan_kanban_application.md](workplan_kanban_application.md) — see [ADR-003](../docs/adr/ADR-003-kanban-pivot.md). Kept as history.

## Goal

The application needs one stable JSON wire contract for todo operations. The api module exposes the endpoints and status semantics of the parent workplan and translates them into todos-module operations, mapping typed outcomes to HTTP responses. Observable outcome: any client can create, list, change, and delete todos over HTTP exactly as the parent contract states, including the stated refusals (done-todo text edits, missing todos, invalid titles).

## Acceptance Criteria

### Scenario: Create succeeds
Given the service is running
When a POST /todos with body `{ "title": "Buy milk" }` arrives
Then the response is 201 with the created todo's JSON, done false and a fresh id

### Scenario: Create with blank title
When a POST /todos with empty or whitespace-only title arrives
Then the response is 422 with `{ "error": "title is required" }`
  And no todo is created

### Scenario: Create over length limit
When a POST /todos with a title longer than 500 characters arrives
Then the response is 422 stating the 500-character limit
  And no todo is created

### Scenario: List todos
Given todos exist
When a GET /todos arrives
Then the response is 200 with the JSON array of todos ordered by id ascending

### Scenario: Change text of a not-done todo
When a PATCH /todos/{id} with `{ "title": "new text" }` arrives for an existing not-done todo
Then the response is 200 with the updated todo JSON and done state unchanged

### Scenario: Change done state in either direction
When a PATCH /todos/{id} with `{ "done": true }` or `{ "done": false }` arrives
Then the response is 200 with the updated todo JSON and the title unchanged

### Scenario: Title edit on done todo refused
When a PATCH /todos/{id} carrying a title arrives for a done todo
Then the response is 422 with `{ "error": "cannot edit a done todo" }`
  And the todo is unchanged

### Scenario: Change missing todo
Given no todo exists with identifier X
When a PATCH /todos/X arrives
Then the response is 404 with `{ "error": "no such todo" }`

### Scenario: Change with empty body
When a PATCH /todos/{id} with an empty JSON object arrives
Then the response is 422 stating that at least one field is required

### Scenario: Delete
When a DELETE /todos/{id} arrives for an existing todo
Then the response is 204 with no body
  And a later GET does not include it

### Scenario: Delete missing todo
Given no todo exists with identifier X
When a DELETE /todos/X arrives
Then the response is 404 with `{ "error": "no such todo" }`

## Decisions
- Outcome mapping is fixed here: invalid-text → 422 "title is required" (or the limit message), invalid-no-fields → 422, done-frozen → 422 "cannot edit a done todo", not-found → 404, create success → 201, delete success → 204, change success → 200 — Rationale: the parent contract names these codes and messages; this module is where data outcomes become transport. Rejected: mapping in todos (couples storage to transport), mapping in ui (duplicates the rule).
- The module exposes an http.Handler for the /todos subtree and owns no listener or routing outside it — Rationale: listener and mounting are composition-root decisions. Rejected: module starting its own server.
- No re-validation: typed outcomes from the todos contract drive responses — Rationale: single source of truth (todos workplan). Rejected: defensive duplicate checks; the frozen-text rule lives in the store, its refusal arrives as an outcome.
- Error bodies are exactly `{ "error": message }` — Rationale: one error shape, matching the parent contract. Rejected: error-code objects, forecasting.
- No CORS handling — Rationale: same-origin browser page served by the same listener; no cross-origin consumer exists. Rejected: configurable CORS, forecasting.
- Malformed JSON or non-numeric ids → 400 with `{ "error": "invalid request" }` — Rationale: unparseable input is distinct from invalid-but-parseable values; one blanket code, no per-field guesswork. Rejected: 422 reuse, conflates transport errors with rule refusals.

## Assumptions
- The todos contract satisfies all validation, freezing, and ordering rules — Depends: this module staying a pure translator. If wrong: rules would need reimplementation here, which the boundaries forbid.

## Risks
- Contract drift between this module and the ui module's expectations — Impact: the page breaks silently. Mitigation: one contract owner (the parent workplan's endpoint tables); e2e exercises the pair.

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
- Status: 422 — `{ "error": "title is required" }` when title is missing, empty, or whitespace-only; the 500-character limit is stated in the error when exceeded
- Status: 400 — `{ "error": "invalid request" }` for malformed JSON

#### PATCH /todos/{id}
**Request**
- Body: `{ "title"?: string, "done"?: boolean }` — at least one field

**Success response**
- Status: 200
- Body: the updated Todo

**Error responses**
- Status: 404 — `{ "error": "no such todo" }` when the identifier does not exist
- Status: 422 — `{ "error": "cannot edit a done todo" }` when the todo is done and the request carries a title
- Status: 422 — `{ "error": "title is required" }` when title is present but empty or whitespace-only; an empty body states that at least one field is required
- Status: 400 — `{ "error": "invalid request" }` for malformed JSON or a non-numeric id

#### DELETE /todos/{id}
**Success response**
- Status: 204
- Body: none

**Error responses**
- Status: 404 — `{ "error": "no such todo" }` when the identifier does not exist
- Status: 400 — `{ "error": "invalid request" }` for a non-numeric id

### Data Models / DTOs
| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | integer | server-assigned, stable, never reused | todo identifier |
| title | string | non-empty after trim, max 500 chars | the todo text |
| done | boolean | required | done state |

## Modularity

### Behavior Analysis
- Translating HTTP requests into todo operations and typed outcomes into transport responses — the single reason this module changes.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Wire contract and status semantics | none (greenfield) | api | hides one decision: how todo operations travel over HTTP |

### Internal Architecture
Thin procedural translation layer: parse, call the injected contract, map outcome to status/body. No state, no goroutines, no framework.

### Boundaries
- Depends on the todos contract through an interface this module declares (consumer-defined port); the concrete store is injected by the composition root — no concrete cross-module types (principle 5).
- Never renders HTML, never touches the data file, never owns the listener.
- ui consumes this contract over HTTP at runtime, never in-process (architecture.spec.toml forbids a ui→api or ui→todos import).
