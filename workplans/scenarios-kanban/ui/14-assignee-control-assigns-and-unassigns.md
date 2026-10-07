# ui: Assignee control assigns and unassigns

Source: workplans/workplan_ui_board.md — Scenario: Assignee control assigns and unassigns

## Scenario
Given a card in To Do or In Progress
When the user opens its edit band and picks a roster name
Then exactly one change request leaves and the card now shows that person's chip
When the band picks "Unassigned"
Then the chip disappears and the card shows no assignee
  And an active filter is left untouched

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
