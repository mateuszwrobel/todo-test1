```json
{"task": "KW2 ui lane — cards ui/04, ui/05 (create appends without reload; rejected create states the reason)", "status": "done", "date": "2026-10-06", "workplan": "workplans/workplan_ui_board.md", "ledger": "workplans/dependencies_kanban.md#kw2"}
```

What this lane changes and why.

The create control joins the page and becomes the board's first live mutation. render.go's shell comment already reserved the spot ("the create band joins it with card ui/04"); the band was never on the KW1 page — the earlier lane's note that it "already exists" matches the mockup-era plan, not the tree, so ui/04 adds it for real: an input and an Add button wired through htmx exactly where the retired todo app had them (git 1ff09e5^:ui/create.go), retargeted from the list area to `#board-area`.

Why the htmx fragment pattern rather than anything new: the repo's proven no-reload mechanism is a swap fragment from a ui fragment endpoint over the api contract (the workplan's re-render-from-server decision — the swap content comes from a fresh board read, never a local guess). `POST /ui/cards` performs `POST {apiBase}/cards` over real HTTP, then answers either the fresh board fragment (+ an out-of-band reset of the create area, so input and button are ready for the next card) on 201, or the create area carrying the contract's verbatim reason on 422 — the contract's status is mirrored so the page's error path stays true to the contract, and the shell's `htmx:responseError` routing (carried over, create arm only) swaps those 4xx bodies through the same engine. ui still imports zero project packages; the new ui→api coupling is HTTP-only, so archspec edges are unchanged.

Composition gap flagged by the api lane: cmd/todo never mounted `/cards` (KW1 mounted only `/board`). The mount joins the JSON-contract block beside `/board`, same one-line style as the existing `/todos` mounts; without it the contract exists only in-process and the page's create would 404 through the listener.

Rejections state the board/api's own strings ("title is required", "title exceeds the 500 character limit") at the create control — surfaced verbatim, never re-worded; the board stays untouched because the rejection response carries no board fragment at all (no phantom card is even expressible). The typed text survives a rejection, kept from the retired app's create behavior (journeys leave input survival open; preserving is the retired precedent).

Double-submit posture matches the retired app exactly (`hx-disabled-elt` on the submit button); the full in-flight blocking contract is ui/12 (KW6), not claimed here.

Tests chain real handlers: a real board store in a temp file behind a real `api.NewHandler` over httptest, with the ui handler pointed at it — same real-chain style the api lane's create tests use, since a canned fake could not prove the 422 strings flow through unchanged. Page-wiring assertions follow KW1's ui test style. The cmd lane gets a mount probe (POST /cards through the composed listener answers 201, not 404) beside the existing fresh-path surface assertions.
