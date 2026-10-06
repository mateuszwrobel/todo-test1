# ui: Edit updates in place

Source: workplans/workplan_ui_board.md — Scenario: Edit updates in place

## Scenario
Given the page shows a card in any column
When the user edits the card's text and saves
Then the card shows the new text at the same position in the same column
  And a rejected edit (blank or over-long) leaves the card's original text visible with the stated reason

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
