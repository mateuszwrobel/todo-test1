# ui: Missing todo states the failure for any operation

Source: workplans/workplan_ui_page.md — Scenario: Missing todo states the failure for any operation

## Scenario
Given a page stale about a todo that no longer exists
When the user toggles, edits, or deletes it
Then no change occurs anywhere
  And the page states that the todo does not exist
  And no row is left looking like the failed operation succeeded

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
