# todos: List is creation order

Source: workplans/workplan_todos_store.md — Scenario: List is creation order

## Scenario
Given the store contains several todos
When List is called
Then todos are returned in ascending identifier order, oldest first

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
