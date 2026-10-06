# server: Start serves board and page

Source: workplans/workplan_server_board_composition.md — Scenario: Start serves board and page

## Scenario
Given a fresh machine state — no board file, no todo data file
When the server starts
Then GET /board answers the three fixed empty columns
  And the page loads from the same process

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
