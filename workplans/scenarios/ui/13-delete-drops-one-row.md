# ui: Delete drops one row

Source: workplans/workplan_ui_page.md — Scenario: Delete drops one row

## Scenario
Given the page shows several todos
When the user activates delete on one row
Then the swapped-in content omits exactly that todo
  And every other row keeps its text, done state, and relative order
  And deleting the last todo lands the page in the "no todos" state

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
