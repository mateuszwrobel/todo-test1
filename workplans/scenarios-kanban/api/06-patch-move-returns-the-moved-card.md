# api: Patch move returns the moved card

Source: workplans/workplan_api_board.md — Scenario: Patch move returns the moved card

## Scenario
Given a card exists with identifier N in the todo column
When a client patches {"column": "in_progress", "position": 1} to /cards/N
Then the response is 200
  And the body is the card with column "in_progress" and position 1

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
