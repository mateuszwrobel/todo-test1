# Kanban Scenario Dependencies — Feature Waves

Companion to the scenario cards in `workplans/scenarios-kanban/` and the module plans [workplan_board_store.md](workplan_board_store.md), [workplan_api_board.md](workplan_api_board.md), [workplan_ui_board.md](workplan_ui_board.md), [workplan_server_board_composition.md](workplan_server_board_composition.md). Parent: [workplan_kanban_application.md](workplan_kanban_application.md) · pivot record: [ADR-003](../docs/adr/ADR-003-kanban-pivot.md). All ordering lives here (cards carry none). The todo-app ledger ([dependencies.md](dependencies.md)) is superseded history; its lane rules are inherited unchanged (see Lane rules below restates them for this ledger).

Work proceeds **feature by feature**: a wave delivers one feature vertically — store → contract translation → page — and ends with that feature integrated against the real code landed earlier in the same wave. No fake-then-replace staging, no module-by-module lanes. The standing law: **a module is never implemented wholesale**; a module grows one feature per wave it appears in.

Superseded code retires as its replacement lands: the `todos` store package is deleted in the wave whose composition no longer references it (KW1 wiring replaces the store's role; package deletion lands with KW4 when the last surviving todo endpoint retires); todo list-render components retire in KW1/KW2 when board rendering replaces them; todo endpoints retire in the wave their kanban counterpart lands (GET in KW1, POST in KW2, PATCH in KW3/KW5, DELETE in KW4).

Card notation: `<module>/<NN>`. Every card appears in exactly one wave; intra-wave rows give each card's concrete dependencies.

## KW1 — Foundation + board view

The board read is the first feature; composition wires from the start so every later wave extends a real process. The kanban.db schema and GET /board + board rendering replace the todo store/list surface here.

| Card | Depends on | Note |
|------|-----------|------|
| board/01 fresh-store-opens-empty-valid | — | Open root; kanban.db lifecycle starts |
| board/02 create-appends-bottom-todo | board/01 | Create exists as the seed primitive; validation completes in KW2 |
| board/05 list-fixed-columns-position-order | board/02 | List seeds via Create |
| api/01 board-read-shape | board/05 | GET handler over the real store |
| ui/01 board-renders-three-fixed-columns | api/01, server/01 | first real render; todo list render retires |
| ui/02 empty-board-rendered-as-stated | ui/01 | render rule |
| ui/03 load-failure-rendered-as-stated | ui/01 | failure render |
| server/01 start-serves-board-and-page | board/01, api/01, ui/01 | todo GET retires here; todos package retires at KW4 when last endpoint retires |
| server/03 start-without-todos-empty-board | server/01, board/01 | startup on absent files |
| server/08 wiring-honors-dependency-directions | all four packages wired | archspec gate goes green at wave end |

**Wave end:** page opens showing the three fixed columns; board / empty / load-failure render truthfully; `archspec verify --strict` green. Parent scenarios **"Board shows fixed columns"** + **"Start without todo data"** pass in the browser.

## KW2 — Create

| Card | Depends on | Note |
|------|-----------|------|
| board/03 create-rejects-blank-text | board/02 | validation completes the create feature |
| board/04 create-rejects-over-long-text | board/02 | same rule site |
| api/02 create-returns-card | board/02 | POST handler over the real store; todo POST endpoint retires here |
| api/03 create-blank-title-stated-422 | api/02, board/03 | outcome→422 mapping |
| api/04 create-over-limit-stated-422 | api/03, board/04 | same handler, limit message |
| ui/04 create-appends-without-reload | ui/01, api/02 | create band + append onto the existing render |
| ui/05 rejected-create-states-reason | ui/04, api/03 | error surface on the create band (covers the limit message too) |

**Wave end:** create end-to-end. Parent scenarios **"Create card"** + **"Reject empty card text"** + **"Reject over-long card text"** pass in the browser.

## KW3 — Edit (+ the change operation's foundations)

The PATCH surface lands here (title direction first); move extends it in KW5. Done cards stay editable — no frozen-text rule survives the pivot.

| Card | Depends on | Note |
|------|-----------|------|
| board/08 title-change-keeps-place-identity | board/02 | Change exists, title direction |
| board/09 change-unknown-card-reported | board/08 | not-found reporting needs Change |
| board/10 invalid-column-rejected | board/08 | enum guard inside Change |
| api/05 patch-title-returns-card | board/08 | PATCH handler, title direction; todo PATCH retires here |
| api/07 patch-unknown-id-stated-404 | api/05, board/09 | 404 mapping |
| api/08 patch-invalid-column-stated-422 | api/05, board/10 | 422 mapping |
| api/09 patch-empty-body-stated-422 | api/05 | defensive mapping |
| ui/06 edit-updates-in-place | ui/01, api/05 | edit affordance on every card in every column + stated rejection |

**Wave end:** edit end-to-end including rejections and the missing-card path for PATCH. Parent scenario **"Edit card text keeps place"** passes; **"Operation on missing card"** green for the edit leg.

## KW4 — Delete

| Card | Depends on | Note |
|------|-----------|------|
| board/11 delete-closes-gap-no-id-reuse | board/02 | Delete + renormalize + id non-reuse |
| board/12 delete-unknown-card-reported | board/11 | not-found reporting |
| api/10 delete-answers-204 | board/11 | DELETE handler; todo DELETE retires here |
| api/11 delete-unknown-id-stated-404 | api/10, board/12 | 404 mapping |
| ui/07 delete-drops-one-card | ui/01, api/10 | card removal swap; column packs without gap |

**Wave end:** delete end-to-end. Parent scenario **"Delete card"** passes; missing-card leg green for DELETE.

## KW5 — Drag: move, reorder, done rendering

The kanban feature proper: position becomes user-manipulated state. PATCH move extends the KW3 handler.

| Card | Depends on | Note |
|------|-----------|------|
| board/06 move-changes-column-keeps-neighbors | board/08 | cross-column move + dual renormalization, one transaction |
| board/07 reorder-within-column | board/06 | same-column index placement |
| api/06 patch-move-returns-moved-card | api/05, board/06 | column/position directions of the PATCH handler |
| ui/08 drag-between-columns-one-request | ui/06, api/06 | drag mechanics + drop indicator + one PATCH per drop |
| ui/09 drag-reorder-persists | ui/08, board/07 | same-column drops; order survives reload |
| ui/10 abandoned-drag-changes-nothing | ui/08 | invalid drop = no request |
| ui/11 stale-operation-states-failure | ui/06, ui/08, ui/07 | stated failure surface must serve edit, drag, and delete |

**Wave end:** move + reorder end-to-end; done treatment reads purely from column membership. Parent scenarios **"Drag card between columns"** + **"Drag reorder within a column"** + **"Done is column membership"** pass; **"Operation on missing card"** fully green.

## KW6 — Lifecycle + migration + in-flight serialization

| Card | Depends on | Note |
|------|-----------|------|
| board/13 state-survives-close-reopen | board/06 | mixed-state seed, close/reopen |
| server/04 board-survives-restart | server/01, board/13 | end-to-end restart story |
| server/07 clean-shutdown-completes-in-flight | server/01 | drain + close |
| server/06 unusable-config-fails-loudly | server/01 | failure path of the same startup |
| board/14 import-seeding-preserves-order | board/02 | ordered seed primitive for the import |
| server/02 first-start-imports-once | server/01, board/14 | guard + single-transaction import; reads todos.db read-only |
| server/05 interrupted-import-leaves-no-half-board | server/02, board/14 | transaction rollback proof |
| ui/12 triggered-control-blocks-in-flight | ui/04, ui/06, ui/07, ui/08 | blocking applies to all real controls incl. the dragged card |

**Wave end:** parent scenarios **"Board survives server restart"** + **"Migrate existing todos on first start"** + **"Repeat activation while an operation is in flight"** pass.

## KW7 — Kanban styling + gallery + visual baselines

Behavior-frozen styling pass, mirroring the retired todo-app W9–W11 pattern in one wave: board components styled to `designs/mockups/kanban/`, every observable state rendered on the component gallery, pixel baselines committed.

| Card | Depends on | Note |
|------|-----------|------|
| ui/13 every-state-renders-in-gallery | KW1–KW6 (all board features) | tokens gain board components; gallery sections for board/empty/load-failure/edit/drag/done/error states; `toHaveScreenshot` baselines deterministic (seeded db, fixed viewport) |

**Wave end:** the page matches the kanban mockups; every state renders on one gallery page; two back-to-back visual runs green; KW1–KW6 e2e and unit suites stay green (styling is behavior-frozen).

## Wave dependencies (the parallel graph)

| Wave | Requires | Can run parallel with |
|------|----------|----------------------|
| KW1 foundation+view | — | — |
| KW2 create | KW1 | KW3, KW4 |
| KW3 edit | KW1 | KW2, KW4 |
| KW4 delete | KW1 | KW2, KW3 |
| KW5 drag | KW2, KW3, KW4 | — |
| KW6 lifecycle+migration+in-flight | KW3, KW4, KW5 | — |
| KW7 styling | KW6 | — |

After KW1 the widest parallel spread is {KW2 ∥ KW3 ∥ KW4}; KW5 is a join on KW2+KW3+KW4; KW6 and KW7 are the tail.

## Lane rules

(inherited from the superseded ledger, restated)

- One lane per wave-branch; lanes land through separate worktrees, orchestrator merges ff-only in wave order of the graph above.
- A wave's cards land in dependency order inside the wave; the wave's integration check (real wiring of its cards + archspec green) gates the merge — integration is the wave end, never an in-flight card requirement.
- No card claims an integration result its wave has not reached.
- Parent scenarios re-executed by Playwright at each stated wave end are acceptance for that feature — the parent file is the suite, no new Gherkin.
- `archspec verify --strict` runs in every lane; from KW1 end it stays green; findings before that name only not-yet-landed packages.
- Retiring superseded code (todo store/endpoints/list renders) is part of the wave that replaces its behavior — no zombie surfaces serving stale contracts.
