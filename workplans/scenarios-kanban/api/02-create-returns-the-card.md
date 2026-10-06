# api: Create returns the card

Source: workplans/workplan_api_board.md — Scenario: Create returns the card

## Scenario
Given an open board
When a client posts {"title": "Buy milk"} to /cards
Then the response is 201
  And the body is the created card with column "todo", the bottom position, and a fresh id

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
