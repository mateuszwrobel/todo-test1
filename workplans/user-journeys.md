# User Journeys — Todo Application

These journeys are derived from the acceptance scenarios in [workplan_todo_application.md](workplan_todo_application.md). They are input for UI design: each journey states what the user does and what the page observably shows — behavior only, no visual design. The workplan's API section is the boundary of what the page can know; every step traces to a scenario or a contract line, and anything the page needs beyond that is explicitly marked as a derived page necessity (see Traceability). Illustrative literals from the scenarios ("Buy milk") stay illustrative.

## Actor

One single local user. No authentication, no roles — whoever holds the page acts on the same list. One browser page, served by the local server itself. Journeys assume one browser session editing at a time (workplan Assumptions); J6 covers what the user sees when that assumption breaks.

## Interaction decision — in-flight control blocking

The control that triggers an operation — the Add button, the per-row toggle, the row's Save while editing, the delete control — is disabled from the moment the request leaves until the response arrives. The create input and Add button return to ready per J2's ready-state rule. This is required behavior, not a UI decision layered on the workplan: it traces to the workplan scenario "Repeat activation while an operation is in flight" (workplan Decisions carry the matching line). One mutation per page action stays fully inside the existing contract — no new endpoints, no new fields.

Consequence: the page never issues a second mutation for an operation already in flight from that page. Same-tab double-clicks cannot produce two requests — Delete clicked twice no longer means a second request hitting 404. The page serializes its own operations per control.

## J1 — Browse the list

**Goal:** see the current todos, each one's done state, and controls for all four operations.

**Entry state:** app running; page not yet loaded or just (re)loaded.

**Main flow:**
1. User opens (or reloads) the page → page shows all todos, creation order, oldest first — newest at the bottom.
2. Each row shows its text and whether it is done or not-done.
3. Each row exposes controls to toggle its done state and delete it; not-done rows also expose a control to edit their text; the create control sits with the list, not a row.
4. User reloads the page, or restarts the server and reloads → the same todos with the same texts and done states appear. Persistence is observable at this step.

**Alternate/error flows:**
- Empty list → the page shows a distinct empty state, not a blank or broken page. The page must be able to tell "there are no todos" apart from "the list could not be read".
- List request fails (server down, non-200) → the page shows that the todos could not be loaded. It must not present an empty or stale list as if it were the truth — the actual state is unknown to the user, and the page says so.

**Exit state:** user sees the current list truthfully (empty, populated, or a stated load failure).

**API calls used:** GET /todos — 200 with the array in creation order; failure → load-error state.

**UI design hooks:**
- The page needs three distinguishable list states: items / empty / could-not-load. The empty state and the load-error state are derived page necessities, not workplan scenarios — the GET contract documents only 200.
- Done vs not-done must be readable at a glance on every row; toggle and delete affordances exist regardless of done state, the edit affordance exists only on not-done rows (done rows show none).
- The page renders the order as given (oldest first) — it never reorders on its own.
- A reload reproduces identical content — the page must not depend on anything beyond a fresh list read.

## J2 — Create a todo

**Goal:** add a todo with the text the user typed.

**Entry state:** page loaded (list or empty state shown); the create-text input is present and ready.

**Main flow:**
1. User types text (e.g. "Buy milk" — illustrative) into the create input and submits.
2. The page shows the new todo at the end of the list — creation order is oldest-first, so the newest is last.
3. The new todo is marked not-done and carries a stable identifier (same identifier across later reloads; whether it is displayed is a UI choice).

**Alternate/error flows:**
- Empty or whitespace-only text → the todo is not created; the page states that the text is required (contract: 422 "title is required"). Whether the typed text survives the rejection is a UI design decision (Open Questions).
- Text longer than 500 characters → the todo is not created; the page states the limit (contract: 422; the error states the limit).

**Exit state:** list contains the new todo last, not-done — or, on a rejected submit, the list is unchanged: no todo was created and the stated reason was shown.

**API calls used:** POST /todos — 201 with the created todo (`done` false); 422 on validation failure.

**UI design hooks:**
- The page must be able to show the new todo at the end of the list without requiring a full reload to become correct.
- The create input and Add button must have a ready state — before the first todo, after a success, and after a rejection — so the user can immediately create the next one: after each response, success or rejection, input and button return to ready. While the create request is in flight both are disabled (in-flight blocking).
- Rejections must surface the stated reason (text required / over the limit) attached to the create control. Whether the rejected text stays in the input for the correcting submit is a UI design decision (Open Questions).

