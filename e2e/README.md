# W1 e2e — foundation + browse

Browser-driven check of the composed server for wave W1: the page opens in
chromium and renders each W1 state truthfully (populated list, empty state,
empty state after the data file is removed while the server is stopped).

## Run

From the repo root:

```sh
make e2e-w1
```

Or directly (builds first via the make target; the script also builds its
own binaries):

```sh
NODE_PATH=$(npm root -g) node e2e/w1-browse.js
```

Requirements: Go toolchain, the global `playwright` CLI package with
chromium installed (`playwright --version` works; `npm root -g` gives the
`NODE_PATH` the script's `require('playwright')` needs).

## What it asserts

1. **Populated list** — seeded mixed-state db; rows render in creation order,
   checkboxes show done state, delete controls on every row, edit controls
   only on not-done rows.
2. **Empty state** — fresh db; `#empty-state` shows, no list, create control
   ready.
3. **Db removed while stopped** — start once with a seeded db, stop, delete
   the file, start again on the same path; the empty state shows.

Binaries land in `e2e/bin/` (git-ignored via the script's `mkdir -p`); the
seed helper is `e2e/testdata/` (go tool and archspec ignore testdata dirs;
test-support tool, not served application code).
