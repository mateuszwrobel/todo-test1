# Workplan: Board Page (UI)

> Parent: [workplan_kanban_application.md](workplan_kanban_application.md) — this module's scenarios are the browser-observable view of the parent's behaviors, including the derived page states stated in [user-journeys_kanban.md](user-journeys_kanban.md).

## Goal

The browser page: renders the board from GET /board, carries every movement through drag-and-drop, exposes create/edit/delete controls on cards, and states every failure the contract defines. Observable outcome: one page showing three fixed columns whose cards the user drags, edits, creates, and deletes — the rendered truth always matches the server, and no movement happens any way other than dragging.

## Acceptance Criteria

### Scenario: Board renders three fixed columns
Given the server holds cards in all three columns
When the user opens the page
Then three column panels appear in the order "To Do", "In Progress", "Done"
  And each panel lists its cards top-to-bottom exactly as the server's arrays order them
  And cards in "Done" render with the done treatment while other cards render plain
  And no done checkbox or toggle exists anywhere on the page

### Scenario: Empty board renders as stated empty
Given the server holds no cards
When the user opens the page
Then the three columns render with their empty treatment — visibly an empty board, not a blank or broken page

### Scenario: Load failure renders as stated failure
Given GET /board fails (server unreachable or non-200)
When the user opens or reloads the page
Then the page states that the board could not be loaded
  And it does not render empty columns as if they were the truth
  And a reload that succeeds renders the board normally

### Scenario: Create appends without reload
Given the page shows the board
When the user types text into the create input and submits
Then the new card appears at the bottom of "To Do" without a page reload
  And the input and Add button return to ready for the next card

### Scenario: Rejected create states the reason
Given the page shows the board
When the user submits a create that the server rejects (blank or over-long text)
Then no card appears
  And the stated reason appears at the create control — "text is required" or the character limit

### Scenario: Edit updates in place
Given the page shows a card in any column
When the user edits the card's text and saves
Then the card shows the new text at the same position in the same column
  And a rejected edit (blank or over-long) leaves the card's original text visible with the stated reason

### Scenario: Delete drops one card
Given the page shows a card
When the user activates that card's delete control
Then the card disappears and its column's remaining cards keep their order with no gap
  And every other card on the board is untouched

### Scenario: Drag between columns moves via one request
Given the page shows a card in "To Do" and cards in "In Progress"
When the user drags the card into "In Progress" between two cards and releases
Then one update request carries the target column and drop position
  And the card renders in "In Progress" at the drop position and is gone from "To Do"
  And during the drag the drop position is indicated before release

### Scenario: Drag reorder persists
Given a column shows at least three cards
When the user drags a card to a new position within the same column
Then the column re-renders in the new order immediately
  And after a page reload the new order is still shown

### Scenario: Abandoned drag changes nothing
Given the page shows the board
When the user starts a drag and drops outside any valid target (or releases without a drop)
Then no request is sent and the board renders unchanged

### Scenario: Stale operation states the failure
Given the page shows a card that the server no longer holds
When the user edits, drags, or deletes that card
Then the operation is not applied — the card is not left looking as if it changed
  And the page states that the card does not exist
  And a reload renders the server's truth without that card

### Scenario: Triggered control blocks while in flight
Given the page shows the board
When the user triggers any card operation and triggers the same control again before the response arrives
Then only one request went out
  And the control becomes ready again exactly when the response arrives

### Scenario: Every state renders in the gallery
Given the board's observable states — board with cards, empty columns, load failure, inline edit band, drag placeholder with drop indication, done treatment, and each stated error surface
When the component gallery route is opened with fixture data
Then every state renders from fixtures without a live server
  And the styled board follows the kanban mockups' structure: three fixed columns, card treatment, green-check done rendering

## Decisions

- Drag-and-drop is the page's only movement mechanic — Rationale: parent decision; no button or menu fallback exists. Rejected: a move menu duplicating PATCH calls.
- The done treatment is derived from the column a card sits in — Rationale: there is no done field to render; a per-card done indicator would contradict the model. One rendering rule keyed on the column name.
- The three page states — board, empty, could-not-load — are distinguished in the render — Rationale: the journeys' derived page necessities; the user must never confuse "nothing to do" with "cannot tell". Rejected: rendering empty-on-failure.
- Mutations re-render from the server's response/board read, not local guesses — Rationale: order and renormalization are server truth; local reordering would drift. Rejected: optimistic local moves without server confirmation. (Whether an in-flight drag previews locally before the response is a style choice; the accepted state always comes from the server.)
- The existing ui module survives with its surface replaced: list-render components retire when the board renders land; the design-token/style layer stays and gains board components — Rationale: the hidden decision (how the page looks and renders) is unchanged; the model rendered is. (ADR-003)
- The component gallery route and pixel-snapshot baselines carry over from the todo app's design-system waves, extended to board states — Rationale: the infrastructure exists and proved out (deterministic baselines, fixture rendering); only the states change. Rejected: shipping the board without gallery/visual coverage.
- In-flight blocking covers every card operation including the dragged card — Rationale: parent scenario 14; a drag whose drop request is still in flight cannot be repeated.

## Assumptions

- The browser's drag events (or the chosen pointer mechanism) work on the user's setup — Depends: DnD-only movement (parent Assumption). If wrong: a keyboard/button fallback returns to scope by the parent's own note.
- The API contract serves the page's needs exactly — Depends: [workplan_api_board.md](workplan_api_board.md). If wrong: the page renders only what the contract carries; new surfaces need contract work first.

## Risks

- Positional drop indication misleads at the column edges (drop at top/bottom) — Impact: perceived off-by-one landing. Mitigation: the drop position indicator always reflects the index the request will carry; server normalization guarantees a valid order either way.
- Partial re-render races a slow response — Impact: stale flash. Mitigation: re-render is driven by the response arrival, one mutation per page action (in-flight blocking already serializes).

## Open Questions

None. (Style-level choices — drag mechanism, drop indicator form, edit inline vs form, delete confirmation — are UI-design open questions recorded in [user-journeys_kanban.md](user-journeys_kanban.md), not behavior gaps.)

## Modularity

### Behavior Analysis
- Page rendering and interaction for the board — changes when the presentation changes.
- The in-flight serialization rule — changes only if the contract's operation model changes; lives within the page behavior (one module).

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Board rendering + drag + stated errors | ui (existing) | — | same hidden decision (the page), new surface; list renders retire here |

### Internal Architecture
Procedural page over the API contract: render from board reads, issue one request per user action, surface stated failures. The surviving style/token layer keeps presentation decisions out of render logic (unchanged from the todo app's design-system waves).

### Boundaries
- ui talks to api over HTTP only — no import of board, no knowledge of databases or files.
- The page's state is fully reconstructible from one GET /board (reload resolves staleness).
