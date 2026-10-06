# kanban application — wave tooling

# The e2e suite drives the composed server in a real browser (Playwright,
# global install — NODE_PATH resolves require('playwright')). Each wave's
# lane script re-executes that wave's parent scenarios; see e2e/README.md.
# The KW1 lane covers "Board shows fixed columns" + "Start without todo
# data". The retired todo-app lanes (w1..w11) deleted with the pivot's
# wave-end swap; the visual-regression lane returns with KW7's baselines.
.PHONY: e2e
e2e:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/kw1-board.js
