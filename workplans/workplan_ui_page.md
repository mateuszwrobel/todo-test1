# Workplan: Ui Module — htmx Page and Fragment Endpoints

## Goal

The user needs one browser page where the whole todo list is visible and every operation works without a full reload. The ui module renders the page and the server-side HTML fragments htmx swaps into it, talking to the api contract over HTTP only. Observable outcome: a single page that creates, toggles, edits, and deletes todos in place, shows stated errors, serializes its own operations, and always matches the server after a reload.

Design references: `workplans/user-journeys.md` (J1–J6, cross-cutting states) and `designs/mockups/` (list, empty, create-error, edit-row, load-error, missing-todo).

## Acceptance Criteria

### Scenario: Page shows the list truthfully
Given the store contains todos with mixed done states
When the user opens the page
Then every todo renders as a row in creation order, oldest first
  And each row shows its text and its done state readably
  And every row offers done-toggle and delete controls
  And only not-done rows offer an edit control

### Scenario: Empty list is stated, not blank
Given the store contains no todos
When the user opens the page
Then the page shows a distinct "no todos" state with the create control ready

### Scenario: Load failure is stated, not faked
Given the list cannot be read (server not reachable)
When the user opens or reloads the page
Then the page shows that the todos could not be loaded
  And it never shows an empty or stale list as if it were the truth

### Scenario: Load failure recovers by retry
Given the page is in the load-failure state
When the user activates retry
Then the list is re-read and rendered, or the failure state is restated

### Scenario: Create appends without reload
Given the page shows the list
When the user types text into the create input and submits
Then the response swaps in fresh list content without a full page reload
  And the new todo is the last row, marked not-done
  And the create input and Add button are ready for the next todo

### Scenario: Rejected create states the reason
Given the page shows the list
When the user submits empty or whitespace-only text
Then no todo is created
  And the create area states that the text is required
  And the typed text stays in the input for the correcting submit

### Scenario: Over-limit create states the limit
When the user submits text longer than 500 characters
Then no todo is created
  And the create area states the 500-character limit

### Scenario: Toggle marks done and reopens
Given the page shows a not-done todo
When the user activates its done toggle
Then the row shows the todo as done without a full reload
  And activating the same toggle again shows it not-done
  And the row's text and position never change from toggling

### Scenario: Inline edit updates in place
Given the page shows a not-done todo
When the user activates edit on the row, changes the text, and saves
Then the row shows the new text without a full reload
  And the todo remains not-done and in the same position

### Scenario: Done rows carry no edit control
Given the page freshly loads a list containing a done todo
Then that row shows no edit affordance
  And its text can only change after reopening via the toggle

### Scenario: Stale edit of a done todo is refused visibly
Given a page stale about a todo's done state shows it as not-done
When the user submits an edit for it
Then the rejection states that the todo is done and its text cannot be edited
  And the row keeps displaying its original text

### Scenario: Empty edit text keeps the original
When the user submits an edit with empty or whitespace-only text
Then the row keeps its original text
  And the edit surface states that the text is required

### Scenario: Delete drops one row
Given the page shows several todos
When the user activates delete on one row
Then the swapped-in content omits exactly that todo
  And every other row keeps its text, done state, and relative order
  And deleting the last todo lands the page in the "no todos" state

### Scenario: Missing todo states the failure for any operation
Given a page stale about a todo that no longer exists
When the user toggles, edits, or deletes it
Then no change occurs anywhere
  And the page states that the todo does not exist
  And no row is left looking like the failed operation succeeded

### Scenario: Reload matches the server
Given any sequence of successful operations has run from the page
When the user reloads
Then the rendered list equals a fresh read of the server state

### Scenario: Controls serialize operations per control
Given an operation triggered from a control is in flight
Then that control is disabled until the response arrives
  And a second activation of the same control while in flight causes no request

