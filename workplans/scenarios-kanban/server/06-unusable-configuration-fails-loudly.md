# server: Unusable configuration fails loudly

Source: workplans/workplan_server_board_composition.md — Scenario: Unusable configuration fails loudly

## Scenario
Given the configuration names a path that cannot be opened (unreadable directory, uncreatable file)
When the server starts
Then startup fails with a stated error and no partially serving process remains

## Done when
- The stated behavior observably holds for the module's contract in this module
- Repo architecture gate green: archspec verify --strict
