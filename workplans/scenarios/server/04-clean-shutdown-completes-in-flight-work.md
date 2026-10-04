# server: Clean shutdown completes in-flight work

Source: workplans/workplan_server_composition.md — Scenario: Clean shutdown completes in-flight work

## Scenario
Given a request is being processed
When shutdown is signaled
Then in-flight requests finish their responses
  And the store is closed so completed operations are durable
  And the process exits without error

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
