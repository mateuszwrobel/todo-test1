# board: Title change keeps place and identity

Source: workplans/workplan_board_store.md — Scenario: Title change keeps place and identity

## Scenario
Given a board holding a card in in_progress at position 1 with identifier N
When the card's text is changed
Then the card shows the new text in in_progress at position 1 with identifier N

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
