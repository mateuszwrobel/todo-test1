# api: Create with blank title

Source: workplans/workplan_api_http.md — Scenario: Create with blank title

## Scenario
When a POST /todos with empty or whitespace-only title arrives
Then the response is 422 with `{ "error": "title is required" }`
  And no todo is created

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
