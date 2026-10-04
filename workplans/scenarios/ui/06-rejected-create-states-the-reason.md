# ui: Rejected create states the reason

Source: workplans/workplan_ui_page.md — Scenario: Rejected create states the reason

## Scenario
Given the page shows the list
When the user submits empty or whitespace-only text
Then no todo is created
  And the create area states that the text is required
  And the typed text stays in the input for the correcting submit

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
