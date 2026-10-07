# board: Assigning and clearing the assignee keeps the card

Source: workplans/workplan_board_store.md — Scenario: Assigning and clearing the assignee keeps the card

## Scenario
Given a card on the board
When the card is changed with an assignee set to a roster name
Then the returned card carries that assignee
  And its column, position, and identifier are unchanged
When the card is later changed with the assignee cleared
Then the returned card carries no assignee
  And its column, position, and identifier are unchanged

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
