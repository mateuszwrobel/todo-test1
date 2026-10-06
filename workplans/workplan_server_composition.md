# Workplan: Server Module — Composition Root and Entry Point

> **Status: superseded** by [workplan_kanban_application.md](workplan_kanban_application.md) — see [ADR-003](../docs/adr/ADR-003-kanban-pivot.md). Kept as history.

## Goal

The application must run as one command that starts everything and owns nothing else's behavior. The server module (cmd/todo) is the composition root: it opens the store at the configured path, wires the api handler and the ui handlers together — including the base URL the ui uses to reach the api contract — mounts them on one HTTP listener, and owns that lifecycle from start to clean shutdown. Observable outcome: one command serves the page and the JSON contract on one address; Ctrl-C stops it cleanly; restart resumes the same state.

## Acceptance Criteria

### Scenario: Start serves both surfaces
Given a data file path and a listen address
When the command is started
Then the page is served at GET / on that address
  And the JSON contract is served at /todos on the same address
  And page operations work end to end

### Scenario: Fresh path starts empty
Given no data file exists at the configured path
When the command is started
Then it starts successfully and the page shows the "no todos" state

### Scenario: Restart resumes state
Given todos exist in the data file
When the command is stopped and started again with the same path
Then the page and the JSON contract report the same todos with the same states

### Scenario: Clean shutdown completes in-flight work
Given a request is being processed
When shutdown is signaled
Then in-flight requests finish their responses
  And the store is closed so completed operations are durable
  And the process exits without error

### Scenario: Unusable configuration fails loudly
Given the listen address is already in use or the data path cannot be opened
When the command is started
Then it exits with a non-zero code and a stated reason
  And no half-wired server is left listening

### Scenario: Wiring honors the dependency directions
Given the composed server is running
When any page operation is performed
Then page requests reach todos data exclusively through the api contract over HTTP
  And no module other than the store's own code opens the data file

## Decisions
- One command, one binary, one listener; page, fragment, and JSON routes mount on it — Rationale: parent workplan states the server also serves the page; one process is the whole deploy. Rejected: separate static host or second process.
- Configuration is two values with defaults: listen address (default 127.0.0.1:8080, overridable) and data file path (default todos.db beside the command, overridable) — Rationale: the app's Assumptions put it local and single-user; flags/ENV beyond these two are forecasting. Rejected: config files, per-route configuration.
- Wiring order is fixed: open store → construct api handler over the store → construct ui handlers with the api's base URL (the listen address itself) → register routes → listen — Rationale: one owner for the lifecycle chain (principle 8); the ui→api edge exists only as HTTP against a listening server, which the spec gate enforces as import-free. Rejected: lazy wiring (unowned lifecycle), in-process injection of the api into ui (violates the parent boundary).
- Routes mount at / (page), /ui/* and /static/* (ui fragments and asset), /todos (api JSON); the composition root owns the routing table, no module owns a listener or a router — Rationale: mounting is wiring, which is exactly what a composition root hides. Rejected: api or ui self-registering via package side effects.
- Graceful shutdown: stop accepting, drain in-flight requests, close the store, exit — Rationale: "completed operations survive process death" is the store contract only when shutdown lets committed writes finish. Rejected: abrupt exit (risks the durability scenario).
- This module contains no business rule, no template, and no status-code mapping — Rationale: every behavior belongs to exactly one functional module; the root that also holds rules is a second god module. Rejected: convenience helpers that re-implement module decisions.

## Assumptions
- The composition root runs with read/write access to the data path and the right to bind the address — Depends: local single-user use (parent Assumptions). If wrong: startup fails loudly per the loud-failure scenario.

## Risks
- Base-URL wiring pointing at an address the listener does not serve — Impact: every page operation fails while the JSON contract works. Mitigation: the ui handlers derive their base URL from the same listen address the root binds; e2e's page journeys fail loudly if they drift.
- Shutdown draining forever on a stuck connection — Impact: Ctrl-C appears ignored. Mitigation: bounded drain; after the bound, remaining requests are answered as failed and the store still closes (completed operations stay durable).

## CLI

### Proposed Commands
| Command | Purpose |
|---------|---------|
| todo | Start the composed server (the only command) |

### Flags / Options
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| --addr | string | 127.0.0.1:8080 | Listen address for page and JSON contract together |
| --db | string | todos.db | Data file path for the store |

### Exit Codes
| Code | Meaning | When Produced |
|------|---------|---------------|
| 0 | Clean shutdown | SIGINT/SIGTERM after successful start |
| 1 | Startup failure | Address in use, data path unusable, malformed flag values |

## API
No new endpoints. This module mounts the api module's /todos contract and the ui module's /, /ui/*, /static/* surfaces on the listener it owns; route-to-module mapping is its entire API knowledge.

## Database
None owned here: the store is opened at the configured path by the todos module's Open operation and handed to the api wiring — the root triggers creation, the todos module owns everything after.

## Modularity

### Behavior Analysis
- Owning the application lifecycle — open, wire, serve, drain, close — the single reason this module changes.

### Module Placement
| Behavior | Fits Existing Module | New Module | Reason |
|----------|---------------------|------------|--------|
| Lifecycle + wiring + mounting | none (greenfield) | server | hides one decision: how the modules are assembled and live |

### Internal Architecture
Plain procedural composition — main(), flag parsing, constructor calls, route table, signal handling. No plugin machinery; a composition root with its own framework is a second system.

### Boundaries
- Imports all three functional modules (the only module permitted to import api and ui together — the spec gate states it).
- Owns the listener and the store handle lifecycle; hands out no mutable state after wiring completes.
- Cross-module dependency directions land exactly as architecture.spec.toml declares: ui → (HTTP only) api → todos; server → all three.
