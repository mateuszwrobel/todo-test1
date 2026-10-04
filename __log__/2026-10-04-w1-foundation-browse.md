```json
{
  "task": "W1 — todo app foundation + browse (vertical slice: store → contract → page → composition)",
  "status": "done",
  "date": "2026-10-04",
  "workplan": "workplans/workplan_todo_application.md (+ workplan_todos_store.md, workplan_api_http.md, workplan_ui_page.md, workplan_server_composition.md)",
  "wave": "W1 (workplans/dependencies.md)"
}
```

What changed and why, per card (TDD from the verbatim scenario cards; one commit each):

- **todos/13** — `todos.Open(path)` opens/creates the SQLite file (pure-Go
  `modernc.org/sqlite`, ADR-002), creates the single `todos` table
  create-if-not-exists, and pins one pooled connection (single-writer
  assumption). A fresh path opens as an empty, fully working store; go.mod
  (module `todo`) lands here.
- **todos/01** — `Create(title)` returns the todo with a fresh autoincrement
  id (never reused), done false, committed before return. The card test added
  the never-reused-id and List-includes-it assertions; behavior came from the
  card-1 seed primitive plus autoincrement.
- **todos/03** — `List()` selects ordered by id ascending — creation order is
  the identifier order, per the parent decision.
- **api/04** — `api.NewHandler` serves GET /todos as a 200 JSON array, empty
  list encoding `[]` not null. The api package declares its own minimal port
  (`TodoStore`) — consumer-defined, concrete store injected — so no concrete
  cross-module types.
- **server/01** — `cmd/todo` composition root: flags `--addr`/`--db` with the
  stated defaults, wiring order store → api handler → listener bind → ui
  handlers with the api base URL taken from the actually-bound address (exact
  even for ephemeral ports; no lazy wiring) → single listener mounting
  `/todos` (api) and `/` + `/static/*` (ui). The test builds and execs the
  real binary. Vendored htmx 2.0.6 at `ui/static/htmx.min.js`, embedded via
  go:embed and served at /static/htmx.min.js.
- **server/02** — absent db path starts successfully (store creates the file)
  and the page states the empty list. Behavior came from the store's
  create-on-open plus the empty surface; the test pins it end to end.
- **server/05** — address-in-use, unusable db path, and malformed flags each
  exit 1 with a stated reason; the listen-before-serve order guarantees
  nothing is left half-wired (verified by dialing the address after failure).
- **ui/01** — page rows render in contract order with title text, checkbox
  done state, delete on every row, edit only on not-done rows, and the
  always-ready create input + button. ui defines its own Todo DTO — it has
  zero internal imports; every read is a real HTTP GET to the injected api
  base URL (tests fake the api with httptest over HTTP, never an import).
- **ui/02** — distinct `#empty-state` ("No todos yet.") rendered only for a
  successful empty read, never as a blank page; create control stays ready.
- **ui/03** — transport failure or non-200 from the api renders the stated
  load-failure surface; the empty and list surfaces are unreachable on that
  path, so a failure never masquerades as empty or stale truth.
- **ui/04** — the failure surface carries a retry control (`<a id="retry"
  href="/">`); activating it re-issues the read — list rendered on recovery,
  failure restated while still down. Recovery stays inside GET semantics.
- **ui/15** — every GET / is a fresh full server render (no ui-side state),
  so a reload equals a fresh read; the test flips the api's state between
  loads and asserts rows and done states match exactly.
- **server/06** — archspec verify --strict green (encoded as a go test, plus
  a static import check: ui imports no internal module, the SQLite driver
  appears only in todos).

Integration bootstrap: `e2e/` Playwright script (chromium via global install)
starts the real binary on ephemeral ports with temp dbs — populated list
(order/checkboxes/control placement), fresh-db empty state, and
db-removed-while-stopped → start → empty state. Wired behind `make e2e-w1`;
`e2e/testdata` is test-support tooling (only place outside todos that touches
the file, mirroring test-scope exemption).

Deviations from the workplan: one micro-ordering note — the listener bind
happens before the ui constructors so the injected base URL is the actual
bound address (workplan's listed order puts listen last); lifecycle
ownership and the no-lazy-wiring decision are preserved. The W1 import scan
excludes `_test.go` files and `testdata` trees (the seed tool lives in
`e2e/testdata/` so archspec keeps the model at exactly the four declared
modules), consistent with "excluded trees are out of the model".
