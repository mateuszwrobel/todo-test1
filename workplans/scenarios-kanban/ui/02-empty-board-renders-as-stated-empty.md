# ui: Empty board renders as stated empty

Source: workplans/workplan_ui_board.md — Scenario: Empty board renders as stated empty

## Scenario
Given the server holds no cards
When the user opens the page
Then the three columns render with their empty treatment — visibly an empty board, not a blank or broken page

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
