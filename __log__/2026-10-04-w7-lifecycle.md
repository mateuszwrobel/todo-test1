```json
{
  "task": "W7 — lifecycle (reopen durability, crash durability, restart, clean shutdown)",
  "status": "done",
  "date": "2026-10-04",
  "workplan": "workplans/dependencies.md#W7",
  "cards": ["todos/12", "todos/14", "server/03", "server/04"],
  "base": "eb70878",
  "card_commits": {
    "todos/12": "3c4b00d",
    "todos/14": "3d12588",
    "server/03": "8db7690",
    "server/04": "a2d1613"
  }
}
```

W7 delivered lifecycle vertically: store-level durability (reopen, crash), then the composed command's restart story and graceful shutdown, each card TDD from its scenario card, committed at its green; the parent scenario "Todos survive server restart" re-executed end to end at wave end.

What changed and why:

- **todos** — the two durability contracts needed no store changes: committed-before-return is what the store already promised (W1 decision), so both cards landed as acceptance tests. `reopen_test.go` seeds a mixed done/not-done mix through real Create/Change operations, closes, reopens the same file, and requires List equality — including that the reopened handle keeps accepting operations. `crash_test.go` kills genuinely: a child copy of the test binary performs Create and Change operations, reports completion on stdout, then SIGKILLs itself — no Close, no deferred cleanup, no exit path runs. The parent verifies the child died by SIGKILL specifically (a clean exit would falsify the card), then reopens the file and requires every completed operation present. SQLite's committed transactions are the mechanism the workplan names; nothing in the store had to change for it to hold.
- **server** — `restart_test.go` drives the real binary: seed → start → toggle over HTTP → stop → start again on the same path, asserting the JSON contract is unchanged across the restart and the page shows the same rows with the same states (title + data-state parsed from the rendered list). `shutdown_test.go` proved red first (SIGTERM default-killed the process mid-request), then the wiring landed: SIGINT/SIGTERM via `signal.NotifyContext` stops the serve loop, `http.Server.Shutdown` drains in-flight requests within a bounded 5s window (past the bound the remaining connections are force-closed and the store still closes — the workplan's stated risk mitigation), Serve is confirmed stopped, and the deferred store Close runs after the drain, so completed operations are durable and the process exits 0. The in-flight test is deterministic in both directions: a PATCH's body is deliberately completed only after the signal, so a non-graceful implementation cannot pass (the handler sits blocked on the body when shutdown starts, and the connection dies with it).
- **main.go hunks stayed minimal** per the conflict discipline: one import-block change plus replacing the `http.Serve` call with the serve-goroutine/select/shutdown block. The fixed wiring order, bind-before-construct baseURL, and both `/todos` + `/todos/` mount lines are untouched; startup-failure exit-1 paths (server/05) are unchanged — the goroutine only exists once the listener is bound.
- **e2e** — `e2e/w7-lifecycle.js` + `make e2e-w7`: scenario 1 runs the parent J1 story in real processes — mixed mix seeded, one toggle through a live checkbox click, SIGTERM stop, start again on the same db, JSON byte-equal plus the reloaded page showing the same texts/states; scenario 2 splits a PATCH body across the SIGTERM (raw socket), asserts the response still completes with 200, exit code 0, the listener stops, and a fresh start on the file shows the completed change.

**Note on W2 pending replay:** POST /todos is not mounted in main yet, so the restart cards drive mid-run state changes through the toggle PATCH over seeded rows — the same stand-in e2e/w3 and e2e/w5 use. When W2 lands, POST-created todos exercise the identical durability path; no W7 behavior depends on the stand-in.

Wave-end gates all green on this tree: `go vet ./...` + `gofmt -l` clean, `go test ./...` green (`-count=3` on the two touched packages for the signal tests), `archspec verify --strict` green (4 modules, 1 constraint), `make e2e-w7` green, and `make e2e-w1` / `e2e-w3` / `e2e-w5` re-run green — the shutdown wiring broke nothing landed earlier.
