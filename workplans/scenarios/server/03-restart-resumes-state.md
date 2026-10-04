# server: Restart resumes state

Source: workplans/workplan_server_composition.md — Scenario: Restart resumes state

## Scenario
Given todos exist in the data file
When the command is stopped and started again with the same path
Then the page and the JSON contract report the same todos with the same states

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
