# todos: Create rejects invalid text

Source: workplans/workplan_todos_store.md — Scenario: Create rejects invalid text

## Scenario
Given the store is open
When Create is called with empty, whitespace-only, or longer-than-500-character text
Then the result is an invalid-text outcome
  And no state changes

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
