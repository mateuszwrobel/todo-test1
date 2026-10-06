# board: Create rejects blank text

Source: workplans/workplan_board_store.md — Scenario: Create rejects blank text

## Scenario
Given an open board
When a card with empty or whitespace-only text is created
Then no card is created and the store reports the text as invalid (required)

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
