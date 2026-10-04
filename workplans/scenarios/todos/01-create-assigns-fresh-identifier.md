# todos: Create assigns fresh identifier

Source: workplans/workplan_todos_store.md — Scenario: Create assigns fresh identifier

## Scenario
Given the store contains no todo with the text "Buy milk"
When Create is called with that text
Then the result is a todo with that text, done false, and an identifier never used before
  And the next List includes it

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
