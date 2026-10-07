# api: Patch title returns the card

Source: workplans/workplan_api_board.md — Scenario: Patch title returns the card

## Scenario
Given a card exists with identifier N
When a client patches {"title": "new text"} to /cards/N
Then the response is 200
  And the body is the card with the new title, same column, same position, same id

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict

## Amendment 2026-10-07 — done freeze

Amendment 2026-10-07: done freeze — title edits refused on cards in Done; edit affordance absent on Done cards (user decision; supersedes this card's done-editable legs; the original text above stands as contract history).
