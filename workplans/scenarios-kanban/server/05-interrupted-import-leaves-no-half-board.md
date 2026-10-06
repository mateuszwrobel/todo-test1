# server: Interrupted import leaves no half board

Source: workplans/workplan_server_board_composition.md — Scenario: Interrupted import leaves no half board

## Scenario
Given a todo data file exists and the board has never been created
When the server process stops hard during the import
Then the next start shows either the fully imported board or a clean board with no trace of a partial import
  And the import completes on that next start if it had not landed

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
