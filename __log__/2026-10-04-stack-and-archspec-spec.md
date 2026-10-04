```json
{
  "task": "stack decision + archspec architecture spec",
  "status": "done",
  "date": "2026-10-04"
}
```

Stack decision recorded, archspec installed as agent skill, first architecture test authored spec-first.

- `docs/adr/ADR-001-stack-go-htmx-playwright-archspec.md` records the user's stack decision: Go only, htmx for page interactivity (server-rendered hypermedia, single binary), Playwright for integration/e2e, archspec as the architecture-test gate (`archspec verify --strict` on the committed root `architecture.spec.toml`). Workplan Decisions bullet points at the ADR.
- `.agents/skills/archspec/SKILL.md` is the byte-verbatim stdout of `archspec skill` (v0.5.2), installed so agents follow the audit protocol.
- `architecture.spec.toml` declares the planned boundaries before any code exists: modules `todos` / `api` / `ui` / `server`, the composition root pinned to `cmd/todo` (package main; chosen over a root main so the root stays clean of wiring). Directions: `api → todos` in-process only; `ui` carries no in-process dependency (its coupling to `api` is runtime HTTP, invisible to the driver, so a go import of api or todos from ui is forbidden drift); `server` wires all three; `no_cycles` over the four. Matching uses module-path-agnostic unit globs (`*/todos` …) because go `matches.units` needs full import paths and the go module path isn't pinned yet.
- Intended red state: with no go sources yet, `archspec verify [--strict]` and `archspec scan` exit 1 with `error: no supported-language sources found under: .` — the gate stands by, executing but unmet, until the modules land.
- Spec semantics validated against a scratch go tree outside the repo: green when wired as planned (`ok: … 4 modules, 1 constraints checked`), red on drift (`forbidden edge: ui -> todos`), so the first module commits flip this file's state toward green by construction, not guesswork.
