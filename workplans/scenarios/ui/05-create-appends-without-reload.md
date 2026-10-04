# ui: Create appends without reload

Source: workplans/workplan_ui_page.md — Scenario: Create appends without reload

## Scenario
Given the page shows the list
When the user types text into the create input and submits
Then the response swaps in fresh list content without a full page reload
  And the new todo is the last row, marked not-done
  And the create input and Add button are ready for the next todo

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
