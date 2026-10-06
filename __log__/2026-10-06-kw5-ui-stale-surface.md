```json
{
  "status": "done",
  "date": "2026-10-06",
  "cards": ["ui/11"],
  "workplan": "workplans/workplan_ui_board.md",
  "wave": "KW5"
}
```

# KW5 ui/11 — one stated-failure surface across edit, move, delete

Base: main @ 715d061 (kw5 ui drag cards landed).

## What changed and why

The three verbs already answered a stale 404 with the same body, but the
mechanism existed as three hand-copied arms inside `handleEdit`,
`handleDelete` and `handleMove`, each carrying forward-referencing seam
comments ("consolidation is card ui/11's"). This card closes those seams by
giving the surface a single owner instead of a third copy:

- `ui/stale.go` (new): the named stale-failure surface. `missingCardTmpl`
  (the `#missing-card` banner, moved verbatim from edit.go) plus
  `writeStaleFailure` — the one server-side answer every stale operation
  takes: the contract's own reason in the banner, over the board re-read
  from server truth (`writeBoardAreaFragment`), at the mirrored 404. Its
  doc comment states the one-surface contract explicitly, including why
  the 422 validation refusal is a distinct class that deliberately stays
  at the card (`attachEditError`) — this surface speaks only to
  operations on a card that no longer exists.
- `ui/edit.go`, `ui/delete.go`, `ui/move.go`: each 404 arm shrinks to one
  `p.writeStaleFailure(w, reason)` call; the duplicated render/sequence
  code and the pending-consolidation comments are gone. Behavior is
  byte-identical to the pre-card state — this is a consolidation, not a
  change. The 422 arms and default arms are untouched.
- `ui/render.go`: no behavior change — the client already converged (htmx
  `responseError` arms for edit/delete and the drag `fetch` all replace
  `#board-area` with the same fragment). Only the comments claiming the
  shared surface was still to come ("Drag refusals join at their own
  card", "card ui/11's seam") now state it has landed.

The failure surface in one sentence: 404 from any card verb →
`#missing-card` banner stating "no such card" above the board re-rendered
as exactly the server's truth, at status 404, swapped into `#board-area` —
same bytes, same element, same region, whatever control triggered it; a
reload reads the same truth, so nothing outlives the reload.

## Tests

- `ui/stale_test.go` (new): `TestStaleOperationsServeOneFailureSurfaceAcrossVerbs`
  — table-driven over the three real client seams (htmx form PATCH, drop
  fetch PATCH, htmx DELETE). Staleness is produced end-to-end through the
  real store: card renders, then `store.Delete` out-of-band, then the verb
  acts. Per verb: mirrored 404, the full banner element stated verbatim,
  no trace of the stale card, the truth present, the fragment's board
  byte-identical to a fresh page's board ("nothing else changes" pinned
  as board diff = exactly the truth), store untouched. Across verbs: the
  three fragments compared byte-for-byte — one surface, not three
  look-alikes.
- Per-verb pins evolved, not duplicated: the edit stale leg and the move
  stale leg now operate on a card deleted out-of-band from the real store
  (previously an id the board never had), pinning the true other-session
  scenario; the delete pins keep their two distinct arms (never-held id +
  delete-then-stale-click, the latter already store-end-to-end) with the
  comments pointing at the shared surface. All existing stale assertions
  stay green unchanged.

## Verification

`go build`, `go vet ./...`, `gofmt` clean, `go test -count=1 ./...` green,
`archspec verify --strict` ok. e2e kw3/kw4 select the failure via
`#missing-card` — markup untouched, compatible. api/board/cmd untouched.
