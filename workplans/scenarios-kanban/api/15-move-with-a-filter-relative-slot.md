# api: Move with a filter-relative slot

Source: workplans/workplan_api_board.md — Scenario: Move with a filter-relative slot

## Scenario
Given a filtered view whose visible cards sit among hidden ones
When PATCH /cards/{id} carries {"column":"doing","slot":1,"within":"Grace"}
Then 200 answers with the moved card and an absolute position that puts it at slot 1 among Grace's cards with the hidden cards unmoved
When the body carries "position" and "slot" together, or "slot" without "within"
Then 422 answers with a stated error
When "within" names a stranger to the roster
Then 422 answers with {"error":"unknown user"}

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
