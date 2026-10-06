```json
{ "status": "done" }
```

# KW5 gap-close — same-column downward drop leg + change.go read-then-move note

Verifier WARN+INFO from the KW5 review. Two small closes, no new behavior:

- `e2e/kw5-drag.js` — new scenario 2b: the DOWNWARD leg of "Drag reorder
  within a column". Upward drops (scenario 2) release above the dragged
  card's slot, so dropPosition's skip-own-slot test never subtracts — the
  downward leg is the branch where counting-past-own-slot actually decides
  the payload: three rendered cards sit ahead of a below-the-last-card line,
  one of them the dragged card's own slot, and the body must carry position
  2 (index-after-removal), not the raw count 3. Asserts exactly one PATCH,
  DOM lands [B, C, A], fresh GET /board order/position agreement, no drag
  chrome residue; the reload proof stays scenario 2's (already proven).
- Geometry note (why the release point is computed off the card's bottom
  edge, not an fy on it): once the indicator sits anywhere above the last
  card it reflows that card ~14px down (2px line + 2×6px margins), and a
  fixed fy on the card then holds a stable above-midpoint equilibrium —
  the first attempt witnessed the line parked between cards 2 and 3 and a
  position-1 payload. The proven mid-viewport technique generalized: a
  clientY past the last card's bottom edge, clamped inside the section,
  lands past the midpoint in every indicator layout, so the line parks
  after the whole list deterministically.
- `api/change.go` — one honest comment on the position-alone leg's
  read-then-move window: a column change racing between the List lookup
  and Move resolves last-write-wins per the parent contract, and the
  single-conn store (board/store.go MaxOpenConns(1)) bounds the
  interleaving. Comment only, no behavior change.

Gates: make e2e fully green (kw5 lane now 7 scenario lines, 2b new), go
test ./..., gofmt/vet clean, archspec verify --strict matches the model.
