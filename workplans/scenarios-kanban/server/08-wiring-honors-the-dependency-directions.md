# server: Wiring honors the dependency directions

Source: workplans/workplan_server_board_composition.md — Scenario: Wiring honors the dependency directions

## Scenario
Given all four packages (board, api, ui, server) are present
When the architecture gate runs
Then the dependency directions of the parent plan hold: ui→api via HTTP only, api→board in-process, server composing all, nothing importing server or reading data files outside its owner

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