## J3 — Change done state (mark done / reopen)

**Goal:** flip a todo between not-done and done with one toggle. One control, both directions — the update contract carries the done flag either way (workplan Decisions).

**Entry state:** list loaded; the todo's current done state is visible.

**Main flow (mark done):**
1. User activates the toggle on a not-done todo.
2. The row shows the todo as done.
3. The todo's text is unchanged; its position in the list is unchanged.

**Alternate flows:**
- Reopen: user activates the same toggle on a done todo → the row shows it as not-done; text unchanged, position unchanged.
- Toggle fails (todo no longer exists — see J6) → the page states that the todo does not exist; nothing changes.

**Exit state:** the todo shows the flipped done state; everything else is as before.

**API calls used:** PATCH /todos/{id} carrying the done field — 200 with the updated todo; 404 "no such todo".

**UI design hooks:**
- One per-row control must carry both directions, and its current position must be readable — the user can tell done from not-done without acting.
- Toggling must leave list order and the row's text untouched.
- The toggle is disabled from request until response (in-flight blocking) — a second click cannot reach the server while the toggle is in flight.

## J4 — Edit a todo's text

**Goal:** change a todo's text without changing anything else about it. A done todo's text is frozen — reopening first (J3) is the documented path (workplan Decisions).

**Entry state:** list loaded; the todo's text and done state are visible; the todo is not-done — done rows carry no edit affordance.

**Main flow:**
1. User changes the todo's text and submits the edit.
2. The row shows the new text.
3. The todo remains not-done.
4. The todo's position in the list is unchanged.

**Alternate/error flows:**
- The todo is done → on a freshly loaded page no edit affordance exists on its row; the path to change its text is reopen via J3, edit, then mark done again.
- A stale page still showing a done todo as not-done submits an edit → rejected (422 "cannot edit a done todo"); the row keeps its original text and the page states that the todo is done and its text cannot be edited.
- New text empty or whitespace-only → rejected (422 "title is required"); the page states that the text is required and the row keeps its original text.
- Edit targets a todo that no longer exists → see J6.

**Exit state:** the todo shows the new text with the same id, still not-done, same position — or, on rejection, the original text intact.

**API calls used:** PATCH /todos/{id} carrying the title field — 200 with the updated todo; 422 on validation failure or when the todo is done; 404 per J6.

**UI design hooks:**
- Editing must be reachable per not-done row and the result shown in place — inline vs a separate input is a UI decision (Open Questions).
- Done rows show no edit affordance; the page derives this from the row's own done state — no contract surface beyond the 422 the stale-page path exercises.
- A rejected edit must surface its stated reason tied to that row, while the row itself keeps displaying the original text.
- The row's done state stays visible throughout the edit.
- The row's Save control is disabled from request until response (in-flight blocking) — a second click cannot reach the server while the save is in flight.

## J5 — Delete a todo

**Goal:** remove a todo from the list for good.

**Entry state:** list loaded; the todo is visible.

**Main flow:**
1. User activates delete on the todo.
2. The todo is gone from the list.
3. Every other row is unchanged: same texts, same done states, same relative order.

**Alternate/error flows:**
- The todo no longer exists → the page states that the todo does not exist; nothing changes (J6).
- Deleting the last todo leaves the empty state (J1) — not a load error.

**Exit state:** the list without that todo. Deletion is final — no undo is in scope (the workplan contains none; adding it would be invented scope).

**API calls used:** DELETE /todos/{id} — 204 with no body; 404 "no such todo".

**UI design hooks:**
- The page must be able to drop exactly one row and leave the rest rendered unchanged.
- Whether delete acts immediately or asks for confirmation is a UI decision — the workplan requires neither (Open Questions).
- The delete control is disabled from request until response (in-flight blocking) — a second click cannot reach the server while the delete is in flight.
- The post-delete list may be empty; the empty state must cover that arrival path too.

## J6 — Operate on a todo that no longer exists

**Goal / context:** the edge of the single-user assumption. The page is stale — it shows a todo that no longer exists. Double-submit from this page is now closed: the in-flight control blocking decision — required behavior, the workplan scenario "Repeat activation while an operation is in flight" — disables the triggered control until its response arrives, so a second mutation of an operation already in flight never leaves this page — the double-clicked Delete whose second request hit 404 is no longer a reachable path. The paths that remain: a second browser tab acting from its own page snapshot, a server restart against a different or emptied data file (workplan Assumptions/Risks), and direct edits of the data file. Last write wins; a concurrent edit can be lost. The app does not merge or revive anything — the operation simply does not take effect. (Noted once here; not repeated in J3–J5 beyond their error flows.)

