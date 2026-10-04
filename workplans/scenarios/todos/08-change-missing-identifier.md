# todos: Change missing identifier

Source: workplans/workplan_todos_store.md — Scenario: Change missing identifier

## Scenario
Given no todo exists with identifier X
When Change is called for X
Then the result is a not-found outcome

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
