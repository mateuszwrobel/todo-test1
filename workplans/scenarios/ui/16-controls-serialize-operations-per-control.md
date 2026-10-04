# ui: Controls serialize operations per control

Source: workplans/workplan_ui_page.md — Scenario: Controls serialize operations per control

## Scenario
Given an operation triggered from a control is in flight
Then that control is disabled until the response arrives
  And a second activation of the same control while in flight causes no request

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
