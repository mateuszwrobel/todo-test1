# board: Assignment on a done card is frozen

Source: workplans/workplan_board_store.md — Scenario: Assignment on a done card is frozen

## Scenario
Given a card in the Done column
When a change carrying an assignee targets it
Then the freeze error answers and nothing is written
  And the same card moved out of Done accepts the assignee change afterwards

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
