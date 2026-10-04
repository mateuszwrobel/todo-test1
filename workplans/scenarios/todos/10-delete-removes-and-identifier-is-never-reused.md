# todos: Delete removes and identifier is never reused

Source: workplans/workplan_todos_store.md — Scenario: Delete removes and identifier is never reused

## Scenario
Given the store contains a todo
When Delete is called for its identifier and then Create is called
Then the deleted todo never appears in List again
  And the new todo's identifier differs from the deleted one

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
