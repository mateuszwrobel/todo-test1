# api: Users roster contract

Source: workplans/workplan_api_board.md — Scenario: Users roster contract

## Scenario
Given the server is running
When GET /users is requested
Then 200 answers with the five roster names in fixed order

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
