# server: Fresh path starts empty

Source: workplans/workplan_server_composition.md — Scenario: Fresh path starts empty

## Scenario
Given no data file exists at the configured path
When the command is started
Then it starts successfully and the page shows the "no todos" state

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
