# ui: Hidden truth under a filter

Source: workplans/workplan_ui_board.md — Scenario: Hidden truth under a filter

## Scenario
Given a filter active with hidden cards interleaved among visible ones
When the user drags a visible card to a visible slot
Then the view updates as moved
  And clearing the filter reveals the true interleaved order with the hidden cards untouched

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
