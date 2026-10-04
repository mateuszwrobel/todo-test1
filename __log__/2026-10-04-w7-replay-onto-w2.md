```json
{
  "task": "W7 replay — lifecycle onto main after W2 landed",
  "status": "done",
  "date": "2026-10-04",
  "wave": "W7 (replay of 4643ef7..afb1bd4, originally based on eb70878)",
  "base": "bee01dd (W2 create landed)",
  "cards": "todos/12, todos/14, server/03, server/04 + e2e harness + flake fix",
  "card_commits": {
    "w7-log-in-progress": "8f24eae (orig 4643ef7)",
    "todos/12": "09740ea (orig 3c4b00d)",
    "todos/14": "a7a7dd3 (orig 3d12588)",
    "server/03": "39b58c3 (orig 8db7690)",
    "server/04": "30ac0b0 (orig a2d1613)",
    "e2e-harness": "cec7172 (orig fa37384)",
    "w7-log-done": "a91c59c (orig afb1bd4)",
    "shutdown-flake-fix": "85f276c (new, replay lane)"
  }
}
```

Replay of the verified W7 lifecycle wave onto main after W2 landed the create
surface, plus a determinism fix for a verifier-proven flake in the in-flight
shutdown test. **This entry supersedes the determinism claim in
`__log__/2026-10-04-w7-lifecycle.md`: its "deterministic in both directions /
`-count=3`" statement about `shutdown_test.go` no longer stands** — that test
was flaky on the merged tree (see below); the barrier added here is what
makes it deterministic. The W7 lane entry itself is left untouched.

Cherry-picks oldest→newest, one commit per card, same card messages; the W7
lane's own log entry came across untouched with its two docs commits.

Resolutions:

- **`cmd/todo/main.go`** — clean, no conflict (W2 never touched it): the W7
  serve-goroutine/select/`Shutdown` block landed over the intact W5-era
  wiring — both `/todos` + `/todos/` mount lines, the W2-era api handler
  already routing `POST /todos` internally, fixed wiring order, and the
  startup-failure exit-1 paths all preserved. Diff vs base is exactly the
  shutdown block plus its imports.
- **`Makefile`** (EOF conflict vs W2) — union: `e2e-w2` and `e2e-w7` targets
  both kept, both building `e2e/bin/todo`, W7 additionally the seed tool.
- **`e2e/README.md`** (EOF conflict vs W2) — union: W2's create section and
  W7's lifecycle section both kept in full. While unioning, W7's now-stale
  note ("POST /todos is not mounted yet, W2 pending replay") was corrected
  in the section and in the same sentence's script comment in
  `e2e/w7-lifecycle.js` — W2 is landed, so mid-run PATCHes are stated as
  riding the same route the browser's checkbox uses, not a stand-in for a
  missing mount. Same cleanup the W5 replay made for its cross-wave notes.

**Shutdown flake — root cause and fix.** The lane test signaled SIGTERM
immediately after writing the partial PATCH. net/http counts a connection
active only once its request is parsed and handed to the handler; a
connection whose headers the server hasn't read yet is *idle* to
`Shutdown`, which closes it. When SIGTERM won that race the connection died
and the test failed with `unexpected EOF` at the response read — reproduced
2 of 5 `-count=3` runs on this lane before the fix. The fix adds a causal
barrier instead of trusting timing: the PATCH now carries
`Expect: 100-continue`, and Go ≥1.19 net/http sends the `100 Continue`
interim response the instant the handler performs its first body read —
strictly after the request is registered active in Shutdown's accounting.
The test blocks on receiving that interim response, then signals; the fixed
100ms sleep is gone. The store drain afterwards is unchanged.

**Honesty proof by mutation:** with `main.go` temporarily reverted to the
plain `http.Serve` path (bee01dd's version, default SIGTERM kill), `-count=3`
is red all three iterations — `in-flight request did not get a response:
... read: connection reset by peer` — because the process now dies mid-request
and the connection resets. Restoring the graceful block → green. The test
still cannot pass against a non-graceful implementation.

Verification on the final replayed tree: `gofmt -l` + `go vet ./...` clean,
`go test -count=1 ./...` green (4 packages), the in-flight test green twice
at `-count=9`, `archspec verify --strict` green (4 modules, 1 constraint),
and all five browser suites green: `make e2e-w1` / `e2e-w2` (the W2
regression on the shutdown-wired tree) / `e2e-w3` / `e2e-w5` / `e2e-w7`.
