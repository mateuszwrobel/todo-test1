# api: Board filtered by assignee

Source: workplans/workplan_api_board.md — Scenario: Board filtered by assignee

## Scenario
Given a board mixing assigned and unassigned cards
When GET /board carries ?assignee=Grace
Then 200 answers the same board shape with only Grace's cards, stored order and positions unchanged
When it carries ?assignee=unassigned
Then only the cards without an assignee appear
When it carries an assignee value outside the roster
Then 422 answers with {"error":"unknown user"}
When it carries no assignee parameter
Then the answer is the full board, unchanged from today

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
