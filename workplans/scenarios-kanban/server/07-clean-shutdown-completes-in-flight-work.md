# server: Clean shutdown completes in-flight work

Source: workplans/workplan_server_board_composition.md — Scenario: Clean shutdown completes in-flight work

## Scenario
Given a request is being processed
When the server is asked to shut down
Then the in-flight request finishes normally and the board file is closed — no mutation is torn

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
