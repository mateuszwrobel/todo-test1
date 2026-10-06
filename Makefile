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
