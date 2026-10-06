# UI mockups

AI-generated visual references for the todo app, one per observable page state defined in
[`workplans/user-journeys.md`](../../workplans/user-journeys.md). These are **not final designs** —
they are diffusion-model renderings of the journey states, meant as a starting point for UI work.
Text and pixel details in them are approximate; the journeys and the API contract remain the truth.

## Kanban board mockups (current)

Superseded todo-app renders below are kept as history (ADR-003); the current set mocks the kanban journeys [workplans/user-journeys_kanban.md](../../workplans/user-journeys_kanban.md), one per journey J1–J7:

| Mockup | Journey / state covered | Prompt file |
|---|---|---|
| `kanban/k1-board-populated.png` | J1 — three fixed columns, cards in position order, Done cards check-marked, create band | `kanban/prompts/k1-board-populated.json` |
| `kanban/k2-create-error.png` | J2 rejected create — red "title is required" under the input, board unchanged | `kanban/prompts/k2-create-error.json` |
| `kanban/k3-drag-between.png` | J3 cross-column drag — dashed source slot, floating chip, blue insertion line in target | `kanban/prompts/k3-drag-between.json` |
| `kanban/k4-reorder.png` | J4 in-column reorder — dashed slot + insertion line inside "To Do" | `kanban/prompts/k4-reorder.json` |
| `kanban/k5-done-column.png` | J5 done rendering — green-bordered Done column focal; arrow drawn in reopen direction | `kanban/prompts/k5-done-column.json` |
| `kanban/k6-edit-card.png` | J6 inline edit — prefilled input + blue Save replacing the card row (Cancel affordance deliberately absent from the render) | `kanban/prompts/k6-edit-card.json` |
| `kanban/k7-delete-card.png` | J7 delete hover — red Delete on the acted card; other rows render controls on hover only | `kanban/prompts/k7-delete-card.json` |

Kanban rendering notes: all accepted on first seeded draw except k6 (two rerolls: "Cancel" label produced ghost fragments — removed from caption; a seed reroll duplicated a row) and k7 (first caption repeated per-row controls and duplicated label rows — simplified to hover-only controls on the acted row). Residual artifacts across the set: small checkbox glyphs leak beside Edit/Delete labels; occasional letter slips. Board copy is shared across mockups (same six cards) — deliberate, aids cross-mockup comparison.

## Todo app mockups (superseded)

Superseded per [ADR-003](../../docs/adr/ADR-003-kanban-pivot.md) — kept as history. The table below maps the old todo-app renders to the pre-pivot journeys in [`workplans/user-journeys.md`](../../workplans/user-journeys.md).

| Mockup | Journey / state covered | Prompt file |
|---|---|---|
| `01-list-populated.png` | J1/J2/J3/J5 main view — list loaded, oldest first, done + not-done rows (done: Delete only, not-done: Edit + Delete), create row | `prompts/01-list-populated.json` |
| `02-list-empty.png` | J1 empty state, J5 post-delete — distinct empty message, create input ready | `prompts/02-list-empty.json` |
| `03-create-error.png` | J2 rejected create — "text is required" on the create control, list unchanged | `prompts/03-create-error.json` |
| `04-edit-row.png` | J4 edit — inline edit with prefilled input + Save (rejected-edit "text is required" surface is shown in 03, same message) | `prompts/04-edit-row.json` |
| `05-load-error.png` | J1 load failure — stated "could not load" + Retry, distinct from empty | `prompts/05-load-error.json` |
| `06-missing-todo.png` | J6 stale operation — "todo does not exist" banner above the list, rows rendered unchanged, no success pretense | `prompts/06-missing-todo.json` |

## Generation recipe

POST the caption JSON (stringified) as `prompt` to `http://192.168.0.110:8020/sdapi/v1/txt2img`
with `{"model":"ming-image","width":1024,"height":1024,"steps":12,"cfg":1,"sampler":"euler"}`.

`jq -r .images[0] resp.json | base64 -d > out.png` — captions are Ming-Image structured
Figma-style JSON; each prompt file holds the exact caption used, so every rendered string is reproducible.

## Known rendering artifacts

Captions use a list-as-panel structure (one layer describing the framed panel and all enumerated rows) after declared per-row label repeats proved to leak ghost bands; that restructure eliminated all ghost bands and floating labels — every committed render below was accepted on its first seeded draw (seed 1).

- `01`, `03`, `04`, `06`: no phantom rows or stray labels; row counts exact; done rows show checkbox + strikethrough + Delete only (frozen-done-text rule), not-done rows show Edit + Delete; `04` has exactly one edit band (prefilled input + blue Save).
- Strikethrough on done rows can extend slightly past the end of the text (occasionally with a small crossing tick at the end) — visual artifact of the diffusion text renderer; the checkbox is the done indicator.
- `06` depicts three todos while others show four — deliberate simplification after the renderer duplicated rows; journeys use four as illustrative copy only.
