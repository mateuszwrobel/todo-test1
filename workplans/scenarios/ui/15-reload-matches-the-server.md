# ui: Reload matches the server

Source: workplans/workplan_ui_page.md — Scenario: Reload matches the server

## Scenario
Given any sequence of successful operations has run from the page
When the user reloads
Then the rendered list equals a fresh read of the server state

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
