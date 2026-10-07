```json
{"task": "kw9 lane 1 — users roster package (constant cast, membership) + board assignee column, Change assignee direction with validity-before-not-found-before-freeze ordering, done freeze extended to the assignee direction, reopen survival; mechanical Change plumbing through api/ui", "status": "done", "date": "2026-10-07", "workplan": "workplans/workplan_users_roster.md, workplans/workplan_board_store.md", "cards": "workplans/scenarios-kanban/users/01, board/15, board/16, board/17, board/18", "base": "1f83e24", "commits": "b936225 log, e29cfe6 users, a2c5b19 board, 17dd561 plumbing"}
```

What changed and why:

- **users (commit e29cfe6)** — the simulation's cast lands as its own bottom module
  exactly as the roster workplan placed it: one unexported constant list, `Names()`
  answering it in contract order (Ada, Grace, Alan, Barbara, Linus), and `IsMember`
  as the single case-sensitive membership rule. `Names` hands back a copy on
  purpose — the order is contract because every dropdown lists the cast in one
  fixed order, and a caller must not be able to rewrite what every later answer
  repeats; users/01 pins the order, the repetition, and the copy. No state, no
  I/O, no imports: replacing this with real accounts later means rewriting this
  file and keeping the two-function contract. The spec registers `users` as a
  bottom module (nothing allowed below it, every app module forbidden from it in
  reverse), adds it to `no_cycles`, and states the KW9 boundary in `ui`'s
  forbidden list — names reach the page over the contract only.

- **board (commit a2c5b19)** — one nullable `assignee` column, one new direction on
  `Change`, one new typed outcome.

  The stored value is NULL for unassigned and the roster name otherwise; no CHECK
  and no foreign key, because the cast is program data (users), not a table —
  membership is a code rule. `Open` upgrades a pre-assignment file in place via
  `pragma_table_info` + `ALTER TABLE ADD COLUMN` of a nullable column: every
  existing row reads as unassigned, no row is rewritten, no migration framework
  enters a two-table store. The reopen test proves the upgrade against a file
  hand-created in the old schema, not against the current schema string.

  Two representation choices, both documented where the code states them. Read
  side: `Card.Assignee` is a plain string with empty meaning unassigned. The
  roster contract excludes the empty name, so the sentinel can never collide with
  a real assignee, and keeping `Card` comparable is what lets this package's
  cell-for-cell pins (`boardEqual`) keep comparing two listings with `==` — a
  pointer field would have made every assigned cell compare by address. The api
  mapping to name-or-null is one line (`!= ""`), and the field's `omitempty` tag
  keeps today's api payloads byte-identical while every card is unassigned; the
  api lane's contract leg replaces that with the explicit null. Direction side:
  `*AssigneeDirection` (`AssignTo` / `ClearAssignee`) instead of a `*string` —
  absent, set, and clear are three states and the store contract should not
  overload the empty string to name the third one, which the roster contract says
  is not even a name.

  Rule order inside `Change` is the card's rank, quoted verbatim in the function's
  comment from the board workplan: roster validity first (so an unknown assignee
  answers beside blank text, an over-long one, a missing card, an invalid column,
  and a done card — board/17 pins each), then the text rules, then the column
  enum, then — inside the transaction — not-found on the existence read, then the
  freeze against the CURRENT column. `ErrUnknownAssignee` is worded in the
  existing store-outcome voice ("board: assignee is not on the users roster") so
  the next lane's api mapping has one class to key on.

  The freeze reuses `ErrDoneFrozen` rather than minting an assignment-shaped
  error: the rule is one rule ("a Done card changes nothing but its column and
  its existence"), and a second error name would invite the api to word one
  decision two ways. Setting and clearing are both edits — the freeze does not
  grade them — and carrying the card out of Done in the same call still refuses,
  because the check reads the row it read for existence. Move, Create, Seed and
  Import are untouched by design: a move places, fresh rows start unassigned.

- **plumbing (commit 17dd561)** — mechanical only, no behavior, no tests: the
  `api.BoardStore` port's `Change` method gains the fourth parameter (with a note
  that today's handlers pass nil, which is the contract's "leave it alone"), the
  three `store.Change` call sites in `api/change.go` pass nil, and `ui`'s
  `moveTo` test helper passes nil. No api request parsing, no api response shape
  change, no ui change — api/12 and api/13 (assignee direction on PATCH,
  GET /users, the `?assignee=` filter) are the next lane's contract legs.

- **Bisect note**: the board commit alone leaves api/ui uncompiling (the signature
  evolves there, the call sites move in the next commit). The tree is green from
  17dd561 onward, which is where the lane's gates were run.

Verified on the final tree: `gofmt -l` clean, `go vet ./...` clean, `go build ./...`,
`go test -count=1 ./...` green — todo/api, todo/board, todo/cmd/todo, todo/ui,
todo/users all ok, every pre-existing board/api/ui/server test unchanged in
behavior (assignee is additive; api payloads stay byte-identical while unassigned),
and `archspec verify --strict` green with six modules (the board→users edge
declared where the import first appears, ui still forbidden from users). `make
e2e` deliberately not run: browser suites are wave-end (lane 4).
