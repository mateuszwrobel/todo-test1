# api: Delete answers 204

Source: workplans/workplan_api_board.md — Scenario: Delete answers 204

## Scenario
Given a card exists with identifier N
When a client sends DELETE /cards/N
Then the response is 204 with no body
  And a later GET /board omits the card

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
