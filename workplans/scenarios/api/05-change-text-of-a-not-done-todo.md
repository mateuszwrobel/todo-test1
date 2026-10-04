# api: Change text of a not-done todo

Source: workplans/workplan_api_http.md — Scenario: Change text of a not-done todo

## Scenario
When a PATCH /todos/{id} with `{ "title": "new text" }` arrives for an existing not-done todo
Then the response is 200 with the updated todo JSON and done state unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
