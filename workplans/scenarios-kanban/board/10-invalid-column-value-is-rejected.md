# board: Invalid column value is rejected

Source: workplans/workplan_board_store.md — Scenario: Invalid column value is rejected

## Scenario
Given an open board
When a change sets a card's column to a value other than todo, in_progress, done
Then nothing changes and the store reports the column as invalid

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
