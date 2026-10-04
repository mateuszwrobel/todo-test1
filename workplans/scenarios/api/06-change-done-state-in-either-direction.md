# api: Change done state in either direction

Source: workplans/workplan_api_http.md — Scenario: Change done state in either direction

## Scenario
When a PATCH /todos/{id} with `{ "done": true }` or `{ "done": false }` arrives
Then the response is 200 with the updated todo JSON and the title unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
