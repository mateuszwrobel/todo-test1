# Workplan: Users Roster (simulated users)

## Goal

The app ships a fixed cast of simulated people — Ada, Grace, Alan, Barbara, Linus. Cards can be assigned to exactly one of them; the roster is the rule for who exists in the simulation. No authentication, no user accounts, no user storage: this is the todo-app replacement's "pretend there's a team" increment, so assignment and filtering have a stable cast. Anyone who later wants real accounts replaces this module; nothing else changes.

## Acceptance Criteria

### Scenario: Roster lists the simulation names
Given the program is built
When the roster is asked for its names
Then it answers Ada, Grace, Alan, Barbara, Linus — in that order, every time
  And membership holds for exactly those five names, case-sensitively
  And every other string — including empty and case variants like "grace" — is not a member

## Decisions
- Roster is a built-in constant list — Rationale: the user asked for simulation, not accounts; a db table or config file is machinery nobody needs yet. Rejected: editable roster (second product), config-file loading (flag surface for no behavior).
- Matching is case-sensitive exact — Rationale: names are program constants; lenient matching invents a rule nobody stated. Rejected: case-insensitive or trimmed comparison.
- Names are exposed as an ordered list and the order is contract — Rationale: dropdowns and galleries list the roster in one fixed order everywhere. Rejected: set semantics with arbitrary iteration order.

## Assumptions
- The cast never changes at runtime — Depends: everything that lists users. If wrong: the module is rewritten to load a roster; callers keep the same contract.

## Risks
- Roster later grows real accounts — Impact: this module is replaced by an auth/user module. Mitigation: callers use only Names/membership contracts, never the constant directly.

## Open Questions
- None.

## Modularity

### Behavior Analysis
- The simulation's cast — who exists, in what display order — changes when the simulation changes (new cast, real accounts). Independent of board data, wire contract, and page.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Roster of simulated users | board (would mix "who exists" into "what the board stores") | users | own reason to change: the cast evolves separately from card mechanics |

### Internal Architecture
- users — plain data module: one constant slice and a membership check. No state, no I/O; no architecture ceremony.

### Boundaries
- users imports nothing.
- board imports users to validate assignee values; api serves the roster through GET /users; composition wires nothing new (no data file).
- ui never imports users — it receives names over the contract, keeping the page server-driven.
