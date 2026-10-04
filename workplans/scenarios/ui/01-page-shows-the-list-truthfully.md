# ui: Page shows the list truthfully

Source: workplans/workplan_ui_page.md — Scenario: Page shows the list truthfully

## Scenario
Given the store contains todos with mixed done states
When the user opens the page
Then every todo renders as a row in creation order, oldest first
  And each row shows its text and its done state readably
  And every row offers done-toggle and delete controls
  And only not-done rows offer an edit control

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
