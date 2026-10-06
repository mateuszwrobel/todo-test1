# e2e — the kanban browser suite

Browser-driven acceptance for the kanban app: each wave's lane script drives
the composed server in real chromium and re-executes that wave's parent
scenarios from `workplans/workplan_kanban_application.md` (acceptance, never
new Gherkin — see `workplans/dependencies_kanban.md`). The suite is CommonJS
scripts using the global `playwright` package; the lane scripts are
self-contained — each builds its own binaries, seeds fresh temp data files,
serves on a free port, and stops the server with SIGTERM.

The superseded todo-app lanes (w1..w11: browse, create, toggle, edit,
delete, stale, lifecycle, in-flight, gallery, components, visual) retired
with the kanban pivot's wave-end swap; their specs and pixel baselines
covered surfaces the pivot deleted.

## Run

From the repo root:

```sh
make e2e
```

Requirements: Go toolchain, the global `playwright` CLI package with
chromium installed (`playwright --version` works; `npm root -g` gives the
`NODE_PATH` the scripts' `require('playwright')` needs).

## KW1 — KW1 board foundation (`e2e/kw1-board.js`)

Re-executes the parent scenarios "Board shows fixed columns" and "Start
without todo data".

### What it asserts

1. **Board shows fixed columns** — a board seeded with cards in all three
   columns renders exactly three columns in the order To Do / In Progress /
   Done; each column lists its cards top-to-bottom in stored position order
   (the DOM order mirrors the contract's array order, and the stored
   positions are contiguous 0..n-1); the done treatment is carried purely by
   membership of the Done column — Done cards show it, no other card does,
   and no done checkbox or toggle exists anywhere on the page.
2. **Start without todo data** — with no data file at all, the server
   starts, the board is created empty, and the page shows the three fixed
   columns each stating its emptiness; no cards render, and GET /board
   answers the three fixed columns, each empty.

## Seeding

`e2e/testdata/` is a go tool (test-support, not served application code;
the go tool and archspec ignore testdata dirs): it seeds a kanban data file
through the board store's Create — the seed primitive — and places cards
into In Progress / Done directly in the data file until the store's move
operation lands at KW5. Cards created there keep stable ids and contiguous
per-column positions.

Binaries land in `e2e/bin/` (git-ignored via the scripts' `mkdir -p`).

## Visual regression

No pixel lane exists after the pivot: the committed baselines all depicted
the retired todo list renders and were deleted with their specs. The
visual-regression lane is restored at KW7 with kanban baselines (card
ui/13, `workplans/dependencies_kanban.md` §KW7).
