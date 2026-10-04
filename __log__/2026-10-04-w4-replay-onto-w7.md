```json
{
  "task": "W4 replay — edit + frozen-done onto main after W7 landed",
  "status": "done",
  "date": "2026-10-04",
  "wave": "W4 (replay of 97735f2..4fb695b, originally based on bee01dd)",
  "base": "31d3346 (W7 lifecycle landed)",
  "cards": "todos/04, todos/06, todos/07, api/05, api/07, ui/09, ui/10, ui/11, ui/12 + e2e-w4 suite",
  "card_commits": {
    "w4-log-in-progress": "9f8ad3e (orig 97735f2)",
    "todos/04": "6a2b309 (orig 8ff8357)",
    "todos/06": "4fdb0fa (orig 0230c04)",
    "todos/07": "7285eab (orig 8ad37be)",
    "api/05": "203d075 (orig e03532d)",
    "api/07": "8195fde (orig 732176b)",
    "ui/09": "0922a9d (orig e6a47af)",
    "ui/10": "2d0aaa4 (orig e0b68c3)",
    "ui/11": "867f203 (orig cdc484e)",
    "ui/12": "fd260d3 (orig 9afe0d8)",
    "e2e-w4-suite": "1aaebf5 (orig 730473d)",
    "w4-log-done": "b41c57f (orig 4fb695b)",
    "gitignore-todos-db": "0658e8c (new, replay lane)"
  }
}
```

Replay of the verified W4 edit wave onto main after W7 landed the lifecycle
surface. Cherry-picks oldest→newest, one commit per card, same card messages;
the W4 lane's own log entry came across untouched with its two docs commits.
The lanes touched disjoint files except two EOF-append files, so everything
auto-merged except exactly those two.

Resolutions:

- **`Makefile`** (EOF conflict on the e2e commit, vs W7) — union: `e2e-w7`
  and `e2e-w4` targets both kept, appended after the existing w1/w3/w5/w2
  targets in landing order; the W4 block is byte-identical to the lane's.
- **`e2e/README.md`** (EOF conflict on the e2e commit, vs W7) — git aligned
  the two waves' section scaffolding ("## Run" / "## What it asserts") into
  three interleaved hunks; unioning emits two whole sections instead — W7's
  lifecycle section intact as on main, then the complete W4 edit section
  appended (verified byte-identical to the lane's section). No cross-wave
  staleness to correct this time: neither section references the other's
  surface as pending.

**`.gitignore` hygiene.** Root cause of stray db files appearing in the
checkout: the binary's default `--db` path is `todos.db`, so running it from
the repo root drops `todos.db` there; the ignore file only covered the
test-era `/todo.db`. Added `/todos.db` — one-line chore commit on the replay
lane, before the lane's log-done.

Verification on the final replayed tree: `gofmt -l` + `go vet ./...` clean,
`go test -count=1 ./...` green (4 packages), `archspec verify --strict`
green (4 modules, 1 constraint), and all six browser suites green:
`make e2e-w1` / `e2e-w2` / `e2e-w3` / `e2e-w4` / `e2e-w5` / `e2e-w7` —
the full regression re-executed once against the combined W1–W7 tree.
