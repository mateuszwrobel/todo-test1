```json
{
  "status": "in-progress",
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
