# server: Start serves both surfaces

Source: workplans/workplan_server_composition.md — Scenario: Start serves both surfaces

## Scenario
Given a data file path and a listen address
When the command is started
Then the page is served at GET / on that address
  And the JSON contract is served at /todos on the same address
  And page operations work end to end

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
