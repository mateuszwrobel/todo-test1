# todos: State survives reopen

Source: workplans/workplan_todos_store.md — Scenario: State survives reopen

## Scenario
Given todos exist with mixed done states
When the store is closed and a new store instance is opened on the same file
Then List returns the same todos with the same texts and done states

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
