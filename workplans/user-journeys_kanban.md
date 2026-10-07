# User Journeys — Kanban Board Application

These journeys are derived from the acceptance scenarios in [workplan_kanban_application.md](workplan_kanban_application.md). They are input for UI design: each journey states what the user does and what the page observably shows — behavior only, no visual design. The workplan's API section is the boundary of what the page can know; every step traces to a scenario or a contract line, and anything the page needs beyond that is explicitly marked as a derived page necessity (see Traceability). Illustrative literals from the scenarios stay illustrative.

## Actor

One single local user. No authentication, no roles — whoever holds the page acts on the same board. One browser page, served by the local server itself. Journeys assume one browser session editing at a time (workplan Assumptions); J8 covers what the user sees when that assumption breaks.

## Interaction decision — in-flight control blocking

The control that triggers an operation — the create input and its button, a card's edit-save, its delete control, or the card being dragged — is disabled from the moment the request leaves until the response arrives. This is required behavior, not a UI decision layered on the workplan: it traces to the workplan scenario "Repeat activation while an operation is in flight" (workplan Decisions carry the matching line), carried over unchanged from the todo app. One mutation per page action stays fully inside the existing contract — no new endpoints, no new fields.

Consequence: the page never issues a second mutation for an operation already in flight from that page. A dragged card cannot be dropped twice into two requests; a double-clicked Delete is no longer a second request hitting 404. The page serializes its own operations per control.

## J1 — View the board

**Goal:** see the whole board — three fixed columns, their cards in priority order, each card's done-or-not rendering, and the controls for every operation.

**Entry state:** app running; page not yet loaded or just (re)loaded.

**Main flow:**
1. User opens (or reloads) the page → the board shows exactly three columns in the fixed order "To Do", "In Progress", "Done".
2. Each column lists its cards top-to-bottom in stored position order — position is priority, so the top card is the highest-priority item of that column.
3. Cards in "Done" render as done; cards in the other two columns render as not-done. No separate done indicator or control exists anywhere on the page — the column a card sits in is the whole done story.
4. Each card exposes controls to edit its text and delete it; every card is also draggable within its column and into other columns. The create control sits with the board, not on a card (control placement is a page necessity — the contract has no per-card create operation).
5. User reloads the page, or restarts the server and reloads → the same cards with the same texts, columns, and positions appear. Persistence is observable at this step.

**Alternate/error flows:**
- Board freshly created with no cards → each column shows its own empty state; the page shows three empty columns, not a blank or broken page.
- Board request fails (server down, non-200) → the page shows that the board could not be loaded. It must not present empty columns as if they were the truth — the actual state is unknown to the user, and the page says so.

**Exit state:** user sees the current board truthfully (populated, empty, or a stated load failure).

**UI design hooks:**
- The page needs three distinguishable board states: cards / all-empty / could-not-load. The empty and load-error states are derived page necessities, not workplan scenarios — the GET contract documents only 200.
- Column order is fixed by the contract's array order; the page renders columns as given and never reorders columns or cards on its own.
- Done rendering must be readable at a glance and derive solely from the card's column — no per-card done field exists in the contract to render.
- A reload reproduces identical content — the page must not depend on anything beyond a fresh board read.

**API calls used:** GET /board — 200 with the three columns and their card arrays in position order; failure → load-error state.

## J2 — Create a card

**Goal:** add a card with the text the user typed; it enters at the bottom of "To Do".

**Entry state:** page loaded (board or empty state shown); the create-text input is present and ready.

**Main flow:**
1. User types text (e.g. "Buy milk" — illustrative) into the create input and submits.
2. The "To Do" column shows the new card at its bottom — below every card already there.
3. The new card renders as not-done (it is in "To Do") and carries a stable identifier (same identifier across later reloads; whether it is displayed is a UI choice).

**Alternate/error flows:**
- Empty or whitespace-only text → the card is not created; the page states that the text is required (contract: 422 "title is required"). Whether the typed text survives the rejection is a UI design decision (Open Questions).
- Text longer than 500 characters → the card is not created; the page states the limit (contract: 422; the error states the limit).

**Exit state:** "To Do" contains the new card last — or, on a rejected submit, the board is unchanged: no card was created and the stated reason was shown.

**UI design hooks:**
- The page must be able to append the new card to the bottom of "To Do" without a full reload to become correct.
- The create input and button must have a ready state — before the first card, after a success, and after a rejection — so the user can immediately create the next one: after each response, success or rejection, input and button return to ready. While the create request is in flight both are disabled (in-flight blocking).
- Rejections must surface the stated reason (text required / over the limit) attached to the create control.

**API calls used:** POST /cards — 201 with the created card (`column` "todo", bottom `position`); 422 on validation failure.

