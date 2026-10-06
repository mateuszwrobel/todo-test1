```json
{
  "status": "in-progress",
  "date": "2026-10-06",
  "cards": ["ui/11"],
  "workplan": "workplans/workplan_ui_board.md",
  "wave": "KW5"
}
```

# KW5 ui/11 — one stated-failure surface across edit, move, delete

Base: main @ 715d061 (kw5 ui drag cards landed).

## Scope

Card ui/11: the stale-operation failure surface must serve edit, drag
(move), and delete identically — stated reason + board truth re-rendered
without faking, and reload resolves. The three verbs already ride the same
fragment (banner-over-truth at the mirrored 404), but the mechanism exists
in three copies with forward-referencing seams. Consolidating it into one
named server-side owner and pinning 3-verb parity; per-operation VALIDATION
refusals (422 at the card) stay a distinct class — untouched.
