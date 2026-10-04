# todos: Change with no fields is invalid

Source: workplans/workplan_todos_store.md — Scenario: Change with no fields is invalid

## Scenario
Given the store is open
When Change is called with neither title nor done supplied
Then the result is an invalid outcome
  And no state changes

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