## J3 — Move a card between columns

**Goal:** advance or return a card along the flow by dragging it into another column, dropping it where it belongs.

**Entry state:** board loaded; the card is visible in its source column; the target column is visible.

**Main flow:**
1. User grabs the card and drags it onto the target column, positioning it between two of the target's cards (or at its top/bottom).
2. The card appears in the target column at the drop position.
3. The card is gone from the source column; the cards left there keep their relative order.
4. The card's text and identifier are unchanged — the drag moved it, nothing else.

**Alternate/error flows:**
- The card no longer exists (stale page — see J8) → the move does not take effect; the page states that the card does not exist.
- The drag is dropped outside any valid target → nothing is sent; the board stays as it was (derived page necessity: a drag that never completes a drop is not an operation).

**Exit state:** the card sits in the target column at the chosen position; both columns are otherwise unchanged.

**UI design hooks:**
- A card must be grabbable and droppable into any column, including between two existing cards — the page needs a way to show where the card will land during the drag (drop-position indication is a derived page necessity; no scenario describes mid-drag rendering).
- Both the source and the target column must update from one operation — the contract is a single PATCH carrying column and position.
- The dragged card is inactive from request until response (in-flight blocking) — the same drag cannot produce two requests.

**API calls used:** PATCH /cards/{id} carrying column and/or position — 200 with the updated card; the server renormalizes positions of affected columns; 404 per J8.

## J4 — Reorder within a column

**Goal:** change a card's priority inside its column by dragging it to a new slot.

**Entry state:** board loaded; the column holds at least three cards.

**Main flow:**
1. User drags a card to a different position in the same column (e.g. the top card below the second card).
2. The column lists its cards in the new order immediately.
3. After reloading the page the new order is still shown — reordering is stored state, not a view trick.

**Alternate/error flows:**
- The card no longer exists (stale page — see J8) → the reorder does not take effect; the page states that the card does not exist.

**Exit state:** the column shows the new top-to-bottom priority; other columns untouched.

**UI design hooks:**
- Same drag affordance as J3 — moving and reordering are one gesture; the page derives from drop context whether column changed.
- The page renders the position order it receives; the server guarantees contiguity, so there are never gaps to render around.
- The dragged card is inactive from request until response (in-flight blocking).

**API calls used:** PATCH /cards/{id} carrying position — 200 with the updated card; 404 per J8.

## J5 — Complete and reopen a card

**Goal:** mark a card done by dragging it into "Done"; undo that by dragging it back. Column membership is the whole done mechanism.

**Entry state:** board loaded; the card sits outside "Done" (to complete) or inside it (to reopen).

**Main flow (complete):**
1. User drags the card into the "Done" column (J3 mechanics).
2. The card now renders as done.

**Main flow (reopen):**
1. User drags a "Done" card back into "To Do" or "In Progress".
2. The card renders as not-done again.

**Alternate/error flows:**
- Nowhere does a separate done control exist — no checkbox, no toggle, no menu item. Any "mark done" UI beyond column position would contradict the contract's model and the workplan decision.
- The card no longer exists (stale page — see J8) → the move does not take effect; the page states that the card does not exist.

**Exit state:** the card's done rendering matches its column; text, identifier, and everything else unchanged.

**UI design hooks:**
- Done styling of a card derives only from its column — one rendering rule, no per-card state to track.
- "Done" cards must be just as draggable as the others — reopening must not look or feel disabled by the done styling.

**API calls used:** PATCH /cards/{id} carrying column — 200; identical contract to J3. No done field exists anywhere in the API.

## J6 — Edit a card's text

**Goal:** change a card's text without changing its column, position, or identity. Amended 2026-10-07 (user decision): a card sitting in the "Done" column refuses text changes — the contract-level done freeze restores the todo app's frozen-text rule and supersedes this journey's pivot-era sentence ("Unlike the todo app, a done card's text is editable", 2026-10-06). Dragging the card out of "Done" is the way to make it editable again.

**Entry state:** board loaded; the card is visible in any column.

**Main flow:**
1. User activates the card's edit control, changes the text, and submits.
2. The card shows the new text.
3. The card stays in the same column at the same position; its identifier is unchanged.

**Alternate/error flows:**
- New text empty or whitespace-only → rejected (422 "title is required"); the card keeps its original text and the page states that the text is required.
- New text longer than 500 characters → rejected (422, states the limit); the card keeps its original text.
- The card sits in the "Done" column → the change is refused with a stated error (amended 2026-10-07, user decision: done freeze); the card keeps its original text and the board is unchanged. Dragging it out of "Done" first unlocks editing.
- The card no longer exists (stale page — see J8) → the edit does not take effect; the page states that the card does not exist.

