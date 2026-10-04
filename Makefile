# todo application — wave tooling

.PHONY: e2e-w1

# W1 e2e: builds the app + seed binaries and drives chromium via the global
# Playwright install. See e2e/README.md.
e2e-w1:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/w1-browse.js
.PHONY: e2e-w3

# W3 e2e: toggle done in a real browser — check → done without a full
# reload, uncheck → not-done, reload persists either direction.
e2e-w3:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/w3-toggle.js
