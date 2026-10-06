# ui: Every state renders in the gallery

Source: workplans/workplan_ui_board.md — Scenario: Every state renders in the gallery

## Scenario
Given the board's observable states — board with cards, empty columns, load failure, inline edit band, drag placeholder with drop indication, done treatment, and each stated error surface
When the component gallery route is opened with fixture data
Then every state renders from fixtures without a live server
  And the styled board follows the kanban mockups' structure: three fixed columns, card treatment, green-check done rendering

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
