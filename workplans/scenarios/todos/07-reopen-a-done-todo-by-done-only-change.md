# todos: Reopen a done todo by done-only change

Source: workplans/workplan_todos_store.md — Scenario: Reopen a done todo by done-only change

## Scenario
Given the store contains a done todo
When Change is called for its identifier with done false only
Then the result is the todo, not-done, text unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
