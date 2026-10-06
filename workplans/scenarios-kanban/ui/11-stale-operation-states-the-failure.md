# ui: Stale operation states the failure

Source: workplans/workplan_ui_board.md — Scenario: Stale operation states the failure

## Scenario
Given the page shows a card that the server no longer holds
When the user edits, drags, or deletes that card
Then the operation is not applied — the card is not left looking as if it changed
  And the page states that the card does not exist
  And a reload renders the server's truth without that card

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
