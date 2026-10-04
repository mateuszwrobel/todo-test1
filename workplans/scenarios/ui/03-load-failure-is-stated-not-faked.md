# ui: Load failure is stated, not faked

Source: workplans/workplan_ui_page.md — Scenario: Load failure is stated, not faked

## Scenario
Given the list cannot be read (server not reachable)
When the user opens or reloads the page
Then the page shows that the todos could not be loaded
  And it never shows an empty or stale list as if it were the truth

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