**Exit state:** the card shows the new text with the same id, column, and position — or, on rejection, the original text intact.

**UI design hooks:**
- Editing must be reachable on every card outside the "Done" column — Done cards carry no edit control at all (amended 2026-10-07, user decision: the contract-level done freeze restores the todo app's hidden-affordance rule, superseding the pivot's "unlike the todo app there is no done-state that hides the edit affordance"). The contract's stated refusal stays the guarantee underneath, whatever a stale or hand-made request submits.
- A rejected edit must surface its stated reason tied to that card while the card keeps displaying its original text.
- The save control is disabled from request until response (in-flight blocking).

**API calls used:** PATCH /cards/{id} carrying title — 200 with the updated card; 422 on validation failure; 422 with the done-freeze stated refusal for a card in "Done" (amended 2026-10-07); 404 per J8.

## J7 — Delete a card

**Goal:** remove a card from the board for good.

**Entry state:** board loaded; the card is visible.

**Main flow:**
1. User activates delete on the card.
2. The card is gone from its column.
3. The column's other cards keep their relative order, packed with no gap — the list simply has one fewer card.

**Alternate/error flows:**
- The card no longer exists → the page states that the card does not exist; nothing changes (J8).
- Deleting the last card of a column leaves that column's empty state (J1) — not a load error.

**Exit state:** the board without that card. Deletion is final — no undo is in scope (the workplan contains none; adding it would be invented scope).

**UI design hooks:**
- The page must be able to drop exactly one card and leave the rest rendered in order — position renormalization is server-side, the page just renders the new array.
- Whether delete acts immediately or asks for confirmation is a UI decision — the workplan requires neither (Open Questions).
- The delete control is disabled from request until response (in-flight blocking).

**API calls used:** DELETE /cards/{id} — 204 with no body; 404 "no such card".

## J8 — Operate on a card that no longer exists

**Goal / context:** the edge of the single-user assumption. The page is stale — it shows a card that no longer exists. Double-submit from this page is closed: the in-flight control blocking decision — required behavior, the workplan scenario "Repeat activation while an operation is in flight" — disables the triggered control until its response arrives, so a second mutation of an operation already in flight never leaves this page. The paths that remain: a second browser tab acting from its own snapshot (workplan Assumptions/Risks), or a server restart against a different data file, or direct edits of the data file — the latter two produce equivalent staleness but are not named in the kanban workplan's risk list. Last write wins; a concurrent drag can be lost. The app does not merge or revive anything — the operation simply does not take effect. (Noted once here; not repeated in J3–J7 beyond their error flows.)

**Entry state:** a stale page showing at least one card whose identifier no longer exists.

**Main flow:**
1. User attempts any mutation on the missing card — edit its text, drag it, or delete it.
2. The operation does not take effect: no card changes anywhere.
3. The page states that the card does not exist (contract: 404 "no such card").
4. User reloads the page → it shows the current truth: that card is not on the board.

**Alternate/error flows:**
- All operations (edit / drag-move / reorder / delete) produce the same outcome and the same stated message — one journey covers all of them.

**Exit state:** the user knows the card is gone; a reloaded page matches the server.

**UI design hooks:**
- The page must be able to surface a "card does not exist" error per operation, whichever control triggered it — including a failed drop.
- A stale card must never be left looking like the operation succeeded — when the answer is 404, the page says it failed.
- Reload resolves staleness: the page's state must be fully replaceable by a fresh board read.

**API calls used:** PATCH /cards/{id} → 404; DELETE /cards/{id} → 404; GET /board on the recovery reload → 200.

## J9 — First run with an existing todo list

**Goal / context:** the one-time migration (workplan scenario "Migrate existing todos on first start"). The user's old todo list becomes the board without any manual step. This journey is server-side; the page only ever sees its result through J1.

**Entry state:** the server has never created the board; the user's todo data store exists holding not-done and done todos.

**Main flow:**
1. User starts the server and opens the page.
2. The board exists: the old not-done todos are cards in "To Do", top-to-bottom in creation order (oldest at top); the old done todos are cards in "Done", likewise ordered. "In Progress" is empty.
3. The cards carry fresh identifiers — old todo ids are not reused.
4. From here the board is ordinary: creating, dragging, editing, deleting work per J2–J7.
5. Server restarts later never re-import — the board shows only what the user has since made of it.

**Alternate/error flows:**
- No todo data store exists → the board is created empty with the three fixed columns (workplan scenario "Start without todo data"); J1's per-column empty states show.
- Server crashes mid-import → the transaction rolls back: nothing is imported and no half-board exists; the next start retries the whole import.

