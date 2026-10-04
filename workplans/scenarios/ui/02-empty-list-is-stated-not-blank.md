# ui: Empty list is stated, not blank

Source: workplans/workplan_ui_page.md — Scenario: Empty list is stated, not blank

## Scenario
Given the store contains no todos
When the user opens the page
Then the page shows a distinct "no todos" state with the create control ready

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
