# api: Create over-limit is a stated 422

Source: workplans/workplan_api_board.md — Scenario: Create over-limit is a stated 422

## Scenario
Given an open board
When a client posts a title longer than 500 characters to /cards
Then the response is 422 stating the character limit
  And the board is unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
