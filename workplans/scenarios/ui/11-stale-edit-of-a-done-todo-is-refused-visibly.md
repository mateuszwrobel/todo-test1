# ui: Stale edit of a done todo is refused visibly

Source: workplans/workplan_ui_page.md — Scenario: Stale edit of a done todo is refused visibly

## Scenario
Given a page stale about a todo's done state shows it as not-done
When the user submits an edit for it
Then the rejection states that the todo is done and its text cannot be edited
  And the row keeps displaying its original text

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
