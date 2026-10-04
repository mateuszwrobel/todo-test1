# todos: Change done state keeps title

Source: workplans/workplan_todos_store.md — Scenario: Change done state keeps title

## Scenario
Given the store contains a todo with text "Buy milk"
When Change is called for its identifier with done false only
Then the result is the todo with text unchanged and done false

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
