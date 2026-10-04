# todos: Fresh file opens as empty valid store

Source: workplans/workplan_todos_store.md — Scenario: Fresh file opens as empty valid store

## Scenario
Given no data file exists at the given path
When the store is opened at that path
Then the store is empty and all operations work

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
