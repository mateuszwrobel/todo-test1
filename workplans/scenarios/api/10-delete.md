# api: Delete

Source: workplans/workplan_api_http.md — Scenario: Delete

## Scenario
When a DELETE /todos/{id} arrives for an existing todo
Then the response is 204 with no body
  And a later GET does not include it

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
