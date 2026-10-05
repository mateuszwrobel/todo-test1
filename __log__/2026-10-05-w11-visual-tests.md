```json
{
  "status": "done",
  "date": "2026-10-05",
  "wave": "W11"
}
```

Visual regression for the design-system layer: Playwright's own screenshot
testing (`@playwright/test` `toHaveScreenshot`) over every anchor the gallery
exposes — the seventeen `#c-*` component examples and the seven `#state-*`
sections of `/__components`. Baselines are committed PNGs under
`e2e/visual/__snapshots__/`; the lane is fully deterministic (fixed viewport,
animations frozen, caret hidden, reduced motion, static focus state), so a
second run without `--update-snapshots` is the real gate.

What changed and why:

- `e2e/playwright.visual.config.js` — the @playwright/test config. One knob
  per drift source: fixed 1280x900 viewport and deviceScaleFactor 1 (no DPI
  drift), `animations: 'disabled'` + `caret: 'hide'` + `reducedMotion:
  'reduce'` + light color scheme (no transitions, caret blink, or theme
  drift), retries 0 with a single worker (a red run is a real diff), and
  `snapshotPathTemplate` flattening baselines into one committed directory.
  Pixelmatch mode stays local (`pk`) — no CI-mode external comparison.
- `e2e/visual-server.js` — globalSetup following the w1..w10 lane pattern:
  builds the binaries itself, seeds a fresh temp data file through the
  existing seed tool (fixed titles incl. unicode and a 485-char long title,
  fixed done indexes → deterministic autoincrement ids), serves on a free
  port. The gallery renders from compile-time fixtures so the seed cannot
  reach any pixel, but the app starts exactly as in production: with a real
  deterministic db behind it.
- `e2e/visual.spec.js` — 24 tests, one element screenshot per anchor,
  `maxDiffPixelRatio 0.001`. No hover, no typing: the focus example is the
  static `.is-focus` markup, so nothing client-side moves between runs.
- `Makefile` — `e2e-visual` (builds binaries, runs the global
  `@playwright/test` CLI through the `NODE_PATH` pattern); `PW_ARGS` passes
  flags through so baseline updates are a deliberate
  `make e2e-visual PW_ARGS=--update-snapshots`.
- `.gitignore` — lane artifacts (`e2e/visual/.results/`) stay untracked;
  baselines are tracked.

Verified: `make e2e-visual` green twice back-to-back with zero snapshot
updates (24 tests), `make e2e-w1..w10` green, `go test -count=1 ./...`
green, `archspec verify --strict` green. No product code changed — the wave
is test surface + docs only; the gallery's no-clock/no-randomness property
(`ui/stories.go` fixtures) is what makes the baselines reproducible at all.