## Decisions
- htmx (vendored as a static asset served by this module) drives all interactivity; no other JavaScript framework — Rationale: ADR-001; hypermedia swaps match the fragment-shaped contract. Rejected: SPA + fetch code (rejected in ADR-001), hand-written fetch/JS render code (duplicates what htmx provides).
- The ui module owns two server-side surfaces: the full page (GET /) and HTML fragment endpoints for every operation (POST /ui/todos, PATCH /ui/todos/{id}, DELETE /ui/todos/{id}); fragments return swapped HTML — Rationale: htmx swaps HTML, the api contract answers JSON; a translation surface is the derived page necessity the journeys name. Rejected: browser-side JSON+JS rendering (turns the page into an SPA), content negotiation on /todos (overloads the JSON contract's one meaning per response).
- Fragment endpoints perform the operation by calling the api HTTP contract against the local listener and then render the resulting list state as HTML — Rationale: the parent boundary states ui→api is HTTP-only, never in-process, so the architecture gate (architecture.spec.toml forbids ui→api/todos imports) stays green and error statuses/messages flow from their single owner, the api contract. Rejected: in-process import of api or todos (violates the boundary and the spec gate).
- Delete acts immediately, no confirmation dialog — Rationale: the workplan requires neither; a confirmation is invented scope; deletion is a visible row removal the user can observe. Rejected: confirmation gate.
- Edit is inline on the row (edit control reveals the row's own input + Save/Cancel) — Rationale: mockup 04-edit-row; the row's done state stays visible during editing. Rejected: separate form (hides the done state the edit must stay honest about).
- The toggle is a checkbox — Rationale: one native control, both directions, readable state. Rejected: two buttons for two states.
- Rejected text survives in the create input and in the inline editor — Rationale: journeys leave input survival open; keeping it serves the correcting submit the stated errors invite, and dropping it only punishes the user. Rejected: clearing typed text.
- Errors render in a stated surface attached to the triggering area (create area or row) and, for missing/stale cases, as a banner-style notice — Rationale: mockups 03/06; a stale row must never look like it succeeded. Rejected: browser alert popups.
- Identifiers stay invisible — Rationale: scenarios require stability, not visibility. Rejected: showing ids.
- The load-failure state offers a retry control — Rationale: recovery inside GET semantics, no invented surface. Rejected: reload-only (a retry button is the same request with a clearer affordance).
- The triggered control is disabled from request until response — Rationale: required behavior from the parent workplan (scenario "Repeat activation while an operation is in flight"; journeys' interaction decision). Rejected: server-side dedup (legit repeats indistinguishable), allowing double submits.

## Assumptions
- One browser tab is the active page — Depends: the parent workplan's single-session assumption; the stale-page scenarios here cover the breakage. If wrong: concurrent tabs race, last write wins (parent states this).
- htmx handles swap semantics (hx-swap targets for the list, the create area, and per-row content) — Depends: the fragment endpoints returning well-targeted fragments. If wrong: content swaps land wrong; e2e covers each journey end-to-end.

## Risks
- Fragment/swap-target drift between fragment endpoints and page markup — Impact: an operation renders in the wrong place or not at all. Mitigation: swap targets are named on the page they belong to, rendered by this same module; Playwright journeys assert the visible result per operation.
- Loopback HTTP from fragment endpoints to the api contract — Impact: if the api base URL is wrong or unmounted, every page operation fails. Mitigation: composition root wires the address; load-failure and error surfaces state failure truthfully rather than faking success.

## API

The ui module owns these page-facing endpoints (fragments are HTML; the JSON data contract stays the api module's, unchanged):

### Endpoints
| Method | Path | Purpose | Success response |
|--------|------|---------|------------------|
| GET | / | Full page | 200 HTML page |
| POST | /ui/todos | Create from form | 201 + list fragment; on 422 from api: 422 + create-area fragment with stated error |
| PATCH | /ui/todos/{id} | Toggle done or save inline edit | 200 + fragment(s); on api 422/404: same status + row/banner fragment with the api's stated error |
| DELETE | /ui/todos/{id} | Delete row | 200 + list fragment; on api 404: 404 + banner fragment |
| GET | /static/htmx.min.js | Vendored htmx asset | 200 JS |

- Fragment responses render the current server list state after the operation (or the error surface on refusal), so a swap alone makes the page truthful.
- Page states rendered by this module: items / empty / could-not-load (with retry), row done/not-done, inline-edit row, create/edit error surfaces, missing-todo banner, control in-flight disabled — per the journeys' cross-cutting states table.
- Everything the module needs about todos arrives by calling the api contract (GET/POST/PATCH/DELETE /todos) over HTTP at runtime.

## Database
None. This module owns no persisted data; the page's state is fully replaceable by a fresh read.

## Modularity

### Behavior Analysis
- Presenting the todo list and every operation as HTML that htmx can swap, including the stated page states and error surfaces — the single reason this module changes.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Page rendering + fragment translation | none (greenfield) | ui | hides one decision: how the contract is shown and operated in a browser |

### Internal Architecture
Server-rendered HTML fragments with a template-per-surface layout (page, list, row, error surfaces). Procedural — parse form values, call the contract over HTTP, render. No framework ceremony.

### Boundaries
- Zero in-process dependencies: api and todos are reached over the local HTTP contract only; architecture.spec.toml fails a ui→api or ui→todos import.
- Owns page/fragment routes and the htmx static asset; the composition root mounts them on its single listener.
- e2e (Playwright) drives this module through a real browser against the composed server; the journeys are the script.
