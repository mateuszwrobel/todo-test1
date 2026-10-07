```json
{
  "status": "done",
  "task": "kw10 filter increment: board filtered reads + filtered-slot move; api GET /board ?assignee= + PATCH slot/within pair",
  "cards": [
    "workplans/scenarios-kanban/board/19-filtered-slot-move-keeps-whole-board-order.md",
    "workplans/scenarios-kanban/api/14-board-filtered-by-assignee.md",
    "workplans/scenarios-kanban/api/15-move-with-a-filter-relative-slot.md"
  ],
  "base": "decabcf"
}
```

What changed and why.

Board (new file `board/store_filter.go`, beside the store it extends): the filter
keyword contract — exact roster name validated through `users.IsMember`, or the
`UnassignedKeyword` sentinel "unassigned" for the NULL rows — spelled once in
`filterCondition` and shared by both operations. `ListFiltered(keyword)` is List
with the keyword's SQL condition per column (`assignee = ?` / `assignee IS NULL`,
the amendment's exact spellings): same fixed three-column shape, matching cells
only, stored absolute positions passed through unchanged (gaps included — the
filter cuts cells, rewrites nothing). `MoveFiltered(id, column, slot, keyword)`
resolves a slot counted among the matching rows of the target column to the
EARLIEST absolute index with exactly that many matching cards before it: the
card is removed first (Move's semantics), the slot clamps to the remaining
matching count, and index 0 answers both "slot 0" and "no matching cards at
all" — so the amendment's "slot 0 into a column with no matching cards places
the card at that column's front" falls out of the one formula rather than a
special case (this reading is the one the front-edge is consistent with; the
alternative — insert before the slot-th match — contradicts it whenever hidden
cards sit before that match). Contiguity renormalizes whole-column; non-matching
cells keep their relative order by construction, and every probe pins it.

Design choice worth the orchestrator's eye: `MoveFiltered` takes the target
column as a DIRECTION (`*Column`, nil = the card's current column, Change's
convention) rather than resolving "no column" at the transport. Reason: the
amendment ranks roster validity over not-found, and a transport-side
read-current-column-then-move window would answer 404 for a stranger-"within"
beside a missing card, breaking the rank. Resolving inside the transaction
keeps the whole ordering — keyword, enum, slot, existence — in one place
(`8e88aa7`). New typed errors: `ErrUnknownAssignee` reused for unknown filter
keywords (one owner for "outside the roster", board/17's rank extended), and
`ErrInvalidSlot` for the negative-slot guard (board-style: guard before Begin,
board/19's totals-like sibling Move instead clamps — the filtered slot is a
count and a negative count is a request defect).

API: `handleBoard` branches on the PRESENCE of `?assignee=` — absent is the
shipped `store.List()` call, code-identical (byte-frozen payload, pinned);
present is `ListFiltered` with the value verbatim, mapping board's
`ErrUnknownAssignee` to the contract's stated 422 `{"error":"unknown user"}` at
the `errorJSON` site (present-but-empty is a present value outside the roster →
422). `handleCardChange` gained the pair: `"slot"` parses with position's shape
rules (non-integer → 400; a well-formed negative travels to board, where
`ErrInvalidSlot` → 422 "invalid slot" through the one `writeChangeError` site)
and `"within"` is a plain string field never screened here. Pair-shape refusals
are body-shape rules, so the handler states them (api/09's precedent):
position+slot → "position and slot are mutually exclusive"; half a pair →
"slot requires within"/"within requires slot"; an edit direction beside the
pair → "cannot combine slot with title or assignee" — the cards' contract
spells the pair as a MOVE (api/15 + the workplan amendment), names no
slot+edit combo, and the lane instruction says refuse rather than invent one.
Routing is the single `applyPatch` switch (additive `case slot != nil` first;
all pre-kw10 legs untouched, so every frozen absolute-position and
assignee/title leg routes operation-for-operation as before).

Frozen legs: no-param GET byte-pinned against the same fixture that the filter
legs probe; absolute-position PATCH legs re-pinned alongside the pair; existing
suites green unchanged; ui untouched (its filter work is lane 2; the port
widening touched no ui code — `api.NewHandler(store)` consumes the concrete
store).

Links: base decabcf (main tip at lane start). Gates at final commit: go build,
go test -count=1 ./... all green (board, api, cmd/todo, ui, users), archspec
verify --strict ok, gofmt/vet clean. No make e2e (wave-end verifier's call).
