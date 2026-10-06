# ui: Load failure renders as stated failure

Source: workplans/workplan_ui_board.md — Scenario: Load failure renders as stated failure

## Scenario
Given GET /board fails (server unreachable or non-200)
When the user opens or reloads the page
Then the page states that the board could not be loaded
  And it does not render empty columns as if they were the truth
  And a reload that succeeds renders the board normally

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
