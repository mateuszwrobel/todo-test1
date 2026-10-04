```json
{
  "task": "W5 replay — delete onto main after W3 landed",
  "status": "done",
  "date": "2026-10-04",
  "wave": "W5 (replay of 0cac87a..2074ff0, originally based on 49a3e22)",
  "base": "26cc902 (W3 toggle landed)",
  "cards": "todos/10, todos/11, api/10, api/11, ui/13 + e2e harness",
  "card_commits": {
    "w5-log-in-progress": "10a553d (orig 0cac87a)",
    "todos/10": "b619cc7 (orig b609971)",
    "todos/11": "c2e57b8 (orig 289e320)",
    "api/10": "e3b55b8 (orig 8850818)",
    "api/11": "452d5bf (orig 6c1909e)",
    "ui/13": "1ae6bcc (orig 7d3deff)",
    "e2e-harness": "16ee896 (orig 181382b)",
    "w5-log-done": "1fee81f (orig 2074ff0)"
  }
}
```

Replay of the verified W5 delete wave onto main after W3 landed the same
shared surface. Cherry-picks of the original card commits, oldest→newest,
one commit per card; the original log entry
(`__log__/2026-10-04-w5-delete.md`) came across untouched with its two
docs cards, and its merge notes proved exact — every predicted overlap
materialized and resolved as that entry anticipated.

Resolutions, all converging on "final state = W3 + W5 coexisting":

- **todos/delete.go** — dropped the card's local `ErrNotFound` declaration
  and converged on the module's single sentinel in `change.go`, whose doc
  comment was generalized from Change-specific to the shared not-found
  outcome. Same value, same matching semantics; no second source of truth.
- **api/delete.go** — dropped the card's `writeErrorJSON` helper in favor
  of W3's `errorJSON` (identical contract error shape); call sites now ride
  the one helper with the unused json import removed.
- **api/handler.go** — kept both waves' appends: the port declares List +
  Change + Delete, the mux registers GET + PATCH + DELETE.
- **cmd/todo/main.go** — the `/todos/` mount line deduped (W3 added it
  already); only its comment broadened to name both item operations.
- **ui/handler.go** — kept both fragment routes (PATCH toggle, DELETE);
  **ui/render.go** merged cleanly: W3's checkbox htmx wiring and W5's
  Delete-button wiring coexist in the one row template.
- **Makefile** — kept both append-only targets (`e2e-w3`, `e2e-w5`) behind
  the shared `e2e-w1`; README gained only W5's product section (W3 never
  touched it, e2e/README.md kept W3's harness section untouched).

Verification on the final replayed tree: `go vet ./...` clean, `gofmt -l`
clean, `go test -count=1 ./...` green across all four packages, `archspec
verify --strict` green (4 modules, no new import edges — ui still reaches
api HTTP-only), and all three browser suites green on the merged binary:
`make e2e-w1`, `make e2e-w3` (the W3 regression on the coexistence tree),
`make e2e-w5`.
