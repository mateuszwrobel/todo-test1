# ui: Over-limit create states the limit

Source: workplans/workplan_ui_page.md — Scenario: Over-limit create states the limit

## Scenario
When the user submits text longer than 500 characters
Then no todo is created
  And the create area states the 500-character limit

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
