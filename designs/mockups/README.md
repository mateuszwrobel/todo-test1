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

Re-rolled per image over seeds 1–3 with frozen captions; the cleanest render was committed. Per the frozen-done-text rule, done rows intentionally lack Edit — checkbox + strikethrough + Delete only; not-done rows keep Edit + Delete.

- `03` (seed 1): clean.
- `01` (seed 3): one ghost band with stray "Edit"/"Delete" buttons and no checkbox or text sits between rows 2 and 3 — the real list is the four rows; ghost bands are the known diffusion leak, ignore them.
- `04` (seed 2): one phantom band labeled "Edit" with plain Edit/Delete buttons sits between the edit row and "Read 20 pages" — the single real edit band is the one with the input and the blue Save button.
- `06` (seed 1): one text-less ghost band with stray "Edit"/"Delete" buttons sits between rows 2 and 3 — the real list is exactly the three todos under the red banner.
- Strikethrough on done rows may extend past the text; the checkbox is the done indicator.
- `06` depicts three todos while others show four — deliberate simplification after the renderer duplicated rows; journeys use four as illustrative copy only.
