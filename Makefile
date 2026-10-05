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

.PHONY: e2e-w4

# W4 e2e: the edit feature driven through a real browser — edit band on
# not-done rows only (done rows expose no control: the ui/10 pin), in-place
# Save with no navigation and stable ordering, Cancel, the frozen-done
# refusal stated visibly on a stale page, and the empty-title refusal with
# the todo intact. See e2e/README.md.
e2e-w4:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/w4-edit.js

# W6 e2e: real-browser stale-page acceptance — the todo deleted behind the
# page's back; toggle, edit-save, and delete of the missing id each state the
# missing-todo failure (banner, contract's "no such todo"), never a fake
# success; a reload shows exactly the server truth with no banner left.
.PHONY: e2e-w6
e2e-w6:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/w6-stale.js

# W8 e2e: in-flight control serialization in a real browser — each of the
# four controls (create Add, row checkbox toggle, edit Save, row Delete) is
# activated twice inside a route-delayed in-flight window; the counting
# proxy proves exactly one request per double activation, GET /todos proves
# exactly one server-side effect, and the control re-enables after success
# AND after a forced 500 failure (retry then lands).
.PHONY: e2e-w8
e2e-w8:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	go build -o e2e/bin/seed ./e2e/testdata
	NODE_PATH=$$(npm root -g) node e2e/w8-inflight.js

# W9 e2e: styling + story gallery in a real browser — GET /__components
# answers 200 with all seven state containers present and visible (each a
# real template rendering of a deterministic fixture, labelled by its
# caption), style.css is served and in effect, and the real page still
# works: one create round-trip with reload persistence proves the styling
# layer changed no behavior.
.PHONY: e2e-w9
e2e-w9:
	@mkdir -p e2e/bin
	go build -o e2e/bin/todo ./cmd/todo
	NODE_PATH=$$(npm root -g) node e2e/w9-gallery.js
