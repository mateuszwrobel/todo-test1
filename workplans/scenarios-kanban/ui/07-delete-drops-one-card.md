# ui: Delete drops one card

Source: workplans/workplan_ui_board.md — Scenario: Delete drops one card

## Scenario
Given the page shows a card
When the user activates that card's delete control
Then the card disappears and its column's remaining cards keep their order with no gap
  And every other card on the board is untouched

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
