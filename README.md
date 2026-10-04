# agentic-coding-template

Generic, reusable agentic-engineering setup for a code authoring agent: skills, agent
definitions, instructions (AGENTS.md), and CI + git watch/restart scripts — scrubbed of
all internal/infrastructure references. Copy it into your own projects and adapt.

## Contents map

| Path | What it is |
|------|------------|
| `AGENTS.md` | Instructions template — fill the `<PLACEHOLDER>` tokens, then adapt + trim |
| `.agents/` | **Canonical, tool-agnostic content store** — single source of truth for all agent content |
| `.agents/agents/` | Markdown agent definitions: `orchestrator` (primary — plans + delegates), `coder` (implementation), `code-verifier` (deep claim-verification), `reviewer` (peer code review), `git-ops` (commit/push only), `ci-watcher` (watch GitHub Actions runs, report-only) |
| `.agents/skills/` | Authoring-skills kit, one directory per skill (`<id>/SKILL.md`): TDD workplan, long-drill discipline, modular design/planner/reviewer |
| `.agents/skills/hotpath-mcp/SKILL.md` | **Tool-specific** skill example (real public Rust profiling library with an MCP server) — shows the shape for language/tool-specific additions; delete or adapt if it does not apply |
| `.agents/commands/` | Prompt templates (e.g. `adr.md` — create an Architecture Decision Record) |
| `.opencode/` | **Opencode loader** — committed symlinks into `.agents/`, zero duplication |
| `.opencode/agents/` | Symlinks → `.agents/agents/*.md` (opencode reads agents from here) |
| `.opencode/commands/` | Symlinks → `.agents/commands/*.md` (opencode reads commands from here) |
| `scripts/` | `watch-ci.sh` (watch a GitHub Actions run to terminal state) and `watch-git.sh` (observe a detached commit/push drive, restart when stuck/failed) |
| `opencode.jsonc` | OpenCode project config — sets the default session agent to the shipped `orchestrator` primary |

## How to use

1. Copy this tree into a new project.
2. Fill the `<PLACEHOLDER>` tokens in `AGENTS.md`, then adapt + trim it for your project.
3. Start opencode — the session defaults to the `orchestrator` primary agent (see `opencode.jsonc`).
4. Edit content under `.agents/` — it is the canonical store. The `.opencode/` symlinks
   make opencode pick up agents and commands with zero copy: change
   `.agents/agents/*.md` or `.agents/commands/*.md` and the next opencode session sees it.
   Skills need no symlink — opencode auto-discovers `.agents/skills` (each skill is a
   directory containing a `SKILL.md`).
5. If you dislike symlinks, replace the `.opencode/` entries with real copies of the
   `.agents/` files — that trades single-source-of-truth for tool-portability.
6. `chmod +x scripts/*.sh`.
7. Adapt/trim as needed — this is a starting kit, not a contract.

> **Models:** agents ship WITHOUT a `model:` line, so they inherit the session model
> (opencode uses the session model when `model:` is absent). Consumers add
> `model: provider/model` per agent when they want a fixed model.

## Flagship patterns

- **Watch CI and react** — `scripts/watch-ci.sh` plus the `ci-watcher` agent: check a
  workflow run to terminal state, classify phases (`queued` → `in_progress` →
  `success`/`failure`/`cancelled`/...), report verdicts with verbatim evidence. Deadlines
  on every wait, machine-readable `STATE|RUN|...` / `FINAL|VERDICT|...` output.
- **Observe commit/push, restart when stuck** — `scripts/watch-git.sh`: monitor a
  detached drive script using the sentinel protocol (`<loop>-<N>.log`, `<loop>-<N>.done`,
  `<loop>.final` = landed HEAD hash or `FAILED`), and re-launch it detached when stuck or
  failed — bounded restarts, hard deadlines, `/var/tmp` scratch guidance.

## Notes

- This is the generic, infra-free extraction. A generic **orchestrator** (primary) + subagent pair
  now ships out of the box; consumers may still add further language/tool-specific agents by
  example of the included tool-specific skill (`.agents/skills/hotpath-mcp/SKILL.md`).
- All watchers follow long-drill discipline (MILE UTC heartbeats, deadlines, machine-readable
  verdicts) — see `.agents/skills/long-drill/SKILL.md` for the full doctrine.

## Todo application (this repo's product code)

Modules: `todos` (SQLite-backed store), `api` (JSON contract), `ui` (htmx page),
`cmd/todo` (composition root). Run: `go run ./cmd/todo --addr 127.0.0.1:8080 --db todos.db`.

W5 — delete: rows carry a Delete control; the click issues an htmx DELETE to
`/ui/todos/{id}`, which performs the api contract's `DELETE /todos/{id}` over HTTP
(204; 404 `{"error":"no such todo"}` when gone) and swaps in the resulting list
state — the row drops without a reload, deleting the last row lands the empty
state, and deleted ids are never reused (schema autoincrement). Browser
acceptance: `make e2e-w5` (also `make e2e-w1` for the foundation).

W8 — in-flight control serialization: while the operation triggered by a
control is in flight, repeat activation of that control causes no request —
all four controls (create Add, row checkbox, edit Save, row Delete) carry
htmx `hx-disabled-elt` naming exactly the activated control, and each
re-enables when the response arrives, success or failure. Blocking is
per-control: controls on other rows stay usable. Browser acceptance:
`make e2e-w8` — a Playwright route delay makes the in-flight window
observable, a counting proxy proves exactly one request (and one server-side
effect via `GET /todos`) per double activation, and a forced 500 proves the
control never stays dead.
