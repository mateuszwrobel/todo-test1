# ui: Filter dropdown filters the board

Source: workplans/workplan_ui_board.md — Scenario: Filter dropdown filters the board

## Scenario
Given the board page
When the filter control is opened
Then it lists "All users", each roster name, and "Unassigned" — exactly the roster
When a name is chosen
Then the board shows only that person's cards in their columns and order, the URL gains the assignee parameter, and a reload keeps the filtered view
When "All users" is chosen
Then the parameter leaves the URL and the full board returns

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
