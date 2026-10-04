# api: Create over length limit

Source: workplans/workplan_api_http.md — Scenario: Create over length limit

## Scenario
When a POST /todos with a title longer than 500 characters arrives
Then the response is 422 stating the 500-character limit
  And no todo is created

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
