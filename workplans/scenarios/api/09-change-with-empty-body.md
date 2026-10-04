# api: Change with empty body

Source: workplans/workplan_api_http.md — Scenario: Change with empty body

## Scenario
When a PATCH /todos/{id} with an empty JSON object arrives
Then the response is 422 stating that at least one field is required

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
