# e2e — the kanban browser suite

Browser-driven acceptance for the kanban app: each wave's lane script drives
the composed server in real chromium and re-executes that wave's parent
scenarios from `workplans/workplan_kanban_application.md` (acceptance, never
new Gherkin — see `workplans/dependencies_kanban.md`). The suite is CommonJS
scripts using the global `playwright` package; the lane scripts are
self-contained — each builds its own binaries, seeds fresh temp data files,
serves on a free port, and stops the server with SIGTERM.

The superseded todo-app lanes (w1..w11: browse, create, toggle, edit,
delete, stale, lifecycle, in-flight, gallery, components, visual) retired
with the kanban pivot's wave-end swap; their specs and pixel baselines
covered surfaces the pivot deleted.

## Run

From the repo root:

```sh
make e2e
```

Requirements: Go toolchain, the global `playwright` CLI package with
chromium installed (`playwright --version` works; `npm root -g` gives the
`NODE_PATH` the scripts' `require('playwright')` needs).

## KW1 — KW1 board foundation (`e2e/kw1-board.js`)

Re-executes the parent scenarios "Board shows fixed columns" and "Start
without todo data".

### What it asserts

1. **Board shows fixed columns** — a board seeded with cards in all three
   columns renders exactly three columns in the order To Do / In Progress /
   Done; each column lists its cards top-to-bottom in stored position order
   (the DOM order mirrors the contract's array order, and the stored
   positions are contiguous 0..n-1); the done treatment is carried purely by
   membership of the Done column — Done cards show it, no other card does,
   and no done checkbox or toggle exists anywhere on the page.
2. **Start without todo data** — with no data file at all, the server
   starts, the board is created empty, and the page shows the three fixed
   columns each stating its emptiness; no cards render, and GET /board
   answers the three fixed columns, each empty.

## KW2 — Create (`e2e/kw2-create.js`)

Re-executes the parent scenarios "Create card", "Reject empty card text"
and "Reject over-long card text". Each scenario gets its own seeded temp
board, port, and server (SIGTERM teardown), mirroring the KW1 lane.

### What it asserts

1. **Create card** — on a seeded board, creating "Buy milk" through the
   create band appends it at the bottom of To Do *without a page reload*
   (a window-object marker survives the append — only a fragment swap keeps
   page-owned state), previously stored cards keep their order above it,
   the card renders without the done treatment, its identifier matches the
   stored one in GET /board, and a later reload shows the same card at the
   same place in agreement with a fresh GET /board.
2. **Reject empty card text** — submitting whitespace-only text states the
   contract's "title is required" at the create control, adds no card (DOM
   and GET /board unchanged), and the control is ready again: the next
   valid submit appends at To Do's bottom.
3. **Reject over-long card text** — submitting 501 characters states the
   character limit at the create control, adds no card, and leaves the
   board unchanged.

## KW3 — Edit (`e2e/kw3-edit.js`)

Re-executes the parent scenarios "Edit card text keeps place" and
"Operation on missing card" (the edit leg — KW3 owns the PATCH surface;
the move and delete legs join at KW5 and KW4), plus the edit's stated
rejections on the card surface. Each scenario gets its own seeded temp
board, port, and server (SIGTERM teardown), mirroring the KW1/KW2 lanes.

### What it asserts

1. **Edit card text keeps place** — on a board seeded across all three
   columns, editing a middle To Do card shows the new text at the same
   position between the same neighbors under the same identifier *without
   a page reload* (window marker survives), without the done treatment;
   editing a Done card keeps that card's done treatment; the band opens
   prefilled with the existing text; a reload shows the same board as a
   fresh GET /board.
2. **Operation on missing card — edit leg** — after the page is rendered,
   the target card is removed server-side out-of-band (direct `sqlite3`
   DELETE against the board data file from the lane process: no card
   DELETE endpoint exists before KW4, so an out-of-band file mutation is
   the honest way to stage "no card exists with identifier X" against a
   live server). Editing the stale card states "no such card" in the
   missing-card banner, the card is gone from the page with no phantom
   remnant, the page mirrors GET /board, and the statement never survives
   a reload.
3. **Edit rejections stated at the card** — a blank title states "title
   is required" at the editing card with the original text intact (DOM and
   GET /board unchanged, titles intact after reload); 501 characters state
   the character limit at the card, board untouched.

## KW4 — Delete (`e2e/kw4-delete.js`)

Re-executes the parent scenario "Delete card" and the delete leg of
"Operation on missing card" (KW4 owns the DELETE surface), plus the stated
empty treatment when a delete empties a column (card ui/02's rule reached
through deletion). Each scenario gets its own seeded temp board, port, and
server (SIGTERM teardown), mirroring the KW1–KW3 lanes.

### What it asserts

1. **Delete card** — on a board seeded across all three columns, deleting a
   middle To Do card drops it *without a page reload* (window marker
   survives), the column's remaining cards keep their relative order with no
   gap (DOM order card-for-card equals GET /board order, whose positions are
   contiguous 0..n-1), and every other card on the board is untouched;
   deleting the Done card — a done card is as deletable as any other — empties
   Done into its stated treatment; a reload still shows both cards gone, in
   agreement with a fresh GET /board.
2. **Operation on missing card — delete leg** — after the page is rendered,
   the target card is removed server-side out-of-band (direct `sqlite3`
   DELETE against the board data file from the lane process, keeping the page
   stale). Clicking its Delete states "no such card" in the missing-card
   banner without a reload, the card is gone from the page, the board under
   the failure IS GET /board — nothing faked — and the statement never
   survives a reload.
3. **Delete a column's last card** — the emptied column shows its stated
   empty treatment while every other column keeps its cards; no reload, and
   the DOM mirrors GET /board, agreement intact after a reload.

