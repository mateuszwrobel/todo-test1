```json
{"task": "kw8 lane 2 — ui done-freeze affordance: no edit affordance on Done cards (delete + drag stay), forced-PATCH refusal surfaced at the card via attachEditError, tests/gallery/baselines evolved", "status": "done", "date": "2026-10-07", "workplan": "workplans/workplan_kanban_application.md", "base": "61219f7", "prev-lane": "__log__/2026-10-07-kw8-done-freeze.md"}
```

What changed and why:

- **The affordance rule (boardTmpl, render.go):** the edit band and its Edit
  control now render under `{{if not .Done}}` — a card whose column is Done
  carries its title, its delete control, and its drag hooks, nothing else.
  Non-Done cards' markup is byte-identical to before (the branch adds no
  bytes on that arm); the refusal paragraph (`{{if .EditError}}`) deliberately
  stayed OUTSIDE the branch so a forced PATCH refusal still paints at the
  card. The contract refusal stays the guarantee behind the absent control:
  forcing PATCH /ui/cards/{id} on a Done card answers 422 and the stated
  "cannot edit a done card" surfaces at the card through the existing
  attachEditError path in handleEdit's 422 arm — the same error-at-card
  machinery as the blank/over-limit refusals, zero new ui wording, zero new
  code paths (the arm already existed from lane 1's api mapping).
- **Comments corrected** where lane 1 left them lying: edit.go's surface
  comment (was "every card in every column — Done included, no frozen-done")
  and render.go's boardTmpl comment (was edit band "including Done") now
  state the freeze restored 2026-10-07, the delete/drag legs staying
  all-columns, and drag-out as the unlock. handleEdit's doc names the three
  422 classes (blank, over-limit, done-frozen).
- **Tests evolved honestly, all at the affordance's seam:**
  edit_test.go `TestEditBandOnEveryCardEveryColumn` →
  `TestEditBandOnEveryCardOutsideDone` (bands pinned on cards 1–3, absence
  pinned on Done card 4 with its delete + drag hooks present);
  `TestEditDoneCardKeepsDoneTreatmentInPlace` →
  `TestEditDoneCardRefusedStatesAtCard` (forced form PATCH on the Done card:
  422 mirrored, contract string verbatim at the card, original title intact,
  board unchanged through the store's List, done treatment + place kept, plus
  the unlock leg — a column-only move-out then the same edit lands 200).
  page_test.go's affordance-wiring sweep and inflight_test.go's markup-seam
  pin lost their "Done included" edit arms the same way (the Done card has no
  save control to in-flight-block; delete's self-block stays everywhere).
  The move/drag legs needed nothing: lane 1 proved Move never sees a title.
- **Gallery (stories.go + stories_test.go):** the `c-card-done` component
  example lost its Edit button (delete stays — the pair with `#c-card` now
  shows both arms); the composite done cards follow automatically since the
  examples render through boardTmpl. Tests gained the frozen-state pins:
  state-board keeps bands for 901/902 and renders no `hx-patch` for the done
  903 (delete pinned present), and the done component example is pinned
  edit-affordance-free.
- **Visual baselines:** one anchor actually moved — `live-card-done.png`
  (the live seeded Done card at 286px column width: with the Edit button gone
  the element box is 69px tall, was 92px; every other anchor's pixels
  unchanged because the controls are opacity-gated at rest). Regenerated
  alone via `--update-snapshots -g 'done treatment on the live board matches
  its baseline'`; `make e2e-visual` full then ran back-to-back twice — 28
  passed each time.
- **Left for the next lane, untouched here:** `e2e/kw3-edit.js` (edits a Done
  card in the browser, asserts the done treatment survives the edit — would
  now find no `.card__edit` on it) and `e2e/kw5-drag.js` (its Done-leg
  cross-check uses `editCardTitle` on a Done card). Their go-test counterparts
  are green; the browser legs are lane 3's.
- **Verification:** `go test -count=1 ./...` entirely green — the known
  lane-1 red (`TestEditDoneCardKeepsDoneTreatmentInPlace`) flipped to the
  refusal pin; gofmt + `go vet ./...` clean; `archspec verify --strict`
  green.
