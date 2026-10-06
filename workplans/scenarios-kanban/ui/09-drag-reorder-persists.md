# ui: Drag reorder persists

Source: workplans/workplan_ui_board.md — Scenario: Drag reorder persists

## Scenario
Given a column shows at least three cards
When the user drags a card to a new position within the same column
Then the column re-renders in the new order immediately
  And after a page reload the new order is still shown

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
