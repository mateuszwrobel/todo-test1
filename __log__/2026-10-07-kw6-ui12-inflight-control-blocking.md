```json
{
  "status": "done",
  "task": "ui/12 — Triggered control blocks while in flight",
  "card": "workplans/scenarios-kanban/ui/12-triggered-control-blocks-while-in-flight.md",
  "wave": "kw6",
  "base": "6af026e87d4007f6cd2af8cb0cd700e9f254816d",
  "commit": "7bcacf8"
}
```

# Triggered control blocks while in flight

Scope verdict — triggered-only, not board-wide. The card's scenario re-acts
"the same control" ("When the user triggers any card operation and triggers
the same control again before the response arrives"), and the journeys'
interaction decision names exactly one control per operation — "the create
input and its button, a card's edit-save, its delete control, or the card
being dragged — is disabled from the moment the request leaves until the
response arrives". The workplan decision line ("in-flight blocking covers
every card operation including the dragged card") is coverage of operation
types, not a board-wide lock: each operation blocks its own control.

What changed and why:

- htmx verbs block themselves declaratively. The create form's
  `hx-disabled-elt` was extended from the submit button to
  `#create-form .input, #create-form button[type=submit]` — a live input
  would re-submit on Enter while the request is in flight, breaking "only one
  request went out", and J2 states both parts disable. Each card's edit form
  gained `hx-disabled-elt="#card-{id} .save"` (the save control, per J6), and
  each delete control gained `hx-disabled-elt="this"` (self-block, per J7).
  Verified against the bundled htmx: disable runs synchronously when the
  request leaves (so a second click cannot slip between click and disable),
  and restore runs on every settled path — the onload leg covers 2xx and 4xx
  alike (the responseError swap is still a response), onerror covers
  transport failure, onabort aborts — with a per-element refcount. No page
  JS was needed for these three verbs; the mechanism the create surface
  already carried just spread to the other two.
- The drag fetch carries its own block, since it bypasses htmx. `drop`
  captures the card, sets `dragPending`, and clears `draggable` before the
  (still exactly one) fetch leaves; `dragstart` refuses to arm a drag on the
  pending card; a single `.finally` at the chain's end — after the catch that
  absorbs the transport leg — restores `draggable` and clears the pending
  state, so success, stated failure and network error share one ready-again
  path landing exactly at response arrival. A response whose swap replaced
  the markup makes the restore moot by construction: the fresh element
  renders draggable, which is the ready state. The one-fetch-site rule
  survives untouched — the chain was extended, not duplicated.
- Tests (`ui/inflight_test.go`): markup seams pin the disable attributes on
  every rendered control (create pair, per-card edit save, per-card delete
  self-block sitting on the hx-delete element, all columns incl. Done);
  script seams pin the mechanism structurally — one fetch, one finally,
  disable before the request, restore inside the finally after the catch,
  dragstart guard ahead of arming. Same honesty level as the existing
  fetch-count pins.

Deferred proofs (wave-end e2e lane, needs a live browser): double-clicked
Delete sending exactly one request; a dragged card refusing a second drag
until its PATCH answers; Enter-key implicit submission staying silent against
a disabled save button; htmx restore timing on 4xx paths in a real engine.
Server-side seams prove the mechanism exists and sits on the one settle
path; they do not race gestures.

Gates: go build / vet / gofmt clean, `go test -count=1 ./...` green,
`archspec verify --strict` ok.
