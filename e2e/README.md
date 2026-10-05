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

# W3 e2e — toggle done

Browser-driven check of wave W3 ("Mark todo done"): the page's checkbox PATCHes
the done state through the composed server and the row updates in place.

## Run

From the repo root:

```sh
make e2e-w3
```

## What it asserts

1. **Check → done without reload** — a marker placed on the live document
   survives the update (an htmx swap, not a reload); the row shows done, drops
   its edit control, texts and row count never move, the other row is intact.
2. **Uncheck → not-done without reload** — the same checkbox reopens the todo
   the same way; the edit control returns.
3. **Done persists across reload** — after a full reload the row shows done,
   read from the store.
4. **Reopen persists across reload** — not-done likewise.

# W2 e2e — create

Browser-driven check of the composed server for wave W2: the create feature
end to end, re-executing the parent scenarios "Create todo" and
"Reject empty todo text".

## Run

From the repo root:

```sh
make e2e-w2
```

Or directly (the script also builds its own binary):

```sh
NODE_PATH=$(npm root -g) node e2e/w2-create.js
```

## What it asserts

1. **Create appends without reload** — form submit through htmx adds the row
   as last, marked not-done; a window marker survives every swap, proving no
   navigation or full reload happened; the input is reset, ready for the
   next todo.
2. **Rejected create states the reason** — blank submit → the create area
   states "title is required", the typed text stays in the input, nothing is
   created, still no navigation.
3. **Over-limit create states the limit** — a 501-character submit states the
   500-character limit, nothing is created.
4. **Reload shows persistence** — a real reload (marker clears, proving the
   reload happened) renders exactly the created todos from the data file.

# W7 e2e — lifecycle

Real-process check of wave W7 ("Todos survive server restart" + clean
shutdown): binaries are started, driven, stopped with real SIGTERM signals,
and started again on the same data file.

## Run

From the repo root:

```sh
make e2e-w7
```

## What it asserts

1. **Restart resumes state** — mixed done mix (seeded), one toggle through a
   live checkbox click, the command stopped (SIGTERM) and started again on
   the same path: the JSON contract reports byte-identical todos and the
   reloaded page shows the same texts and states. Mid-run state changes go
   through the toggle PATCH over seeded rows, the same route the browser's
   checkbox uses.
2. **SIGTERM completes in-flight work** — a PATCH whose request body lands
   only after the signal (the handler sits mid-flight): the response still
   arrives with 200, the process exits 0, the listener stops, and a fresh
   start on the same file shows the completed change.

# W4 e2e — edit

Browser-driven check of the edit feature for wave W4: title editing through
the Change operation and the frozen-done rule, against the composed server.

## Run

From the repo root:

```sh
make e2e-w4
```

## What it asserts

1. **Done rows expose no edit control** — the done row has no Edit button and
   no edit band in the DOM at all; the not-done rows each have one (the ui/10
   pin: done freezes the title, so no affordance is offered).
2. **Edit opens in place** — clicking Edit swaps the title for a band:
   visible and prefilled with the current title. No navigation.
3. **Save applies the new title in place** — htmx swaps the list fragment:
   the row shows the new text with the band collapsed, position and done
   state are stable, the URL never changes, and a reload shows the new title
   (the value survives).
4. **Cancel collapses the band** — typed-but-unsaved text is discarded, the
   original title stays, and a reload confirms nothing was written.
5. **An edit saved after the todo was done behind the page's back is refused
   visibly** — marking the row done through the API and then saving an edit
   from the stale page states "cannot edit a done todo" on the row; the todo
   keeps its original title, the row now renders done with no edit control
   left, and a reload confirms the same.
6. **An empty edit is refused with the stated reason** — clearing the title
   and saving states "title is required"; the todo itself is intact (a reload
   shows the original title back).

# W6 e2e — stale page / missing todo

Browser-driven check of card ui/14 for wave W6: with a todo deleted behind
the open page's back (a direct api DELETE — the "second page"), every
operation on that missing id states the failure. CommonJS, global Playwright.

## Run

From the repo root:

```sh
make e2e-w6
```

## What it asserts

1. **Toggle on a missing todo states the failure** — clicking the stale row's
   checkbox swaps in the missing-todo banner stating "no such todo" (the
   contract's own reason), exactly once, visibly; the stale row is not left on
   the page faking the toggle, survivors keep text, done state, and order, and
   the server JSON is byte-equal before and after — no change occurred
   anywhere. No navigation.
2. **Edit-save on a missing todo states the same failure** — saving the stale
   row's edit band lands the same banner; the todo stays absent, survivors
   untouched, server unchanged.
3. **Delete of a missing todo states the same failure** — the stale Delete is
   not a silent fake success: the banner states it above the truthfully
   rendered (empty) list; the server already had nothing to delete and keeps
   nothing.
4. **Each reload shows the server truth** — after every failed operation a
   reload carries no banner and exactly the surviving (or empty) truth: the
   failure statement never outlives the reload.

# W9 e2e — styling + story gallery

Browser-driven check of wave W9: the page's visual layer (style.css) and the
`/__components` story gallery render in a real browser, while the real page's
behavior stays frozen. CommonJS, global Playwright.

## Run

From the repo root:

```sh
make e2e-w9
```

## What it asserts

1. **Gallery answers with every state** — `GET /__components` is 200, all
   seven state containers (`#state-list-populated`, `#state-empty`,
   `#state-create-error`, `#state-edit-band`, `#state-load-failure`,
   `#state-missing-todo`, `#state-in-flight-disabled`) are present and
   visible, each labelled by its `state: …` caption.
2. **Fixtures render the real fragments** — the populated fixture shows the
   four mixed-state rows, the create-error and missing-todo fixtures state
   the contract's own reasons ("title is required", "no such todo"), the
   edit band is prefilled, the failure offers its retry, and every control
   the in-flight window disables carries the `disabled` attribute.
3. **The stylesheet is served and applied** — `/static/style.css` answers
   200 and the rendered body carries the styled background, so the visual
   layer is provably in effect.
4. **The real page still works** — one create round-trip appends the row
   without a reload and survives a reload: the styling layer changed no
   behavior.

# W10 e2e — design system: tokens, components, drift gates

Browser-driven check of wave W10: the token layer (`/static/tokens.css`)
and the named component primitives render in a real browser, and the
`/__components` gallery's components block stays synced with both.
CommonJS, global Playwright.

## Run

From the repo root:

```sh
make e2e-w10
```

## What it asserts

1. **Every component example renders** — `GET /__components` is 200, the
   `#components` block sits above the state sections, and all seventeen
   `#c-*` examples (buttons incl. disabled, inputs incl. the static
   `.is-focus` state, the three checkbox states, both row states, error
   text, banner, hint, empty state, panel, heading, tokens) are present
   and visible.
2. **The frozen states render** — the disabled button and disabled
   checkbox carry `disabled`, the checked checkbox is checked, and the
   done row carries its `row--done` state class.
3. **Chips match the token file** — the `--color-*` inventory parsed from
   the served `tokens.css` and the labels of the `#c-tokens` chips are
   the same set: a token added or renamed without its gallery chip fails
   in the browser too (the same gate exists as a Go unit test).
4. **The token layer is live** — the primary button's computed background
   is the token's resolved color, and `style.css` consumes the tokens via
   `var()`; the real page still renders through the layer.
