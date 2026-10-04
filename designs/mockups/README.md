# UI mockups

AI-generated visual references for the todo app, one per observable page state defined in
[`workplans/user-journeys.md`](../../workplans/user-journeys.md). These are **not final designs** —
they are diffusion-model renderings of the journey states, meant as a starting point for UI work.
Text and pixel details in them are approximate; the journeys and the API contract remain the truth.

| Mockup | Journey / state covered | Prompt file |
|---|---|---|
| `01-list-populated.png` | J1/J2/J3/J5 main view — list loaded, oldest first, done + not-done rows, create row, per-row edit/delete | `prompts/01-list-populated.json` |
| `02-list-empty.png` | J1 empty state, J5 post-delete — distinct empty message, create input ready | `prompts/02-list-empty.json` |
| `03-create-error.png` | J2 rejected create — "text is required" on the create control, list unchanged | `prompts/03-create-error.json` |
| `04-edit-row.png` | J4 edit + rejected edit — inline edit with Save/Cancel, done state kept, rejection reason on its row | `prompts/04-edit-row.json` |
| `05-load-error.png` | J1 load failure — stated "could not load" + Retry, distinct from empty | `prompts/05-load-error.json` |
| `06-missing-todo.png` | J6 stale operation — per-row "todo does not exist", no success pretense | `prompts/06-missing-todo.json` |

## Generation recipe

POST the caption JSON (stringified) as `prompt` to `http://192.168.0.110:8020/sdapi/v1/txt2img`
with `{"model":"ming-image","width":1024,"height":1024,"steps":12,"cfg":1,"sampler":"euler"}`.

`jq -r .images[0] resp.json | base64 -d > out.png` — captions are Ming-Image structured
Figma-style JSON; each prompt file holds the exact caption used, so every rendered string is reproducible.
