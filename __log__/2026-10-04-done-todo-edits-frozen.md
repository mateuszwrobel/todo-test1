```json
{"task":"done todos reject text edits","status":"done","date":"2026-10-04"}
```

- `workplans/workplan_todo_application.md`: replaced scenario "Edit todo text keeps done state" with two — "Edit todo text leaves done state alone" (not-done row, edit leaves state untouched) and "Done todo rejects text edits" (stale page attempts an edit on a done todo; text unchanged, refusal stated). Added Decisions entry freezing done text (reopen first; page-only enforcement rejected — server is the truth a stale page cannot see), a PATCH 422 `cannot edit a done todo` error line, a Data Flow clause (title for a done todo refused before any column update), and an edit-affordance-on-not-done-rows clause in the Page interaction behavior bullet.
- `workplans/user-journeys.md`: J1 hook rewritten (toggle/delete regardless of state, edit only on not-done rows); J4 rewritten honestly (no done-edit happy path; done row → no edit affordance, reopen via J3; stale page → 422 with the row keeping its text); cross-cutting table gains "no edit affordance" on the done row plus the new error-surface row; traceability now lists nine scenarios.
- Why: done marks settled state — text edits and done toggles stay independent observable operations, and a stale page gets a stated refusal instead of a silent rewrite.
- Flagged for regen: mockups 01 / 03 / 04 / 06 under `designs/mockups/` show an Edit affordance on done rows — pending, not regenerated here.
