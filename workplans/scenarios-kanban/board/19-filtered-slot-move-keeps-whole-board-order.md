# board: Filtered-slot move keeps whole-board order

Source: workplans/workplan_board_store.md — Scenario: Filtered-slot move keeps whole-board order

## Scenario
Given a column whose cards interleave those matching an assignee filter with those not matching it
When a card is moved with a target column and a slot counted among the matching cards only
Then the card lands at that slot relative to the matching cards
  And the non-matching cards keep their relative order cell-for-cell
  And positions stay contiguous 0..n-1 in every column
  And slot 0 into a column holding no matching cards places the card at that column's front

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
