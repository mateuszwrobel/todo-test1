# board: Reorder within a column

Source: workplans/workplan_board_store.md — Scenario: Reorder within a column

## Scenario
Given a column holding cards [A, B, C] at positions 0, 1, 2
When card A is moved to position 2 of the same column
Then the column holds [B, C, A] with positions 0, 1, 2

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
