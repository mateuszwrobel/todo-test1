# board: Move changes column and keeps neighbors' order

Source: workplans/workplan_board_store.md — Scenario: Move changes column and keeps neighbors' order

## Scenario
Given a todo column holding cards [A, B, C] and an in_progress column holding [X, Y]
When card B is moved to in_progress at position 1
Then in_progress holds [X, B, Y] with positions 0, 1, 2
  And todo holds [A, C] with positions 0, 1
  And B's text and identifier are unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
