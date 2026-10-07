# server: Assignments survive restart

Source: workplans/workplan_server_board_composition.md — Scenario: Assignments survive restart

## Scenario
Given assignments set over HTTP across all columns, including an assigned card in Done and cards with no assignee
When the process restarts with the same data files
Then every card reports exactly the same assignee state as before

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
