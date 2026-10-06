# api: Patch invalid column is a stated 422

Source: workplans/workplan_api_board.md — Scenario: Patch invalid column is a stated 422

## Scenario
Given a card exists with identifier N
When a client patches {"column": "someday"} to /cards/N
Then the response is 422 with the error "invalid column"
  And the card is unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
