```json
{"task": "kw8 — contract-level done freeze: title edits refused on cards in Done (board Change + PATCH), docs and card amendments first; ui/e2e affordance pins left for the next lane", "status": "done", "date": "2026-10-07", "workplan": "workplans/workplan_kanban_application.md", "base": "81de81a"}
```

What changed and why:

- **The rule (user decision 2026-10-07, deliberate reversal of the pivot):** a card
  sitting in the Done column is not title-editable at the contract level. Drag-out
  is the only unlock. Delete, column-direction moves (out of and into Done), and
  Create/Seed/Import into Done stay allowed — none is an edit. This restores the
  todo app's frozen-text rule that the 2026-10-06 pivot had deleted; the freeze is
  derived from column membership alone, no new stored state (the pivot's one-
  source-of-truth decision survives untouched).
- **Docs first** (commit d7ada4b). New parent scenario **"Edit of a done card is
  rejected"** appended to workplan_kanban_application.md as scenario 15 — appended
  at the end deliberately: parent scenarios are numbered by order of appearance
  (workplan_ui_board.md cites "parent scenario 14"), so inserting mid-list would
  silently renumber every implemented scenario's parent reference. The one-line
  decision bullet (one update operation) carries the freeze with the supersession
  note. user-journeys_kanban.md J6 inverted honestly: the Goal sentence ("Unlike
  the todo app, a done card's text is editable...") replaced with the freeze +
  supersession citation, a done-rejection arm added to J6's alternate flows, the
  UI hook ("editing reachable on every card in every column") restricted to
  cards outside Done, and the API-calls line gained its 422 class. The board
  workplan's Change data-flow sentence gained the current-column freeze clause;
  workplan_ui_board.md gained the amended edit-affordance decision (Done cards
  carry no edit control; the contract's stated refusal is the guarantee that
  stays). The three contract cards (board/08, api/05, ui/06) keep their original
  text and append an "Amendment 2026-10-07" clause — cards are history. The
  dependencies ledger's "Done cards stay editable — no frozen-text rule survives
  the pivot" line is marked superseded rather than rewritten.
- **board** (commit 46e140b): Change grows one typed outcome, `ErrDoneFrozen`
  ("board: card is done; move it out of Done to edit"), checked inside the
  transaction right after the existence read and strictly before any write —
  against the card's CURRENT column, so a combined title+column change escaping
  toward todo still refuses while the card sits in Done (ordering quoted in the
  code comment). Move never sees a title, so drag-out stays a one-call unlock;
  Create/Seed/Import write fresh text and are untouched. board/08's done row in
  TestChangeTitleKeepsPlaceAndIdentity flipped to TestChangeTitleOnDoneCardIsFrozen
  (refusal cell-for-cell unchanged, combined title+column refused, column-only
  out-of-Done fine + edit-after-unlock, Seed-into-Done accepted); the column-move
  table's "stays editable" row renamed to name the freeze unlock, assertions
  unchanged.
- **api** (commit e31bf46): one new mapping case, board.ErrDoneFrozen → 422
  `{"error": "cannot edit a done card"}` at the single writeChangeError site.
  Status + shape mirror the retired todo app's frozen-todo convention verbatim
  in kind: it answered 422 "cannot edit a done todo" from the same one-site
  mapping of todos.ErrDoneFrozen (api/change.go at 8195fde; store error
  "todos: done todo, title is frozen"). TestPatchCardTitleOnDoneCardSucceeds
  flipped to TestPatchCardTitleOnDoneCardRefused — title-alone and
  title+column-together refusals byte-pinned (the todo app's leg shape, both
  bodies), board-unchanged probe, move-out-then-PATCH-title success leg; the
  flip cites the api/05 card amendment in the test comment. No-title legs —
  including every move leg — answer exactly as before; not-found still beats
  freeze (a nonexistent card cannot be frozen).
- **Left for the next (ui/e2e) lane, untouched here:** ui/edit_test.go
  TestEditBandOnEveryCardEveryColumn (edit band asserted on every card incl.
  Done — green but affordance-pinning), ui/edit_test.go
  TestEditDoneCardKeepsDoneTreatmentInPlace (edits a Done card expecting 200 —
  the lane's only red), ui/edit.go's surface comment ("Done included, no
  frozen-done"), ui/render.go's boardTmpl comment (edit band "including Done"),
  ui/stories.go's c-card-done gallery fixture (carries an Edit button), and
  e2e/kw3-edit.js (edits a Done card in the browser, asserts the done
  treatment survives the edit). The ui's 422 arm already surfaces the contract's
  stated refusal verbatim on a stale page — verified by the red pin's response
  body showing the refusal rendering.
- **Verification:** `go test -count=1 ./...` — board, api, cmd/todo green; ui red
  only on the named done-edit pin above; archspec verify --strict green; gofmt
  and `go vet ./...` clean.
