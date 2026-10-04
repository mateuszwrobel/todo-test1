# ui: Toggle marks done and reopens

Source: workplans/workplan_ui_page.md — Scenario: Toggle marks done and reopens

## Scenario
Given the page shows a not-done todo
When the user activates its done toggle
Then the row shows the todo as done without a full reload
  And activating the same toggle again shows it not-done
  And the row's text and position never change from toggling

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
