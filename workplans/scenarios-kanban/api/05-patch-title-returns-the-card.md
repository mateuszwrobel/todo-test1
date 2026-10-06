# api: Patch title returns the card

Source: workplans/workplan_api_board.md — Scenario: Patch title returns the card

## Scenario
Given a card exists with identifier N
When a client patches {"title": "new text"} to /cards/N
Then the response is 200
  And the body is the card with the new title, same column, same position, same id

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
