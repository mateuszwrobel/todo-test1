# Scenario Dependencies — Feature Waves

Companion to the scenario cards in `workplans/scenarios/`. All ordering lives here (cards carry none). Work proceeds **feature by feature**: a wave delivers one feature vertically — store → contract translation → page — and ends with that feature integrated against the real code landed earlier in the same wave. No fake-then-replace staging, no module-by-module lanes: integration for a feature happens at its wave end, at the first moment all its pieces exist.

The standing law: **a module is never implemented wholesale.** Every increment is a functional/common-scenario slice — the thinnest cut through the modules that makes one feature observable end to end. Whole-module waves are forbidden; a module grows one feature per wave it appears in.

Card notation: `<module>/<NN>`. Every card appears in exactly one wave; intra-wave rows give each card's concrete dependencies.

## W1 — Foundation + browse

The list is the first feature; composition wires from the start so every later wave has a real process to extend.

| Card | Depends on | Note |
|------|-----------|------|
| todos/13 fresh-file-opens-as-empty-valid-store | — | Open root |
| todos/01 create-assigns-fresh-identifier | todos/13 | Create exists as the seed primitive; validation completes in W2 |
| todos/03 list-is-creation-order | todos/01 | List seeds via Create |
| api/04 list-todos | todos/03 | GET handler over the real store |
| ui/01 page-shows-the-list-truthfully | api/04, server/01 | first real render |
| ui/02 empty-list-is-stated-not-blank | ui/01 | render rule |
| ui/03 load-failure-is-stated-not-faked | ui/01 | failure render |
| ui/04 load-failure-recovers-by-retry | ui/03 | retry lives in the failure state |
| ui/15 reload-matches-the-server | ui/01 | full re-render path |
| server/01 start-serves-both-surfaces | todos/13, api/04, ui/01 | wiring lands now, not last |
| server/02 fresh-path-starts-empty | server/01, todos/13 | startup on absent file |
| server/05 unusable-configuration-fails-loudly | server/01 | failure path of the same startup |
| server/06 wiring-honors-the-dependency-directions | all four packages wired | archspec gate goes green at wave end |

**Wave end:** page opens; list / empty / load-failure + retry render truthfully; archspec verify --strict green (structure complete). Parent view clauses satisfied.

## W2 — Create

| Card | Depends on | Note |
|------|-----------|------|
| todos/02 create-rejects-invalid-text | todos/01 | validation completes the create feature |
| api/01 create-succeeds | todos/01 | POST handler over the real store |
| api/02 create-with-blank-title | api/01, todos/02 | outcome→422 mapping on the POST handler |
| api/03 create-over-length-limit | api/02 | same handler, limit message |
| ui/05 create-appends-without-reload | ui/01, api/01 | form + swap onto the existing page |
| ui/06 rejected-create-states-the-reason | ui/05, api/02 | error surface on the create area |
| ui/07 over-limit-create-states-the-limit | ui/06, api/03 | same surface, limit message |

**Wave end:** create end-to-end. Parent scenarios "Create todo" + "Reject empty todo text" pass in the browser.

## W3 — Toggle done (+ the Change operation's foundation)

| Card | Depends on | Note |
|------|-----------|------|
| todos/05 change-done-state-keeps-title | todos/01 | Change exists, done direction |
| todos/08 change-missing-identifier | todos/05 | not-found reporting needs Change |
| todos/09 change-with-no-fields-is-invalid | todos/05 | empty-change rejection |
| api/06 change-done-state-in-either-direction | todos/05 | PATCH handler, done direction |
| api/08 change-missing-todo | api/06, todos/08 | 404 mapping |
| api/09 change-with-empty-body | api/06, todos/09 | 422 mapping (defensive path) |
| ui/08 toggle-marks-done-and-reopens | ui/01, api/06 | checkbox → PATCH done |

**Wave end:** mark done / reopen end-to-end. Parent scenario "Mark todo done" passes.

## W4 — Edit

Text editing; the frozen-done rule reads the done flag, so it lands after the flag's toggle (W3) exists — its scenarios state done todos as Given.

| Card | Depends on | Note |
|------|-----------|------|
| todos/04 change-title-of-a-not-done-todo-keeps-done-state | todos/01, todos/05 | title direction of Change |
| todos/06 change-title-on-a-done-todo-is-refused | todos/04, todos/05 | frozen rule over real done state |
| todos/07 reopen-a-done-todo-by-done-only-change | todos/05, todos/06 | reopen-unlocks-edit story |
| api/05 change-text-of-a-not-done-todo | todos/04 | PATCH title direction |
| api/07 title-edit-on-done-todo-refused | todos/06, api/06 | 422 "cannot edit a done todo" |
| ui/09 inline-edit-updates-in-place | ui/01, api/05 | inline edit band + save |
| ui/10 done-rows-carry-no-edit-control | ui/01, todos/06 | render rule: no edit affordance on done rows |
| ui/11 stale-edit-of-a-done-todo-is-refused-visibly | ui/09, api/07 | stated refusal path |
| ui/12 empty-edit-text-keeps-the-original | ui/09, api/05 | empty title → stated, original intact |

