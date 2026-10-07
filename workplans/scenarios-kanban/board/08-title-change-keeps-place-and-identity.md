# board: Title change keeps place and identity

Source: workplans/workplan_board_store.md — Scenario: Title change keeps place and identity

## Scenario
Given a board holding a card in in_progress at position 1 with identifier N
When the card's text is changed
Then the card shows the new text in in_progress at position 1 with identifier N

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict

## Amendment 2026-10-07 — done freeze

Amendment 2026-10-07: done freeze — title edits refused on cards in Done; edit affordance absent on Done cards (user decision; supersedes this card's done-editable legs; the original text above stands as contract history).
