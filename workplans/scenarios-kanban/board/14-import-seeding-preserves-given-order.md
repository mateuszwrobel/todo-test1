# board: Import seeding preserves given order

Source: workplans/workplan_board_store.md — Scenario: Import seeding preserves given order

## Scenario
Given an empty board
When cards are seeded in bulk — a list of texts with a target column
Then the column holds exactly those texts top-to-bottom in the given order
  And each card carries a fresh identifier

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
