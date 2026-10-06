# board: Delete removes and closes the gap

Source: workplans/workplan_board_store.md — Scenario: Delete removes and closes the gap

## Scenario
Given a column holding cards [A, B, C] with positions 0, 1, 2
When card B is deleted
Then the column holds [A, C] with positions 0, 1
  And a later created card receives a fresh identifier — identifier B is never reused

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
