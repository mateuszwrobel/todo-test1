# server: First start imports existing todos once

Source: workplans/workplan_server_board_composition.md — Scenario: First start imports existing todos once

## Scenario
Given a todo data file exists holding not-done todos [A, B, C] and done todos [D, E]
  And the board has never been created
When the server starts
Then the board holds [A, B, C] as cards in the todo column in that order and [D, E] in the done column in that order
  And every card carries a fresh identifier
  And the todo data file is unchanged on disk
When the server restarts again
Then the board is exactly as it was — no card is re-imported

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
