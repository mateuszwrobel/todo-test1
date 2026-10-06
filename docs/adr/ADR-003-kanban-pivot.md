# ADR-003: Kanban pivot — single-board application replaces the todo model

## Status

Accepted — 2026-10-06

## Context

The personal todo application was implemented through the wave ledger
(`workplans/dependencies.md`): a flat todo list with a done flag, four CRUD operations,
server-rendered list UI. The user's actual working need is board-style prioritization —
seeing work positioned by where it stands and moving it by dragging — and the user wants a
kanban. The todo model (list + done flag) does not express column position or
move-by-drag; extending it would twist a model the pivot intends to delete.

## Decision

- **Pivot the application to a single-board kanban.** One board, three fixed columns:
  To Do, In Progress, Done. No board management, no column configuration.
- **Cards carry persistent per-column positions.** Position within a column is stored
  state, not render order; reordering within a column is a first-class operation.
- **Done is membership of the Done column — no done flag.** A card is done exactly when
  it lives in the Done column; there is no separate boolean anywhere in the model.
- **Drag-and-drop is the sole movement mechanism.** Cards change column or position only
  by being dragged; no menus, no buttons, no editing position fields.
- **One-time migration of existing todos on first start.** Existing todo rows become
  cards: not-done todos land in To Do, done todos land in Done, in stable order. The
  migration runs once and never again.
- **The todo workplan family is superseded** — parent workplan, the four sub-workplans,
  the user journeys, and the scenario dependency ledger all point at the kanban workplan.
  They stay in the repo as history.
- **Stack and storage are unchanged.** ADR-001 (Go + htmx + Playwright + archspec) and
  ADR-002 (embedded SQLite) keep their full force; only the domain model and its
  contracts change.

## Consequences

- The `todos` store module's model is deleted: the todo table/struct and its CRUD
  operations are replaced by a card/column model with positions.
- The `ui` list rendering is replaced by board rendering with drag interactions; the page
  renders three columns and the drag affordances, not a flat list.
- The `api` endpoints are replaced: the todo CRUD contract gives way to card and move
  endpoints that the drag interactions call.
- The scenario cards and the dependency ledger are rewritten for kanban behavior — the
  todo wave ledger stops where it stands and kanban waves are authored fresh.
- Migration code lives in the composition root: one owner runs the one-time todo→card
  translation at startup (one-owner principle), and nothing else references the old model
  after it completes.

## Alternatives

- **Keep the todo model, add a priority field** — rejected: column-position ordering and
  done-as-position are a different model; a priority integer is not a per-column position
  and a done flag is not column membership. Twisting the todo model to fake a board
  violates delete-don't-twist.
- **Multi-board kanban** — rejected: out of scope for a single-user personal app; one
  board is what the need describes.
- **Keep the done flag alongside the columns** — rejected: two sources of truth for the
  same fact; the flag and the column would be able to disagree.
