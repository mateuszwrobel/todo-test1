# api: List todos

Source: workplans/workplan_api_http.md — Scenario: List todos

## Scenario
Given todos exist
When a GET /todos arrives
Then the response is 200 with the JSON array of todos ordered by id ascending

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
