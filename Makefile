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

.PHONY: e2e-w5

# W5 e2e: real-browser delete acceptance — click delete removes the row via
# an htmx swap (no navigation), reload shows persistence, deleting the last
# row lands the empty state.
e2e-w5:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/w5-delete.js

# W2 e2e: the create feature driven through a real browser against the
# composed server. See e2e/README.md.
.PHONY: e2e-w2
e2e-w2:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	NODE_PATH=$$(npm root -g) node e2e/w2-create.js

# W7 e2e: lifecycle in real processes — restart resumes state (mixed done
# mix survives a real SIGTERM stop + start on the same db, page + JSON agree)
# and SIGTERM completes in-flight work (exit 0, durable change).
.PHONY: e2e-w7
e2e-w7:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/w7-lifecycle.js
