# board: Create rejects over-long text

Source: workplans/workplan_board_store.md — Scenario: Create rejects over-long text

## Scenario
Given an open board
When a card with text longer than 500 characters is created
Then no card is created and the store reports the limit as exceeded

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
