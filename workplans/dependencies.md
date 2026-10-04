# Scenario Dependencies — Parallel Work Graph

Companion to the scenario cards in `workplans/scenarios/`. States which tasks can run
concurrently. Edges are behavioral (a task's Given needs another task's behavior), not
process rules. Format: task → depends-on (all edges within one module unless the target
names a module).

## Cross-module edges (contract vs code)

| Edge kind | Meaning | Parallelism unlocked |
|-----------|---------|----------------------|
| contract edge — X → Y-contract | X's tests exercise Y through the **published contract** (Y's workplan API partial), with an in-test fake behind the consumer-defined port | X starts immediately; the contract is committed |
| code edge — X → Y | X needs Y's **real implementation** wired | X lands only after Y |

- api → todos **contract only** (injected port, fake in tests) — no code edge
- ui → api **contract only** (tests run a fake contract server) — no code edge
- server → todos + api + ui **code** (composition wires the real modules)
- archspec gate: green requires the real package structure of all modules to exist

## todos (internal code edges — same module, same package)

| Task | Depends on | Why |
|------|-----------|-----|
| todos/01 create-assigns-fresh-identifier | — | root: Create |
| todos/02 create-rejects-invalid-text | — | Create validation, no state prerequisite |
| todos/13 fresh-file-opens-as-empty-valid-store | — | Open only |
| todos/03 list-is-creation-order | 01 | List test seeds via Create |
| todos/04 change-title-of-a-not-done-todo-keeps-done-state | 01 | needs an existing todo |
| todos/05 change-done-state-keeps-title | 01 | needs an existing todo |
| todos/10 delete-removes-and-identifier-is-never-reused | 01 | Delete + Create interplay |
| todos/12 state-survives-reopen | 01 | seed via Create, reopen, read |
| todos/14 completed-operation-survives-process-death | 01 | Create then kill |
| todos/06 change-title-on-a-done-todo-is-refused | 01, 05 | needs a done todo (done-only change) |
| todos/07 reopen-a-done-todo-by-done-only-change | 01, 05 | same |
| todos/08 change-missing-identifier | 04, 05 | Change must exist to miss |
| todos/09 change-with-no-fields-is-invalid | 04, 05 | Change must exist to reject empty |
| todos/11 delete-missing-identifier | 10 | Delete must exist to miss |

## api (all tasks: contract edge to todos only — fully parallel with todos)

| Task | Depends on | Why |
|------|-----------|-----|
| api/01–11 (every card) | todos contract (committed) | translator tests use the injected port + fake; status mapping is api's own rule |

Internal: none — the handler table (parse → call → map) is one cohesive change; cards
are test-first increments of the same surface, safe to pair in one lane, order-free.

## ui (all tasks: contract edge to api only — fully parallel with api and todos)

| Task | Depends on | Why |
|------|-----------|-----|
| ui/01 page-shows-the-list-truthfully | api contract | page renders a GET result (fake server in tests) |
| ui/02 empty-list-is-stated-not-blank | api contract | same, empty result |
| ui/03 load-failure-is-stated-not-faked | api contract | unreachable/failing fake server |
| ui/04 load-failure-recovers-by-retry | ui/03 | retry lives in the failure state |
| ui/05 create-appends-without-reload | ui/01 | swap targets exist on the page first |
| ui/06 rejected-create-states-the-reason | ui/05 | error surface attaches to the create area |
| ui/07 over-limit-create-states-the-limit | ui/06 | same surface, other message |
| ui/08 toggle-marks-done-and-reopens | ui/01 | row controls render first |
| ui/09 inline-edit-updates-in-place | ui/01 | row edit control renders first |
| ui/10 done-rows-carry-no-edit-control | ui/01 | render rule on the row |
| ui/11 stale-edit-of-a-done-todo-is-refused-visibly | ui/09 | surfaces the edit rejection path |
| ui/12 empty-edit-text-keeps-the-original | ui/09 | same |
| ui/13 delete-drops-one-row | ui/01 | row delete control renders first |
| ui/14 missing-todo-states-the-failure-for-any-operation | ui/08, ui/09, ui/13 | banner must serve all three operations |
| ui/15 reload-matches-the-server | ui/01 | reload = full re-render |
| ui/16 controls-serialize-operations-per-control | ui/05, ui/08, ui/09, ui/13 | blocking applies to the four real controls |

## server (code edges — composition lands after todos + api + ui)

| Task | Depends on | Why |
|------|-----------|-----|
| server/01 start-serves-both-surfaces | todos, api, ui (code) | wiring has nothing to wire otherwise |
| server/02 fresh-path-starts-empty | server/01, todos/13 | startup on absent file |
| server/03 restart-resumes-state | server/01, todos/12 | the restart story end to end |
| server/04 clean-shutdown-completes-in-flight-work | server/01 | drain belongs to the listener root owns |
| server/05 unusable-configuration-fails-loudly | server/01 | failure path of the same startup |
| server/06 wiring-honors-the-dependency-directions | all modules (code) + archspec gate | the gate itself is the test |

## Parent scenarios (integration wave — after server/01)

The 9 parent scenarios become the Playwright e2e suite; each depends on: everything its
module cards depend on, plus server/01 (one composed process to drive).

## Parallel lanes (what runs at once)

```
lane-A (todos)   : wave1 {01, 02, 13} → wave2 {03, 04, 05, 10, 12, 14} → wave3 {06, 07, 08, 09, 11}
lane-B (api)     : {01..11} immediately — contract fake, zero waiting
lane-C (ui)      : wave1 {01, 02, 03} → wave2 {04, 05, 08, 09, 10, 13, 15} → wave3 {06, 07, 11, 12, 14, 16}
lane-D (server)  : starts when A+B+C merged → {01} → {02, 03, 04, 05} ∥ {06 whenever structure exists}
lane-E (e2e)     : parent 9 scenarios after server/01 green
```

Lane rules:
- Lanes land through separate worktrees; only the orchestrator merges ff-only, in merge order A, B, C (they touch disjoint packages — conflicts not expected).
- A lane's contract-fake tests must also run the real dependency once it lands (integration test behind the same port) before the lane claims done.
- archspec `verify --strict` runs in every lane's CI; it goes green only at lane-D, findings before that must name exactly the not-yet-landed packages.
