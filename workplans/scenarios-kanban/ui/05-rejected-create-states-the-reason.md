# ui: Rejected create states the reason

Source: workplans/workplan_ui_board.md — Scenario: Rejected create states the reason

## Scenario
Given the page shows the board
When the user submits a create that the server rejects (blank or over-long text)
Then no card appears
  And the stated reason appears at the create control — "text is required" or the character limit

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