**Entry state:** a stale page showing at least one todo whose identifier no longer exists.

**Main flow:**
1. User attempts any mutation on the missing todo — edit text, toggle done, or delete.
2. The operation does not take effect: no todo changes anywhere.
3. The page states that the todo does not exist (contract: 404 "no such todo").
4. User reloads the page → it shows the current truth: that todo is not among the todos.

**Alternate/error flows:**
- All three operations (edit / toggle / delete) produce the same outcome and the same stated message — one journey covers all three.

**Exit state:** the user knows the todo is gone; a reloaded page matches the server.

**API calls used:** PATCH /todos/{id} → 404; DELETE /todos/{id} → 404; GET /todos on the recovery reload → 200.

**UI design hooks:**
- The page must be able to surface a "todo does not exist" error per operation, whichever control triggered it.
- A stale row must never be left looking like the operation succeeded — when the answer is 404, the page says it failed.
- Reload resolves staleness: the page's state must be fully replaceable by a fresh list read.

## Cross-cutting UI states

Every observable page/app state the journeys require:

| Observable state | Needed by |
|---|---|
| List loaded with items, oldest first | J1, J2, J3, J4, J5 |
| Empty list — "no todos", distinct from load failure * | J1, J2 (empty start), J5 (last deleted) |
| List load failure — "could not load", truth unknown * | J1 |
| Todo row: not-done | J1, J2, J3, J4 |
| Todo row: done — no edit affordance | J1, J3, J4 |
| Error surface "text is required" — create | J2 |
| Error surface "text is required" — edit | J4 |
| Error surface "cannot edit a done todo" | J4 |
| Error surface "over the 500-character limit" | J2 |
| Error surface "todo does not exist" | J6 (and J3/J4/J5 error flows) |
| Create input ready — initial / after success / after rejection | J2 |
| Control in-flight — triggered control disabled, response pending | J2, J3, J4, J5 |
| Row unchanged except the field just acted on | J3, J4, J5 |

\* Empty-list and load-failure states are derived page necessities, not workplan scenarios — the GET /todos contract documents only 200. The page still cannot render the truth without telling these three list outcomes apart.

The contract's 422 "at least one field required" (empty PATCH body) is defensive: the page always sends at least one field, so no journey step triggers it; it lives in the API contract only.

## Traceability

| Journey | Workplan scenario(s) | Endpoint(s) |
|---|---|---|
| J1 Browse | Todos survive server restart; "list shows…" clauses of every scenario | GET /todos |
| J2 Create | Create todo; Reject empty todo text | POST /todos |
| J3 Toggle done | Mark todo done (reopen per Decisions line) | PATCH /todos/{id} |
| J4 Edit text | Edit todo text leaves done state alone; Done todo rejects text edits | PATCH /todos/{id} |
| J5 Delete | Delete todo | DELETE /todos/{id} |
| J6 Stale id | Operation on missing todo | PATCH /todos/{id}, DELETE /todos/{id} |

All 9 workplan scenarios are covered: Create (J2), Reject empty (J2), Mark done (J3), Edit leaves done alone (J4), Done refuses edits (J4), Delete (J5), Survive restart (J1), Missing todo (J6), Repeat activation in flight (J2–J5, via the in-flight control blocking decision). Error variants trace to contract lines: POST 422 (J2), PATCH 422 (J4), 404s (J6). Journey steps trace to a scenario or a contract line, with one stated exception: the empty-list and load-failure page states (J1) are derived page necessities — the GET contract documents only 200, and no scenario describes a failed load.

## Open questions for UI design

Decisions the workplan deliberately leaves open — to be settled in UI design, not added as features:

- Delete: immediate, or confirmation-gated? The workplan requires neither.
- Edit: inline on the row, or a separate input/form?
- Toggle control form: checkbox, button, other — it must carry both directions and read its state.
- Error surfacing: on submit only, or earlier too (e.g. a live character count)? The stated contract messages are the floor; the page may add, never replace.
- Are identifiers ever visible to the user? Scenarios require stability, not visibility.
- After a rejection, does the attempted text stay in the input — for create, for edit, or neither? The workplan only requires: rejected create creates nothing, rejected edit keeps the original text; input survival is unstated for both.
- Load-failure recovery: does the error state offer a retry, or is reload the only recovery? Both stay inside GET /todos.
