```json
{"task":"UI mockups rework — 3 stricter captions","status":"done","date":"2026-10-04"}
```

- Regenerated `01-list-populated.png`, `04-edit-row.png`, `06-missing-todo.png` after visual check of the first pass; all three rendered ok on first attempt with the same ming-image pipeline.
- Defects fixed via stricter captions: strikethrough conflict (not-done row rendered struck text despite unchecked box), edit-row garbling (hallucinated second row in edit mode with garbled buttons/text), phantom row (the "todo does not exist" notice rendered as its own fifth row with a checkbox).
- Caption changes: done rows now say "checked blue box with white tick, single thin strikethrough line crossing the middle of the text"; not-done rows say "empty unchecked box, solid dark text with no strikethrough line on this row"; 04 states exactly four rows with exactly one input row and drops any error surface; 06 states the notice lives inside row 4 with no checkbox and no fifth row. See `designs/mockups/prompts/`.
