```json
{
  "status": "done",
  "task": "KW6 wave end — e2e lane kw6-lifecycle (restart + migration + in-flight)",
  "wave": "kw6",
  "base": "349e75c8e3683e122960595170d84b455564193a"
}
```

# KW6 wave end — e2e/kw6-lifecycle.js

New lane re-executing the KW6 wave-end parent scenarios "Board survives
server restart", "Migrate existing todos on first start" and "Repeat
activation while an operation is in flight" against the composed process in
real chromium, plus the browser-bound proofs ui/inflight_test.go deferred to
this lane. kw1–kw5 untouched; Makefile gains the sixth lane line; e2e/README
gains the lane section.

What changed and why:

- `e2e/kw6-lifecycle.js` — four scenarios, each leg with its own temp dir,
  port, server (SIGTERM teardown) and fresh page, per the kw1–kw5 idiom.
  Every spawn carries an explicit `--todo-db` (temp fixture or
  never-created path); the repo-root todos.db is never a target.
  Scenario 1 creates/edits/drags on a live board, pins SIGTERM to exit
  code 0, respawns on the same address + board file, demands the revived
  page ≡ pre-restart board ≡ fresh GET /board, and proves the window with
  one further create (exactly one POST, contract 201). Scenario 2 builds
  the superseded todos.db via sqlite3 CLI with the schema verbatim from
  git 48a4ca5^, rows interleaved across done states with two deleted
  mid-sequence so source ids are gapped [1,2,4,5,7]; the board must carry
  them in creation order with fresh contiguous ids [1..5], the fixture's
  sha256 must be identical before and after every start (mode=ro reader),
  and a restart must not re-import. Scenario 3 holds one request
  in flight via page.route with a deferred route.continue() — the
  held-URL array is the in-flight witness — across four legs: dbl-clicked
  Delete = one DELETE; second Enter mid-POST = silent; drag attempted
  mid-PATCH = no second PATCH then a third legit drag succeeds; out-of-band
  removal → one 404 → zero [disabled] controls → next delete succeeds.
  Scenario 4 mirrors cmd/todo/interrupted_import_test.go at process level
  (no browser): poisoned row → non-zero exit, stated stderr, at most an
  empty board file.
- Instrumentation is kw5's request-counter pattern extended with response
  statuses and the route-hold; id matching accepts both /ui/cards/{id} and
  /ui/cards/{id}/move — the first draft's endsWith predicate silently
  missed every move PATCH, which is what leg c caught at run time.
- Drag landing targets are measured LIVE mid-drag (2px dragover nudge then
  re-measure): until KW7 the columns stack vertically without board CSS,
  so pre-drag bounding boxes diverge from mid-drag layout by ~a card row
  and releasing into a stale gap abandons the drag with zero requests —
  the exact silence that made the first runs fail. Tall viewport (1400px)
  as auto-scroll defense on drag pages.
- Two expectations were wrong about the contract, both corrected against
  source rather than asserted: browser create success is 201 (ui/create.go
  writes StatusCreated), not 200; the move PATCH endpoint is
  /ui/cards/{id}/move, not /board/cards (that path is the JSON API).

Verification: make e2e green on all six lanes (kw6 prints four scenario
OKs); go test ./... green. No product bug witnessed — every asserted
behavior held once the lane's own geometry and status facts were fixed;
the kw5 README's stale post-drop-wiring bug note was left untouched,
outside this lane's scope.
