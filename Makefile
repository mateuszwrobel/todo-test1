# kanban application — wave tooling

# The e2e suite drives the composed server in a real browser (Playwright,
# global install — NODE_PATH resolves require('playwright')). Each wave's
# lane script re-executes that wave's parent scenarios; see e2e/README.md.
# The KW1 lane covers "Board shows fixed columns" + "Start without todo
# data"; the KW2 lane covers "Create card" + "Reject empty card text" +
# "Reject over-long card text"; the KW3 lane covers "Edit card text keeps
# place" + "Operation on missing card" (edit leg); the KW4 lane covers
# "Delete card" + "Operation on missing card" (delete leg); the KW5 lane
# covers "Drag card between columns" + "Drag reorder within a column" +
# "Done is column membership" + "Operation on missing card" (move leg, which
# completes that scenario); the KW6 lane covers "Board survives server
# restart" + "Migrate existing todos on first start" + "Repeat activation
# while an operation is in flight". The retired
# todo-app lanes (w1..w11) deleted with the pivot's wave-end swap; the
# visual-regression lane returns with KW7's baselines.
.PHONY: e2e
e2e:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/kw1-board.js
	NODE_PATH=$$(npm root -g) node e2e/kw2-create.js
	NODE_PATH=$$(npm root -g) node e2e/kw3-edit.js
	NODE_PATH=$$(npm root -g) node e2e/kw4-delete.js
	NODE_PATH=$$(npm root -g) node e2e/kw5-drag.js
	NODE_PATH=$$(npm root -g) node e2e/kw6-lifecycle.js
	NODE_PATH=$$(npm root -g) node e2e/kw8-freeze.js
	NODE_PATH=$$(npm root -g) node e2e/kw9-assign.js
	NODE_PATH=$$(npm root -g) node e2e/kw10-filter.js

# Visual regression — committed pixel baselines under
# e2e/visual/__snapshots__/ compared by Playwright's own toHaveScreenshot
# (e2e/visual.spec.js; servers seeded + started by e2e/visual-server.js).
# Regenerate deliberately:
#   make e2e-visual PW_ARGS=--update-snapshots   (review diffs, commit PNGs)
.PHONY: e2e-visual
e2e-visual:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node $$(npm root -g)/@playwright/test/cli.js test --config e2e/playwright.visual.config.js $(PW_ARGS)

# Schema diagram — regenerate docs/db-schema.md from the live schema:
# cmd/db-diagram materializes the schema through board.Open on a throwaway
# file (board owns the schema SQL; the tool copies none) and renders a
# Mermaid ER diagram from PRAGMA introspection. The doc is tool-owned: the
# schema changes in board/store.go, the diagram regenerates, never by hand.
.PHONY: db-diagram
db-diagram:
	go run ./cmd/db-diagram

# Git hooks — install the committed .githooks/* into this repository's hooks
# dir. The target is the COMMON git dir, so in a worktree setup the hook lands
# once and gates commits in every worktree. The pre-commit hook re-runs
# db-diagram and blocks a commit whose docs/db-schema.md is stale; it never
# mutates the tree. Standalone — wired to no other target.
.PHONY: hooks
hooks:
	@hooks=$$(git rev-parse --git-common-dir)/hooks; \
	mkdir -p "$$hooks" && \
	install -m 0755 .githooks/pre-commit "$$hooks/pre-commit" && \
	echo "installed pre-commit -> $$hooks/pre-commit"
