# api: Patch empty body is a stated 422

Source: workplans/workplan_api_board.md — Scenario: Patch empty body is a stated 422

## Scenario
Given a card exists with identifier N
When a client patches an empty JSON object to /cards/N
Then the response is 422 stating that at least one field is required
  And the card is unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
