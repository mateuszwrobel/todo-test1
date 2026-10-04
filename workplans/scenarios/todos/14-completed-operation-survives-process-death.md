# todos: Completed operation survives process death

Source: workplans/workplan_todos_store.md — Scenario: Completed operation survives process death

## Scenario
Given Create returns successfully
When the process is killed immediately after and restarted on the same file
Then the todo is present

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
