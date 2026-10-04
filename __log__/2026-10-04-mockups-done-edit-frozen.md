```json
{"task":"mockups updated — done rows hide edit","status":"done","date":"2026-10-04"}
```

- Regenerated `01-list-populated.png`, `03-create-error.png`, `04-edit-row.png`, `06-missing-todo.png` (ming-image, same pipeline) so the frozen-done-text rule reads visually: done rows show checkbox + strikethrough + Delete only, no Edit; not-done rows keep checkbox + Edit + Delete. Prompts updated with explicit per-row button wording and declared render counts ("Edit" on the not-done rows only, "Delete" on every row).
- 04 keeps the save-on-Save-only edit row (inline input prefilled "Walk the dog in the park"); 06 keeps the three-todo simplification with the red banner above the list; 02/05 untouched — list states unaffected by the rule.
- Render artifacts noted and documented in `designs/mockups/README.md`: ghost stray labels near rows in 01/04, one phantom empty row in 03, and 06's done rows leaked an Edit label — diffusion text-render limitation, not a caption omission; captions carry the rule explicitly.
