# board: Delete of unknown card is reported

Source: workplans/workplan_board_store.md — Scenario: Delete of unknown card is reported

## Scenario
Given no card exists with identifier Z
When a delete targets identifier Z
Then nothing changes and the store reports no such card

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
