# todos: Change title on a done todo is refused

Source: workplans/workplan_todos_store.md — Scenario: Change title on a done todo is refused

## Scenario
Given the store contains a done todo
When Change is called for its identifier with a title
Then the result is a done-frozen outcome
  And the todo's text and done state are unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
