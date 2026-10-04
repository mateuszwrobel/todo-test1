# api: Create succeeds

Source: workplans/workplan_api_http.md — Scenario: Create succeeds

## Scenario
Given the service is running
When a POST /todos with body `{ "title": "Buy milk" }` arrives
Then the response is 201 with the created todo's JSON, done false and a fresh id

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
