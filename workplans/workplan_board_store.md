# Workplan: Board Store Module

> Parent: [workplan_kanban_application.md](workplan_kanban_application.md) — this module's scenarios are the store-contract view of the parent's behaviors.

## Goal

A module that owns the kanban board's data: the card collection, the three fixed columns, per-column ordering, and persistence. Callers create, list, change, and delete cards without knowing anything about storage; the board's ordering invariant (positions of every column contiguous, top-to-bottom = priority) is this module's guarantee. Observable outcome: a caller can hold a board of cards across close and reopen, and the columns' orders are always exactly what the last accepted mutation made them.

## Acceptance Criteria

### Scenario: Fresh store opens as an empty valid board
Given no data file exists at the store's path
When the store is opened
Then the board lists three columns in the order todo, in_progress, done
  And every column holds no cards

### Scenario: Create appends to bottom of first column
Given a board holding two cards in the todo column
When a card with the text "Buy milk" is created
Then the todo column lists three cards with "Buy milk" last at position 2
  And the card carries an identifier assigned by the store
  And the card's column is todo

### Scenario: Create rejects blank text
Given an open board
When a card with empty or whitespace-only text is created
Then no card is created and the store reports the text as invalid (required)

### Scenario: Create rejects over-long text
Given an open board
When a card with text longer than 500 characters is created
Then no card is created and the store reports the limit as exceeded

### Scenario: List is fixed columns in position order
Given a board holding cards spread across the three columns with positions 0..n-1 in each
When the board is listed
Then the columns arrive in the order todo, in_progress, done
  And each column's cards arrive top-to-bottom by position

### Scenario: Move changes column and keeps neighbors' order
Given a todo column holding cards [A, B, C] and an in_progress column holding [X, Y]
When card B is moved to in_progress at position 1
Then in_progress holds [X, B, Y] with positions 0, 1, 2
  And todo holds [A, C] with positions 0, 1
  And B's text and identifier are unchanged

### Scenario: Reorder within a column
Given a column holding cards [A, B, C] at positions 0, 1, 2
When card A is moved to position 2 of the same column
Then the column holds [B, C, A] with positions 0, 1, 2

### Scenario: Title change keeps place and identity
Given a board holding a card in in_progress at position 1 with identifier N
When the card's text is changed
Then the card shows the new text in in_progress at position 1 with identifier N

### Scenario: Change of unknown card is reported
Given no card exists with identifier Z
When any change targets identifier Z
Then nothing changes and the store reports no such card

### Scenario: Invalid column value is rejected
Given an open board
When a change sets a card's column to a value other than todo, in_progress, done
Then nothing changes and the store reports the column as invalid

### Scenario: Delete removes and closes the gap
Given a column holding cards [A, B, C] with positions 0, 1, 2
When card B is deleted
Then the column holds [A, C] with positions 0, 1
  And a later created card receives a fresh identifier — identifier B is never reused

### Scenario: Delete of unknown card is reported
Given no card exists with identifier Z
When a delete targets identifier Z
Then nothing changes and the store reports no such card

### Scenario: State survives store close and reopen
Given a board holding cards in a mix of columns and positions
When the store is closed and opened again at the same path
Then the board lists the same cards with the same texts, columns, and positions

### Scenario: Import seeding preserves given order
Given an empty board
When cards are seeded in bulk — a list of texts with a target column
Then the column holds exactly those texts top-to-bottom in the given order
  And each card carries a fresh identifier

## Decisions

