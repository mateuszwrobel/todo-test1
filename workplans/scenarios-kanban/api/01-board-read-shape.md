# api: Board read shape

Source: workplans/workplan_api_board.md — Scenario: Board read shape

## Scenario
Given a board holding cards in all three columns
When a client sends GET /board
Then the response is 200
  And the body carries columns in the fixed order todo, in_progress, done with their display titles
  And each column's cards array is in position order with id, title, column, position

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
