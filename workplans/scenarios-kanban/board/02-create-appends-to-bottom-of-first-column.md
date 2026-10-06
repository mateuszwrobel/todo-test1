# board: Create appends to bottom of first column

Source: workplans/workplan_board_store.md — Scenario: Create appends to bottom of first column

## Scenario
Given a board holding two cards in the todo column
When a card with the text "Buy milk" is created
Then the todo column lists three cards with "Buy milk" last at position 2
  And the card carries an identifier assigned by the store
  And the card's column is todo

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
