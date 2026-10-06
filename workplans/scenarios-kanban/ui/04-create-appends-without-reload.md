# ui: Create appends without reload

Source: workplans/workplan_ui_board.md — Scenario: Create appends without reload

## Scenario
Given the page shows the board
When the user types text into the create input and submits
Then the new card appears at the bottom of "To Do" without a page reload
  And the input and Add button return to ready for the next card

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
