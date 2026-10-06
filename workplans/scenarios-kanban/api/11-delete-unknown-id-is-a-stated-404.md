# api: Delete unknown id is a stated 404

Source: workplans/workplan_api_board.md — Scenario: Delete unknown id is a stated 404

## Scenario
Given no card exists with identifier Z
When a client sends DELETE /cards/Z
Then the response is 404 with the error "no such card"

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
