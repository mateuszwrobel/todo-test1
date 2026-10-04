# api: Title edit on done todo refused

Source: workplans/workplan_api_http.md — Scenario: Title edit on done todo refused

## Scenario
When a PATCH /todos/{id} carrying a title arrives for a done todo
Then the response is 422 with `{ "error": "cannot edit a done todo" }`
  And the todo is unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
