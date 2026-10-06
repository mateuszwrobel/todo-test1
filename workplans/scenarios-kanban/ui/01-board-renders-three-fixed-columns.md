# ui: Board renders three fixed columns

Source: workplans/workplan_ui_board.md — Scenario: Board renders three fixed columns

## Scenario
Given the server holds cards in all three columns
When the user opens the page
Then three column panels appear in the order "To Do", "In Progress", "Done"
  And each panel lists its cards top-to-bottom exactly as the server's arrays order them
  And cards in "Done" render with the done treatment while other cards render plain
  And no done checkbox or toggle exists anywhere on the page

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
