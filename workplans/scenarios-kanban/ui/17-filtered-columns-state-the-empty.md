# ui: Filtered columns state the empty

Source: workplans/workplan_ui_board.md — Scenario: Filtered columns state the empty

## Scenario
Given a filter under which one column holds no matching cards
When the filtered view renders
Then that column shows the stated empty treatment with the filter named

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
