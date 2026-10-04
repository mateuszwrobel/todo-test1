# todos: Change title of a not-done todo keeps done state

Source: workplans/workplan_todos_store.md — Scenario: Change title of a not-done todo keeps done state

## Scenario
Given the store contains a not-done todo
When Change is called for its identifier with a new title only
Then the result is the todo with the new title and still not-done

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