## KW5 — Drag: move, reorder, done membership (`e2e/kw5-drag.js`)

Re-executes the parent scenarios "Drag card between columns", "Drag reorder
within a column" and "Done is column membership", plus the MOVE leg of
"Operation on missing card" — the leg that completes that scenario fully
(edit leg KW3, delete leg KW4). Each scenario gets its own seeded temp board,
port, and server (SIGTERM teardown), mirroring the KW1–KW4 lanes; the
missing-card scenario adds a second browser context on the same server.

Drag is real HTML5 DnD in chromium: the lane drives mouse choreography
(hover, mouse.down, stepped mouse.moves, mouse.up) — Playwright's drag
interception turns these into dragstart/dragenter/dragover/drop. The
one-request-per-drop and zero-request-per-abandonment rules are proven by
counting PATCH requests with a `page.on('request')` listener per gesture:
exactly one per accepted drop (body carries the target column + drop index),
zero per abandoned one. The server-side contract-call count (ui → api PATCH)
is ui/move_test.go's recorder; from the browser the client's single fetch is
what the lane counts.

### What it asserts

1. **Drag card between columns** — dragging a middle To Do card onto In
   Progress *between its two cards* issues EXACTLY ONE PATCH with body
   `{column: "In Progress", position: 1}`, the card lands at the drop index
   (DOM order card-for-card equals GET /board under contiguous positions),
   the source column packs its gap, text and identifier are unchanged, no
   reload (window marker survives), and drag chrome cleans up after itself.
   Mid-gesture the insertion line's position is witnessed before mouse-up
   (best-effort; the request body's index is the hard proof that the marked
   gap is the gap that landed).
2. **Drag reorder within a column** — dragging the bottom card of a
   four-card To Do column to its top is one PATCH with position 0 (the
   contract's index-after-removal), the column lists the new order
   immediately, and the order survives an actual `page.reload()` with the
   stored positions contiguous 0..n-1 agreeing card-for-card.
3. **Done is column membership** — dragging the In Progress card into Done
   lands it with the done treatment (`card--done` off membership), and
   dragging it back out clears the treatment. No separate done control
   exists anywhere on the page at any point: zero checkbox/radio/switch
   markup, no done/toggle button or link. A cross-check block then
   re-exercises kw3's "editing a Done card keeps the treatment" on a card
   the DRAG placed in Done — asserted on the fresh truth render (one PATCH
   to the card endpoint, no reload, treatment kept) — and pins the
   post-drop wiring hard: on markup the drag's fetch swap injected, an
   edit Save is exactly one PATCH to the card endpoint with no navigation,
   and a Delete is exactly one DELETE with the card gone.

### Post-drop interaction wiring (asserted)

Markup a DROP re-rendered arrives through the drag's own fetch, not
through an htmx request, and htmx 2.0.6 auto-processes nothing it did not
swap itself (the MutationObserver auto-scan is gone) — so render.go's swap
site calls `htmx.process` on the swapped region. The scenario 3
cross-check pins this: Edit Save and Delete on fetch-injected cards take
the same one-request htmx path as htmx-swapped markup — no native form
navigation, no inert controls. (Before the fix this was a witnessed
product bug: raw `innerHTML` left post-drop controls unwired until the
next full render — see `__log__/2026-10-06-kw5-e2e-drag.md` and
`__log__/2026-10-06-kw5-drag-swap-htmx-fix.md`.)
4. **Operation on missing card — move leg** — staged with two browser
   contexts on one server: context A deletes the card through its Delete
   control, context B's never-refreshed page stays genuinely stale and still
   shows the card as draggable. Dragging that stale card attempts EXACTLY
   ONE PATCH; the contract answers 404; the shared `#missing-card` surface
   states "no such card" over the re-rendered truth (the page under the
   banner IS GET /board — nothing faked, nothing moved), and the statement
   dies at reload.
5. **Abandoned drag changes nothing** — a drag that hovers the gutter and a
   column but releases outside every column (over the page heading) issues
   ZERO PATCH requests — dragover outside a column is never accepted, so
   drop and the single fetch site are unreachable from it. The board is
   unchanged in DOM and GET /board, no reload, no drag chrome left behind.

### Known product bug this lane witnessed (not asserted)

The drag's fetch swap assigns the board fragment with plain `innerHTML`
(render.go), and the bundled htmx 2.0.6 removed MutationObserver
auto-processing — so EDIT and DELETE controls on markup a DROP re-rendered
are left unwired until the next full render: an edit Save there submits
natively as a page navigation (`GET /?title=...`, edit discarded) and a
Delete click does nothing. The lane witnesses this verbatim in its run
output (scenario 3 cross-check) and kw3/kw4 stay green because their swaps
go through htmx, which processes what it swaps. The fix belongs to the
drag swap site (`htmx.process` the swapped region, or route the fetch answer
through `htmx.swap`) — see `__log__/2026-10-06-kw5-e2e-drag.md`.

## Seeding

`e2e/testdata/` is a go tool (test-support, not served application code;
the go tool and archspec ignore testdata dirs): it seeds a kanban data file
through the board store's Create — the seed primitive — and places cards
into In Progress / Done directly in the data file until the store's move
operation lands at KW5. Cards created there keep stable ids and contiguous
per-column positions.

Binaries land in `e2e/bin/` (git-ignored via the scripts' `mkdir -p`).

## Visual regression

No pixel lane exists after the pivot: the committed baselines all depicted
the retired todo list renders and were deleted with their specs. The
visual-regression lane is restored at KW7 with kanban baselines (card
ui/13, `workplans/dependencies_kanban.md` §KW7).
