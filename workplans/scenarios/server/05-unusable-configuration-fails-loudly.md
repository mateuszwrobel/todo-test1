# server: Unusable configuration fails loudly

Source: workplans/workplan_server_composition.md — Scenario: Unusable configuration fails loudly

## Scenario
Given the listen address is already in use or the data path cannot be opened
When the command is started
Then it exits with a non-zero code and a stated reason
  And no half-wired server is left listening

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
