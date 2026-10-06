```json
{ "status": "in-progress" }
```

# KW5 gap-close — same-column downward drop leg + change.go read-then-move note

Verifier WARN+INFO from the KW5 review. Two small closes, no new behavior:

- `e2e/kw5-drag.js` — add the DOWNWARD same-column drop leg (scenario 2b):
  3-card column, drag the TOP card to the BOTTOM (drop line below the last
  card). Upward drags (scenario 2) release above the dragged card's slot, so
  dropPosition's skip-own-slot test never subtracts — the downward leg is the
  branch where counting-past-own-slot actually decides the payload: the body
  must carry position 2 (index-after-removal), not the raw count 3. Asserts
  one PATCH, DOM order [B, C, A], fresh GET /board agreement. Reload proof
  stays scenario 2's.
- `api/change.go` — one honest comment on the position-alone leg's
  read-then-move window (concurrent column change between the List lookup
  and Move → last write wins per the parent contract; the single-conn store
  bounds interleaving). Comment only, no behavior change.
