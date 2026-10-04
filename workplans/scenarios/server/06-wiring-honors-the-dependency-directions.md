# server: Wiring honors the dependency directions

Source: workplans/workplan_server_composition.md — Scenario: Wiring honors the dependency directions

## Scenario
Given the composed server is running
When any page operation is performed
Then page requests reach todos data exclusively through the api contract over HTTP
  And no module other than the store's own code opens the data file

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
