# board: State survives store close and reopen

Source: workplans/workplan_board_store.md — Scenario: State survives store close and reopen

## Scenario
Given a board holding cards in a mix of columns and positions
When the store is closed and opened again at the same path
Then the board lists the same cards with the same texts, columns, and positions

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
