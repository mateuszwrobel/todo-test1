# ui: Abandoned drag changes nothing

Source: workplans/workplan_ui_board.md — Scenario: Abandoned drag changes nothing

## Scenario
Given the page shows the board
When the user starts a drag and drops outside any valid target (or releases without a drop)
Then no request is sent and the board renders unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
