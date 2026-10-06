# Workplan: Board API (HTTP Contract)

> Parent: [workplan_kanban_application.md](workplan_kanban_application.md) — this module's scenarios are the contract view of the parent's behaviors.

## Goal

The HTTP surface of the kanban board: one read endpoint for the whole board and three mutations for cards, exactly as the parent contract states. The module translates requests into board operations and outcomes into status codes and stated error bodies — no business rule lives here. Observable outcome: any HTTP client can drive the entire board through four endpoints with stable shapes and stated failures.

## Acceptance Criteria

### Scenario: Board read shape
Given a board holding cards in all three columns
When a client sends GET /board
Then the response is 200
  And the body carries columns in the fixed order todo, in_progress, done with their display titles
  And each column's cards array is in position order with id, title, column, position

### Scenario: Create returns the card
Given an open board
When a client posts {"title": "Buy milk"} to /cards
Then the response is 201
  And the body is the created card with column "todo", the bottom position, and a fresh id

### Scenario: Create blank title is a stated 422
Given an open board
When a client posts an empty or whitespace-only title to /cards
Then the response is 422 with the error "title is required"
  And the board is unchanged

### Scenario: Create over-limit is a stated 422
Given an open board
When a client posts a title longer than 500 characters to /cards
Then the response is 422 stating the character limit
  And the board is unchanged

### Scenario: Patch title returns the card
Given a card exists with identifier N
When a client patches {"title": "new text"} to /cards/N
Then the response is 200
  And the body is the card with the new title, same column, same position, same id

### Scenario: Patch move returns the moved card
Given a card exists with identifier N in the todo column
When a client patches {"column": "in_progress", "position": 1} to /cards/N
Then the response is 200
  And the body is the card with column "in_progress" and position 1

### Scenario: Patch unknown id is a stated 404
Given no card exists with identifier Z
When a client patches any fields to /cards/Z
Then the response is 404 with the error "no such card"

### Scenario: Patch invalid column is a stated 422
Given a card exists with identifier N
When a client patches {"column": "someday"} to /cards/N
Then the response is 422 with the error "invalid column"
  And the card is unchanged

### Scenario: Patch empty body is a stated 422
Given a card exists with identifier N
When a client patches an empty JSON object to /cards/N
Then the response is 422 stating that at least one field is required
  And the card is unchanged

### Scenario: Delete answers 204
Given a card exists with identifier N
When a client sends DELETE /cards/N
Then the response is 204 with no body
  And a later GET /board omits the card

### Scenario: Delete unknown id is a stated 404
Given no card exists with identifier Z
When a client sends DELETE /cards/Z
Then the response is 404 with the error "no such card"

## Decisions

- Board operations map 1:1 onto the parent contract — Rationale: the parent workplan's API section is the contract; this module exists to serve it, not to revise it.
- Outcome-to-status mapping table in one place (ok→200/201/204, no-such-card→404, invalid-text→422 required-or-limit message, invalid-column→422, empty-change→422) — Rationale: one mapping site; the board module's outcomes are the only inputs (principle 4: depend on the contract). Rejected: per-handler ad-hoc codes.
- Request validation is shape-only (known fields, JSON well-formed); semantic validation stays in the board module — Rationale: one rule source. Rejected: duplicating title rules here.
- The existing api module survives with replaced endpoints — Rationale: same hidden decision (the HTTP contract), new content (principle 3). The todos endpoints retire when this lands.

## Assumptions

- The board module's contract (operations + outcomes) exists as specified in [workplan_board_store.md](workplan_board_store.md) — Depends: composition order; the api grows against it per the wave ledger. If wrong: the mapping layer has nothing to map.
- JSON encoding of the parent's Card model is the only cross-boundary format — Depends: parent API section. If wrong: contracts drift.

## Risks

- A board outcome appears that the mapping table does not cover — Impact: unmapped failure surfaces as a generic error. Mitigation: the mapping covers the full outcome enumeration; a new outcome forces a mapping entry at review time.

## Open Questions

None.

## API

### Endpoints
| Method | Path | Purpose |
|--------|------|---------|
| GET | /board | Three fixed columns, each with cards in position order |
| POST | /cards | Create a card (appended to bottom of "To Do") |
| PATCH | /cards/{id} | Change a card's text, column, and/or position (at least one field) |
| DELETE | /cards/{id} | Delete a card |

Contracts, request/response bodies, and error strings are exactly the parent workplan's API section (single source — this module implements that contract verbatim).

### Data Models / DTOs
The Card model of the parent contract: id (int, stable, never reused), title (non-blank ≤500), column (todo|in_progress|done), position (0-based, contiguous per column).

## Modularity

### Behavior Analysis
- The HTTP wire contract — changes when the interface contract changes, independent of storage or page.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| HTTP contract | api (existing) | — | same hidden design decision (the wire contract), new endpoints |

### Internal Architecture
Thin procedural translation: parse → call one board operation → map outcome to status/body. No domain logic (principle 6 — adding any would split cohesion).

### Boundaries
- api depends on board's in-process contract only; it never touches the database or file paths.
- ui consumes these endpoints over HTTP only; nothing else calls api in-process.