**Exit state:** the user's backlog sits on the board, prioritized implicitly by age; the migration never runs twice.

**UI design hooks:**
- None beyond J1 — the import is invisible to the page; the first page load simply shows a populated board. No "imported!" notice exists in the contract; adding one would be invented scope (Open Questions if wanted).

**API calls used:** none from the page; the import is composition-root work at startup.

## Cross-cutting UI states

Every observable page/app state the journeys require:

| Observable state | Needed by |
|---|---|
| Board loaded — three fixed columns, cards in position order | J1, J2, J3, J4, J5, J6, J7 |
| Column empty — "no cards", distinct from load failure * | J1, J2 (empty start), J7 (last deleted), J9 (fresh "In Progress") |
| Board load failure — "could not load", truth unknown * | J1 |
| Card: not-done (in "To Do"/"In Progress") | J1–J6 |
| Card: done (in "Done") — rendered as done, still draggable | J1, J5, J6 |
| Card being dragged / drop-position indication * | J3, J4, J5 |
| Error surface "text is required" — create | J2 |
| Error surface "text is required" — edit | J6 |
| Error surface "over the 500-character limit" | J2, J6 |
| Error surface "card does not exist" | J8 (and J3–J7 error flows) |
| Error surface "invalid column" — defensive contract path only | — |
| Create input ready — initial / after success / after rejection | J2 |
| Control in-flight — triggered control disabled, response pending | J2, J3, J4, J5, J6, J7 |
| Card unchanged except the field just acted on | J3, J4, J5, J6, J7 |

\* Empty-column, load-failure, and mid-drag drop-indication states are derived page necessities, not workplan scenarios — the GET contract documents only 200 and no scenario describes a failed load or mid-drag rendering. The page still cannot render the truth without telling board outcomes apart, and cannot execute a positional drop without showing where it will land.

The contract's 422 "invalid column" and "at least one field required" are defensive: the page only ever sends the three real column names and at least one field, so no journey step triggers them; they live in the API contract only.

## Traceability

| Journey | Workplan scenario(s) | Endpoint(s) |
|---|---|---|
| J1 View | Board shows fixed columns; Board survives server restart; start-state clauses of every scenario | GET /board |
| J2 Create | Create card; Reject empty card text; Reject over-long card text | POST /cards |
| J3 Move between columns | Drag card between columns | PATCH /cards/{id} |
| J4 Reorder within column | Drag reorder within a column | PATCH /cards/{id} |
| J5 Complete / reopen | Done is column membership | PATCH /cards/{id} |
| J6 Edit text | Edit card text keeps place | PATCH /cards/{id} |
| J7 Delete | Delete card | DELETE /cards/{id} |
| J8 Stale id | Operation on missing card | PATCH /cards/{id}, DELETE /cards/{id} |
| J9 First-run import | Migrate existing todos on first start; Start without todo data | — (startup work) |

All 14 workplan scenarios are covered: Fixed columns (J1), Create (J2), Reject empty (J2), Reject over-long (J2), Drag between (J3), Drag reorder (J4), Done-is-membership (J5), Edit keeps place (J6), Delete (J7), Survive restart (J1), Missing card (J8), Migrate (J9), Start empty (J9), Repeat activation in flight (J2–J7, via the in-flight control blocking decision). Error variants trace to contract lines: POST 422 (J2), PATCH 422 (J6), 404s (J8). Journey steps trace to a scenario or a contract line, with stated exceptions: the empty-column, load-failure, and mid-drag drop-indication page states are derived page necessities — the GET contract documents only 200, and no scenario describes a failed load or mid-drag rendering.

## Open questions for UI design

Decisions the workplan deliberately leaves open — to be settled in UI design, not added as features:

- Delete: immediate, or confirmation-gated? The workplan requires neither.
- Edit: inline on the card, or a separate input/form?
- Drop indication: placeholder gap, insertion line, ghost card — the mechanics must show where the card lands, the style is open.
- Drag mechanism: native HTML5 drag-and-drop vs a pointer-events library — behavior is fixed (drag to move), the means is an implementation choice with the browser-support assumption attached.
- Error surfacing: on submit only, or earlier too (e.g. a live character count)? The stated contract messages are the floor; the page may add, never replace.
- Are identifiers ever visible to the user? Scenarios require stability, not visibility.
- After a rejection, does the attempted text stay in the input — for create, for edit, or neither? The workplan only requires: rejected create creates nothing, rejected edit keeps the original text; input survival is unstated for both.
- Load-failure recovery: does the error state offer a retry, or is reload the only recovery? Both stay inside GET /board.
- Migration notice: should the first page load after import mention that the todo list was imported? The contract has no such surface — staying silent is the derived default; adding a notice needs an explicit yes.
