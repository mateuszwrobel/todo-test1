# ui: Done rows carry no edit control

Source: workplans/workplan_ui_page.md — Scenario: Done rows carry no edit control

## Scenario
Given the page freshly loads a list containing a done todo
Then that row shows no edit affordance
  And its text can only change after reopening via the toggle

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
