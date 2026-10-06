```json
{
  "status": "done",
  "task": "kw7 visual lane — restore visual-regression lane for the kanban board: config + server harness + spec, committed baselines, two back-to-back green runs",
  "card": "workplans/scenarios-kanban/ui/13-every-state-renders-in-the-gallery.md (baselines half)",
  "wave": "kw7",
  "base": "5460f95"
}
```

What changed and why:

- **The lane retired at KW1 (84d344a) returns kanban-shaped.** The three
  retired files came back under their old names (visual.spec.js,
  playwright.visual.config.js, visual-server.js) with the W11 determinism
  contract kept verbatim — fixed 1280x900 viewport at scale 1, animations
  disabled, caret hidden, reduced motion, light scheme, retries 0, one
  worker, pixelmatch at maxDiffPixelRatio 0.001, flat committed
  `__snapshots__/` via snapshotPathTemplate. Everything the old config pinned
  exists to make pixels reproducible; nothing was loosened relative to it.
- **Two servers instead of one** (visual-server.js): the board's truth is now
  the seeded db, so the lane needs two deterministic data shapes side by
  side — one server over a file seeded through the e2e seed tool (fixed
  titles: plain + unicode + one 485-char wrap card, fixed placements so all
  three columns are populated and the Done card carries the unicode title),
  one over a never-created board file — the kw1 "Start without todo data"
  shape for the stated-empty page. Both on free ports, `--todo-db` pointed
  at a never-created path per the kw1..kw6 no-leak convention. The gallery
  server half is data-independent by construction (compile-time fixtures),
  so the seeded file cannot leak into a gallery pixel.
- **Spec captures five live surfaces + every gallery anchor.** Live: full
  board page over the seeded db (three populated columns, k1 layout), the
  Done card element (green-check done treatment on real seeded data), the
  create rejection on the live page, and the full stated-empty page. Gallery:
  all 15 `#c-*` component examples and all 9 `#state-*` sections — the
  every-state gallery ui/13 just landed, anchor-by-anchor so a diff names
  its state.
- **The live create rejection is deterministic without any hover.** The
  state is not GET-reachable (POST /ui/cards answers it with an OOB create-
  area swap), so the capture drives the kw2 lane's proven whitespace-fill +
  submit gesture — a form gesture the functional lanes execute green every
  run — then parks the cursor at (0,0) before the screenshot so the `.btn
  :hover` paint can never enter the frame. The result is the as-rendered
  k2 surface (red-outlined input, stated reason).
- **Hover-gated controls are captured at rest, as rendered.** Card
  Edit/Delete are opacity-gated (always in the DOM, paint gated — the KW7
  styling pass's deviation), so with no synthesized hover every baseline
  shows the laid-out-but-unpainted controls; nothing forces them visible,
  which would have frozen a state no rest render ever shows.
- **Two back-to-back `make e2e-visual` runs green** against the committed
  baselines (28/28 each, ~3.5s), and `go test ./...` stays green — the
  product was not touched.
- **Wiring:** Makefile gains `e2e-visual` (old name, old `PW_ARGS=
  --update-snapshots` regeneration line, binaries built the same way the
  functional target builds them); the default `e2e` target is untouched —
  the six functional lanes. `.gitignore` regains the retired lane's
  `e2e/visual/.results/` entry; baselines themselves are committed data.
