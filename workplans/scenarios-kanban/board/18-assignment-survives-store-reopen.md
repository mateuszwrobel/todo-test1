# board: Assignment survives store reopen

Source: workplans/workplan_board_store.md — Scenario: Assignment survives store reopen

## Scenario
Given a board with assigned and unassigned cards
When the store is closed and reopened
Then every card carries exactly the assignee it carried before

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
