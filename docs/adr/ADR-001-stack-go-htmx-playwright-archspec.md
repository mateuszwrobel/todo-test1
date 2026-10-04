# ADR-001: Implementation stack — Go, htmx, Playwright, archspec

## Status

Accepted — 2026-10-04

## Context

The todo application is greenfield (see `workplans/workplan_todo_application.md`): a
single-user local web app with four operations over one todo collection, persisted in one
embedded data file, served by one local process that also serves the browser page. The
workplan's Modularity section fixes the module shape (`todos`, `api`, `ui`, plus a
composition root) but deliberately leaves the stack open. The architecture needs a
mechanical gate so module boundaries cannot drift as code lands.

## Decision

- **Go for all backend code.** One language for the whole served system: the HTTP server,
  the todo data module, and the server-rendered page. Single static binary; the Go toolchain
  is the only build dependency.
- **htmx for UI interactivity.** The page is server-rendered HTML enhanced with htmx
  attributes; the server returns HTML fragments, not JSON, to the interactive controls.
  No SPA framework, no client-side build step.
- **Playwright for integration/e2e tests.** Browser-driven tests exercise the acceptance
  scenarios end to end against the running server. The `playwright` CLI with chromium is
  installed globally in the environment.
- **archspec as the architecture-test gate.** The architecture spec lives at the repo root
  as `architecture.spec.toml`; `archspec verify --strict` is the architecture test command
  and must pass for boundary work to be considered done. The go driver and toolchain are
  confirmed by `archspec doctor`. The spec is authored spec-first — before the Go modules
  exist, `verify` reports the expectations as unmet; that red state is the gate standing by,
  not a regression.

## Consequences

- One toolchain, one binary, one language for contributors and agents to work in; the
  composition root (`cmd/todo`) wires `todos`, `api`, and `ui`.
- Page interactivity is constrained to what hypermedia + htmx express — sufficient for the
  four operations and error display, which is all the app requires.
- E2E tests need a running server and a browser binary; they are integration-tier, slower
  than unit tests, and gate the observable scenarios rather than internals.
- Architecture conformance becomes a committed artifact: boundary changes must edit
  `architecture.spec.toml` deliberately, and `archspec verify --strict` fails on drift.
- Until the Go packages exist, the architecture test is red by design; the first module
  commits flip it toward green.

## Alternatives

- **Rust / other languages** — rejected: user constraint is Go only.
- **SPA JS framework (React/Vue/Svelte)** — rejected: htmx was chosen; a framework adds
  build ceremony, a second toolchain, and a client state layer the app has no use for.
- **Architecture unit / dependency-cruiser-style tools** — rejected: archspec was chosen;
  it supports the go driver, ships as a single binary, and keeps the spec and the check in
  one committed file.
