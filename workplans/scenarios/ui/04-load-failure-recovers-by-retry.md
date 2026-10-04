# ui: Load failure recovers by retry

Source: workplans/workplan_ui_page.md — Scenario: Load failure recovers by retry

## Scenario
Given the page is in the load-failure state
When the user activates retry
Then the list is re-read and rendered, or the failure state is restated

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
