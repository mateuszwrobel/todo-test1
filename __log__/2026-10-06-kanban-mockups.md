```json
{"task": "kanban mockups J1–J7", "status": "done", "date": "2026-10-06"}
```

- 7 renders landed with captions: `designs/mockups/kanban/k1…k7.png` + `kanban/prompts/k1…k7.json`, one per kanban journey J1–J7 from `workplans/user-journeys_kanban.md`; each copy byte-identical to its source in the render drop (`cmp` clean). README now leads with the kanban table as the current set.
- k6/k7 reroll history recorded in the README rendering notes: k6 needed two rerolls (the "Cancel" label produced ghost fragments — dropped from the caption; a seed reroll then duplicated a row); k7's first caption repeated per-row controls and duplicated label rows — simplified to hover-only controls on the acted row. All other mockups accepted on first seeded draw. Residual artifacts (checkbox glyph leaks beside Edit/Delete, occasional letter slips) noted honestly rather than captioned around.
- Generation recipe unchanged — same Ming-Image txt2img endpoint and caption format as the todo-app set; old table kept intact under a "Todo app mockups (superseded)" heading marked per ADR-003 so the history stays browsable.
