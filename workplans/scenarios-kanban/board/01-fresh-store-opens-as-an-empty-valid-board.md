# board: Fresh store opens as an empty valid board

Source: workplans/workplan_board_store.md — Scenario: Fresh store opens as an empty valid board

## Scenario
Given no data file exists at the store's path
When the store is opened
Then the board lists three columns in the order todo, in_progress, done
  And every column holds no cards

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
