# server: Board survives server restart

Source: workplans/workplan_server_board_composition.md — Scenario: Board survives server restart

## Scenario
Given the server ran, the board accumulated cards in a mix of columns and positions, and the process stopped
When the server starts again at the same paths
Then the board lists the same cards with the same texts, columns, and positions

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
