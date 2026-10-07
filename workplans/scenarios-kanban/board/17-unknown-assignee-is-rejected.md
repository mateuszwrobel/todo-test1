# board: Unknown assignee is rejected

Source: workplans/workplan_board_store.md — Scenario: Unknown assignee is rejected

## Scenario
Given any card
When a change carries an assignee outside the roster
Then the unknown-assignee error answers before any write — roster validity outranks the text rules, the not-found lookup, and the freeze
  And the board is unchanged cell-for-cell

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
