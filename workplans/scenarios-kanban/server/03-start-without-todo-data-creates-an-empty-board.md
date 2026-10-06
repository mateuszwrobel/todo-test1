# server: Start without todo data creates an empty board

Source: workplans/workplan_server_board_composition.md — Scenario: Start without todo data creates an empty board

## Scenario
Given no todo data file exists and the board has never been created
When the server starts
Then the board exists with the three fixed columns holding no cards

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
