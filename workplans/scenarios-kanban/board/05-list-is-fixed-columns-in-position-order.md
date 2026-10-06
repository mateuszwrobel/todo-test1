# board: List is fixed columns in position order

Source: workplans/workplan_board_store.md — Scenario: List is fixed columns in position order

## Scenario
Given a board holding cards spread across the three columns with positions 0..n-1 in each
When the board is listed
Then the columns arrive in the order todo, in_progress, done
  And each column's cards arrive top-to-bottom by position

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