- Title validation lives here, not in callers — Rationale: one rule source; every path into the board (API, import) gets the same guarantee. Rejected: validating only in the HTTP layer, letting direct callers insert junk.
- Positions are integers contiguous 0..n-1 per column, renormalized inside every mutating operation — Rationale: the observable order equals the stored order with no gaps; callers never manage gaps. Rejected: fractional or gapped positions, order-by-timestamp.
- The column set is the module's enumeration (todo, in_progress, done); the display titles come from the contract layer — Rationale: the flow is fixed by the parent decision; storing the enum keeps storage stable if display copy changes.
- Identifiers are auto-increment integers, never reused — Rationale: carried over from the todo store; stale pages can never address a different card. Rejected: id reuse after delete.
- Done is not stored — column membership is the only done state — Rationale: parent decision (ADR-003); a flag here would be a second truth.
- One SQLite file owned exclusively by this module — Rationale: single owner of the data-file lifecycle inside the module boundary (principle 8 at the data level); the composition root decides the path, the module owns everything behind it. Rejected: sharing the todo database file.
- Bulk seeding is an ordered insert, not a migration policy — Rationale: the store exposes a plain seed operation; deciding *what* to import stays in the composition root (parent Modularity decision). Rejected: a migration-aware store.
- Internal architecture: flat record store — the domain is one record type plus one invariant; no DDD ceremony (principle 7).

## Assumptions

- The store's file path is decided and created by the composition root — Depends: single-owner file lifecycle. If wrong: two openers race on the same file.
- SQLite serializes writers adequately for one process — Depends: single-user assumption from the parent. If wrong: concurrent mutations could interleave; same last-write-wins consequence as the parent.

## Risks

- A mutation crashes between renormalizing and committing — Impact: torn ordering. Mitigation: every mutation (including multi-column renormalization) runs in one transaction, so it applies fully or not at all.
- Renormalization on every mutation is O(column size) — Impact: irrelevant at personal-board scale. Mitigation: none needed (principle 9 — no gap-position machinery for hypothetical scale).

## Open Questions

None.

## Database

### Existing Data Store
The superseded todo app used SQLite `todos.db` with a `todos(id, title, done)` table. Extension is wrong: the card model (column + position) is not the todo model (done flag), and twisting it violates principle 2. New store per the parent decision.

### Proposed Tables
#### cards
| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | integer | primary key, auto-increment | card identifier, never reused |
| title | text | not null, non-blank, max 500 chars | the card text |
| column | text | not null, CHECK in ('todo','in_progress','done') | column membership |
| position | integer | not null, ≥ 0 | order within the column |

Index on `(column, position)` for the ordered read. Invariant maintained by every mutation: per column, positions are exactly 0..n-1.

### Relationships
None — single table.

### Data Flow
- Create: insert with position = count(cards in 'todo'), return the Card.
- List: select ordered by column order then position; grouped by the module into the three-column shape.
- Change: apply the given fields in one transaction; when column or position changes, place the card at the requested index of the target column and renormalize target (and source when changed). Amended 2026-10-07 (user decision, contract-level done freeze): the title direction is frozen against the card's CURRENT column — a change carrying text for a card that sits in done is refused with a done-frozen outcome before any write (the board is exactly as it was, no identifier consumed); the column directions on done cards are unchanged — moving out of (or into) done is not an edit, and moving out of done unlocks the title direction.
- Delete: remove and renormalize the card's column in one transaction.
- Seed: insert a caller-supplied ordered list into one column, positions assigned in order.
- The schema satisfies the API Card model exactly (id, title, column, position) — nothing more is stored.

## Modularity

### Behavior Analysis
- Board data and ordering rules — changes when the card/column model changes. Single behavior; one module.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Board data + ordering + persistence | — | board | the `todos` module's model is superseded (deleted, not twisted — principle 2); board owns the new model |

### Internal Architecture
Flat CRUD-style record store with one invariant (contiguous per-column positions) enforced in the module, not callers. No entities/aggregates/services — one record type would drown in ceremony (principle 7).

### Boundaries
- api depends on board through its in-process contract only (list/create/change/delete/seed operations and their outcomes).
- Nobody else imports board; ui never reaches it (HTTP only).
- The composition root constructs the store and owns the file path; migration calls Seed through the same contract.