**Wave end:** edit end-to-end including the frozen refusal. Parent scenarios "Edit todo text leaves done state alone" + "Done todo rejects text edits" pass.

## W5 — Delete

| Card | Depends on | Note |
|------|-----------|------|
| todos/10 delete-removes-and-identifier-is-never-reused | todos/01 | Delete + id non-reuse |
| todos/11 delete-missing-identifier | todos/10 | not-found reporting |
| api/10 delete | todos/10 | DELETE handler |
| api/11 delete-missing-todo | api/10, todos/11 | 404 mapping |
| ui/13 delete-drops-one-row | ui/01, api/10 | row removal swap |

**Wave end:** delete end-to-end. Parent scenario "Delete todo" passes.

## W6 — Stale page / missing todo

| Card | Depends on | Note |
|------|-----------|------|
| ui/14 missing-todo-states-the-failure-for-any-operation | ui/08, ui/09, ui/13 | banner must serve toggle, edit, and delete |

(api/todos 404 reporting for all three operations landed with W3–W5.)

**Wave end:** parent scenario "Operation on missing todo" passes.

## W7 — Lifecycle

| Card | Depends on | Note |
|------|-----------|------|
| todos/12 state-survives-reopen | todos/01, todos/05 | mixed-state seed, close/reopen |
| todos/14 completed-operation-survives-process-death | todos/01 | durability across kill |
| server/03 restart-resumes-state | server/01, todos/12 | end-to-end restart story |
| server/04 clean-shutdown-completes-in-flight-work | server/01 | drain + close |

**Wave end:** parent scenario "Todos survive server restart" passes.

## W8 — In-flight control serialization

| Card | Depends on | Note |
|------|-----------|------|
| ui/16 controls-serialize-operations-per-control | ui/05, ui/08, ui/09, ui/13 | blocking applies to all four real controls |

**Wave end:** parent scenario "Repeat activation while an operation is in flight" passes.

## W9 — Visual styling + story gallery

| Card | Depends on | Note |
|------|-----------|------|
| ui/17 styling-to-mockups + component-story-gallery | ui/01–ui/16 (the W1–W8 features) | behavior-frozen style.css + /__components rendering every observable state from fixtures |

**Wave end:** the page matches `designs/mockups/` and every state renders on one gallery page; parent scenarios unchanged — W1–W8 e2e stay green (styling is behavior-frozen).

## W10 — Design system: tokens, components, gallery examples

| Card | Depends on | Note |
|------|-----------|------|
| ui/18 design-system-tokens-and-components | ui/17 (the W9 feature) | behavior-frozen `tokens.css` token layer + named component classes consumed by the templates + `#c-*` gallery examples with token-drift gates |

**Wave end:** every design value lives in `ui/static/tokens.css` and every styled primitive has a named class with a gallery example; parent scenarios unchanged — W1–W9 e2e stay green (class swap and token split are behavior- and pixel-frozen).

## Wave dependencies (the parallel graph)

| Wave | Requires | Can run parallel with |
|------|----------|----------------------|
| W1 foundation+browse | — | — |
| W2 create | W1 | W5, W7 |
| W3 toggle | W1 | W5, W7 |
| W4 edit | W3 | W5 |
| W5 delete | W1 | W2, W3, W4, W7 |
| W6 stale | W3, W4, W5 | — |
| W7 lifecycle | W3 | W2, W4, W5 |
| W8 in-flight | W2, W3, W4, W5 | — |

After W1 the widest parallel spread is {W2, W3∥W5, W7}; W4 follows W3; W6 and W8 are joins.

## Lane rules

- One lane per wave-branch; lanes land through separate worktrees, orchestrator merges ff-only in wave order of the graph above.
- A wave's cards land in dependency order inside the wave; the wave's integration check (real wiring of its cards + archspec green) gates the merge — integration is the wave end, never an in-flight card requirement.
- No card claims an integration result its wave has not reached.
- Parent scenarios re-executed by Playwright at each stated wave end are acceptance for that feature — the parent file is the suite, no new Gherkin.
- archspec verify --strict runs in every lane; from W1 end it stays green (structure complete); findings before that name only not-yet-landed packages.
