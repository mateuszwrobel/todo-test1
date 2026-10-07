# ui: Assignment and filter states in the gallery

Source: workplans/workplan_ui_board.md — Scenario: Assignment and filter states in the gallery

## Scenario
Given the component gallery route
When it is opened
Then it shows an assigned card, an unassigned card, a Done card with chip and no assign control, an edit band with the assignee select, and a filtered column with its empty treatment
  And the visual baselines cover these anchors

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
