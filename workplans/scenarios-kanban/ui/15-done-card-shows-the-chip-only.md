# ui: Done card shows the chip only

Source: workplans/workplan_ui_board.md — Scenario: Done card shows the chip only

## Scenario
Given an assigned card in Done
Then the card shows its chip with the name
  And no assign control appears anywhere on the card
  And forcing an assignee change over the seam states the refusal at the card
When the card is dragged out of Done
Then the edit band offers the assignee select again

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
