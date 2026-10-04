# api: Change missing todo

Source: workplans/workplan_api_http.md — Scenario: Change missing todo

## Scenario
Given no todo exists with identifier X
When a PATCH /todos/X arrives
Then the response is 404 with `{ "error": "no such todo" }`

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
