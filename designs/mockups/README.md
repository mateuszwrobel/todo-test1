# UI mockups

AI-generated visual references for the todo app, one per observable page state defined in
[`workplans/user-journeys.md`](../../workplans/user-journeys.md). These are **not final designs** —
they are diffusion-model renderings of the journey states, meant as a starting point for UI work.
Text and pixel details in them are approximate; the journeys and the API contract remain the truth.

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
