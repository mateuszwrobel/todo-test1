# ui: Inline edit updates in place

Source: workplans/workplan_ui_page.md — Scenario: Inline edit updates in place

## Scenario
Given the page shows a not-done todo
When the user activates edit on the row, changes the text, and saves
Then the row shows the new text without a full reload
  And the todo remains not-done and in the same position

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
