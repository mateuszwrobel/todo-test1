# ui: Empty edit text keeps the original

Source: workplans/workplan_ui_page.md — Scenario: Empty edit text keeps the original

## Scenario
When the user submits an edit with empty or whitespace-only text
Then the row keeps its original text
  And the edit surface states that the text is required

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
