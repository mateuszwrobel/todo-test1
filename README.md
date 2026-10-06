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

Modules: `board` (SQLite-backed board store), `api` (JSON contract),
`ui` (board page), `cmd/todo` (composition root). Run:
`go run ./cmd/todo --addr 127.0.0.1:8080 --board-db kanban.db`.

KW1 — board page: `GET /` renders the board, read over HTTP from the contract's
`GET /board` — the three fixed columns To Do / In Progress / Done with their
cards top-to-bottom in server order, a stated empty treatment on each empty
column, and a stated load-failure state (retry re-reads) instead of columns
standing in for a truth the server could not give. The done treatment is
carried purely by membership of the Done column — the contract carries no done
field — so no row checkbox or toggle exists anywhere on the page. Browser
acceptance: `make e2e` (the retired per-wave targets `e2e-w1..w11` are now
this single suite).

Transitional state: the todo surface is fully retired — every `*_ /todos*`
endpoint is gone (last one deleted at KW4) and the `todos` store package no
longer exists; the page's card controls are inert placeholders until their
operations wire on.
