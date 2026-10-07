# api: PATCH assignee returns the card

Source: workplans/workplan_api_board.md — Scenario: PATCH assignee returns the card

## Scenario
Given a card that is not in Done
When PATCH /cards/{id} carries {"assignee":"Grace"}
Then 200 answers with the full card including that assignee and unchanged column and position
When a later PATCH carries {"assignee":null}
Then the card answers unassigned
When the field carries a name outside the roster
Then 422 answers with {"error":"unknown user"} and the card is unchanged
When the card is in Done and the request carries a title or an assignee
Then 422 answers with {"error":"cannot edit a done card"}

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
