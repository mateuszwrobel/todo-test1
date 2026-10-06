# ui: Triggered control blocks while in flight

Source: workplans/workplan_ui_board.md — Scenario: Triggered control blocks while in flight

## Scenario
Given the page shows the board
When the user triggers any card operation and triggers the same control again before the response arrives
Then only one request went out
  And the control becomes ready again exactly when the response arrives

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
