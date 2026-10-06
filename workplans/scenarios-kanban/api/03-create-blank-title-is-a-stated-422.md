# api: Create blank title is a stated 422

Source: workplans/workplan_api_board.md — Scenario: Create blank title is a stated 422

## Scenario
Given an open board
When a client posts an empty or whitespace-only title to /cards
Then the response is 422 with the error "title is required"
  And the board is unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
