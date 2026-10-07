```json
{"task": "pre-commit schema-diagram gate — .githooks/pre-commit blocks a commit when docs/db-schema.md drifts from what cmd/db-diagram generates from the working-tree code; make hooks installs it into the shared git hooks dir (gates every worktree); reviewer.md gains a Database changes checklist", "status": "done", "date": "2026-10-07", "base": "46ef139"}
```

What changed and why:

- **.githooks/pre-commit** (new, mode 755): the schema-diagram gate. Resolves
  the repo top level (`git rev-parse --show-toplevel`) and cd's there, then
  bootstrap-tolerates: if `cmd/db-diagram/` or `docs/db-schema.md` is absent it
  exits 0 silently (nothing to compare yet). Otherwise it regenerates into a
  `mktemp` file via `go run ./cmd/db-diagram -out "$tmp"` — from the WORKING
  TREE, not the committed diagram — so a schema edit is caught whether or not
  the author regenerated. A generation failure fails loudly with the
  generator's own stderr (go run passes it through); a `cmp -s` mismatch fails
  with the message. The hook never mutates the tree; the fix is
  `make db-diagram` + stage the result. POSIX sh, `trap`-cleanup on the tmp.
- **Makefile `hooks`** (new, standalone): installs `.githooks/pre-commit` into
  `$(git rev-parse --git-common-dir)/hooks/pre-commit` at mode 0755. Because the
  destination is the COMMON git dir, one install gates commits in EVERY
  worktree (hooks are process-of-the-repo, not per-worktree). `git config
  core.hooksPath` is left untouched — git discovers the hook at its default
  common-dir location. Wired to nothing else.
- **.agents/agents/reviewer.md**: added a "Database changes" section — trigger
  (diff touches schema facts: board/store.go schema const, anything the
  generator introspects, or docs/db-schema.md), a labeled checklist (PK sane;
  FKs only where intended and matching diagram arrows; indexes match real
  query/ordering needs — flag missing + weird ones; constraint comments
  faithful; no smuggled structural change), and the rule that any oddity is a
  finding the coder must prove by design (scenario / comment / ADR); "it works"
  is not accepted. `.opencode/agents/reviewer.md` is a symlink to this file and
  was not touched.

Design honesty: regeneration reflects working-tree code, so the gate catches a
schema edit even when the committed diagram looks untouched — that IS the
design, not a false positive. The gate is silent by design on a fresh diagram;
`GIT_TRACE=1` confirms the hook executes on every commit (schema-relevant or
not) from the shared `.git/hooks/pre-commit`.

Proof (SCRATCH throwaway worktree at 46ef139, hook installed in the shared
common dir; scratch removed after — never in the main checkout):

- Block — schema edit, diagram NOT regenerated:
  `sed -i '/check ("column" in/a\\tpinned integer not null default 0,' board/store.go`
  then `git add board/store.go && git commit -m ...` →
  ```
  stale: schema diagram does not match generated output — run 'make db-diagram' and stage docs/db-schema.md
  ```
  exit 1; HEAD unchanged (46ef139). (An earlier malformed-schema attempt blocked
  via the loud generation-failure path — surfaced `db-diagram: ... SQL logic
  error: near "pinned": syntax error` + `pre-commit: db-diagram failed to
  generate the schema diagram`, exit 1 — the generation-failure branch works too.)
- Pass — regenerate + stage: `go run ./cmd/db-diagram && git add docs/db-schema.md && git commit -m ...`
  → committed (working-tree `pinned` column now in the diagram), exit 0.
- Pass — non-schema commit, fresh diagram: `echo ... > file && git add file && git commit -m ...`
  → committed, exit 0; `GIT_TRACE` shows `start_command: .../.git/hooks/pre-commit`.
- Bootstrap — `docs/db-schema.md` temporarily removed: commit passes silently.
- Install works: `make hooks` printed `installed pre-commit ->
  /home/mateuszw/projects/www/todo-test1/.git/hooks/pre-commit`; the scenario
  commits above all executed through that installed hook.

Acceptance: go build/vet clean; `go test -count=1 ./...` green (todo/api,
board, cmd/todo, ui); `make db-diagram` byte-identical across back-to-back runs
and equal to the committed diagram. Final tree: `.githooks/pre-commit` (+x),
Makefile (+hooks), `.agents/agents/reviewer.md` (+section), this entry — nothing
else.
