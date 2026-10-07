# users: Roster lists the simulation names

Source: workplans/workplan_users_roster.md — Scenario: Roster lists the simulation names

## Scenario
Given the program is built
When the roster is asked for its names
Then it answers Ada, Grace, Alan, Barbara, Linus — in that order, every time
  And membership holds for exactly those five names, case-sensitively
  And every other string — including empty and case variants like "grace" — is not a member

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
