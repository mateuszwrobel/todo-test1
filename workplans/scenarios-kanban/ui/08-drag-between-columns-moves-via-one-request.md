# ui: Drag between columns moves via one request

Source: workplans/workplan_ui_board.md — Scenario: Drag between columns moves via one request

## Scenario
Given the page shows a card in "To Do" and cards in "In Progress"
When the user drags the card into "In Progress" between two cards and releases
Then one update request carries the target column and drop position
  And the card renders in "In Progress" at the drop position and is gone from "To Do"
  And during the drag the drop position is indicated before release

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
