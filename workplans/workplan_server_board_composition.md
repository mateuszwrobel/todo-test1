# Workplan: Server Composition + Migration

> Parent: [workplan_kanban_application.md](workplan_kanban_application.md) — this module's scenarios are the process-lifecycle view of the parent's behaviors.

## Goal

The composition root: opens the board store, serves the API and the page from one process, and owns the one-time todo import at first start. It is the only code that knows file paths and the only place startup policy lives. Observable outcome: `server start` serves a correct board on every machine state — fresh, migrated, or restarted — and the old todo file is respected as read-only evidence.

## Acceptance Criteria

### Scenario: Start serves board and page
Given a fresh machine state — no board file, no todo data file
When the server starts
Then GET /board answers the three fixed empty columns
  And the page loads from the same process

### Scenario: First start imports existing todos once
Given a todo data file exists holding not-done todos [A, B, C] and done todos [D, E]
  And the board has never been created
When the server starts
Then the board holds [A, B, C] as cards in the todo column in that order and [D, E] in the done column in that order
  And every card carries a fresh identifier
  And the todo data file is unchanged on disk
When the server restarts again
Then the board is exactly as it was — no card is re-imported

### Scenario: Start without todo data creates an empty board
Given no todo data file exists and the board has never been created
When the server starts
Then the board exists with the three fixed columns holding no cards

### Scenario: Board survives server restart
Given the server ran, the board accumulated cards in a mix of columns and positions, and the process stopped
When the server starts again at the same paths
Then the board lists the same cards with the same texts, columns, and positions

### Scenario: Interrupted import leaves no half board
Given a todo data file exists and the board has never been created
When the server process stops hard during the import
Then the next start shows either the fully imported board or a clean board with no trace of a partial import
  And the import completes on that next start if it had not landed

### Scenario: Unusable configuration fails loudly
Given the configuration names a path that cannot be opened (unreadable directory, uncreatable file)
When the server starts
Then startup fails with a stated error and no partially serving process remains

### Scenario: Clean shutdown completes in-flight work
Given a request is being processed
When the server is asked to shut down
Then the in-flight request finishes normally and the board file is closed — no mutation is torn

### Scenario: Wiring honors the dependency directions
Given all four packages (board, api, ui, server) are present
When the architecture gate runs
Then the dependency directions of the parent plan hold: ui→api via HTTP only, api→board in-process, server composing all, nothing importing server or reading data files outside its owner

## Decisions

- The composition root owns data-file lifecycle and paths; the board module owns everything behind the path — Rationale: principle 8, one owner per lifecycle; the store workplan's matching assumption. Rejected: the store choosing its own path.
- Import guard is the board's recorded import decision — a marker row the board stores beside its cards, read before import — Amended 2026-10-07, defect fix (see `__log__/2026-10-07-kw6-import-guard-marker-fix.md`): the original wording ("board already exists", presence of the board schema) proved unimplementable — the store's open commits the schema before the import decision runs, so an interrupted import would read as a created board and never complete (violating the interrupted-import scenario's retry arm); the emptiness guard substituted in its place resurrects deleted todos when the user deletes every card and restarts with the todo file still present (live-reproduced — an emptied board re-answers "never created"). The marker answers both: `Import` commits it inside the import's own transaction, a first start that imported nothing records it alone, and no later card mutation can rewind it. Rejected then and now adopted: the meta flag row — the schema-existence and emptiness alternatives each fail an acceptance scenario, so the extra state is what the behavior costs. Rejected still: a "migrated" marker file beside the data file — a second lifecycle for one fact the data file already owns (principle 8).
- Migration is composition-root code reading the todo file read-only through a narrow query — Rationale: the import policy (which column, which order) is startup policy, one behavior with one owner; the store only offers ordered seeding. Rejected: a migration-aware store, a standalone migration command.
- The import runs through the board module's own create/seed operations inside one transaction — Rationale: schema knowledge stays in one module; the transaction guarantee belongs to the writer. Rejected: raw SQL in the composition root duplicating schema knowledge.
- Existing server wiring (HTTP startup, static serving, shutdown drain) carries over; only what it composes changes — Rationale: same hidden decision (process lifecycle and composition), new parts. (ADR-003)

## Assumptions

- The todo data file, when present, holds the superseded schema todos(id, title, done) — Depends: the import mapping (not-done→todo by id order, done→done by id order). If wrong: import reports the failure loudly and the board starts empty per the unusable-input path, never guesses.
- The board store contract (open, seed, list) exists as specified in [workplan_board_store.md](workplan_board_store.md) — Depends: wave order; the import is its first caller besides the API. If wrong: nothing to compose.

## Risks

- The marker is recorded while the data insert is a separate write — Impact: marker set with cards missing, a silently half-done import whose retry the guard would refuse. Mitigation: the import operation commits its cards AND the marker inside one transaction, so an interrupted import records neither and the guard retries; only a decision that imports nothing writes the marker alone, and it leaves no card state to be half of.
- The todo file is present but corrupt — Impact: startup fails or imports nothing. Mitigation: stated loud failure; behavior then matches "start without todo data" only after the user removes/repairs the file — the server never invents data.

## Open Questions

None.

## Database

### Existing Data Store
SQLite `todos.db` with `todos(id, title, done)` — the superseded app's store, left exactly as found. This module reads it read-only, once, at first start; it never writes it. The board's own file (`kanban.db`, cards table) is owned by the board module; this module decides only its path and lifecycle (open at start, close at shutdown).

### Data Flow
- Startup: resolve paths → open board store (fresh board when the file is absent) → if no import decision is recorded AND the todo file exists: import in one transaction via the store's import operation, which commits the marker row together with the cards — not-done ordered by todo id → todo column top-to-bottom; done likewise → done column. A first start that finds no todo file records the decision without importing, so a file appearing later is past evidence, never migration material.
- Serve: compose api handlers and ui over the one board store instance.
- Shutdown: drain in-flight requests, close the store.

## Modularity

### Behavior Analysis
- Process composition and lifecycle — changes when how-the-process-is-built changes.
- One-time import policy — changes when the superseded store's shape changes; retires after the first real run on a machine holding todos.

Both are startup/ownership behaviors of one concern — the composition root — kept together (cohesion: splitting them would leave a migration module reaching into the root's decisions and vice versa).

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Composition + lifecycle | server (existing) | — | same hidden decision (lifecycle/composition), new parts |
| One-time import | server (existing) | — | startup policy with a single owner (principle 8); dies with its first real run |

### Internal Architecture
Procedural wiring: linear startup sequence, explicit shutdown. No framework, no plugin system (principle 9).

### Boundaries
- server imports board, api, ui; nothing imports server.
- server is the only code that touches file paths and the only reader of `todos.db`.
- After composition, request handling never re-enters server code beyond the served surfaces.
