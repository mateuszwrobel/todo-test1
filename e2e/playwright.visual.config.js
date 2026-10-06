// KW7 visual regression — the lane retired with KW1's todo-app swap, rebuilt
// kanban-shaped, via Playwright's own toHaveScreenshot (the W11 pattern).
//
// Everything here exists to make pixels reproducible:
// - fixed 1280x900 viewport, deviceScaleFactor 1 (scale: 'css') — no DPI drift;
// - animations disabled, caret hidden, reduced motion, light color scheme —
//   no transitions, no blinking caret, no prefers-dark drift (style.css
//   carries no transitions by its own contract, so 'disabled' is belt-only);
// - retries 0 and a single worker — a red run is a real diff, never noise;
// - baselines live in e2e/visual/__snapshots__/ via snapshotPathTemplate,
//   committed to git; mode stays the default pixelmatch ('pk').
//
// The servers themselves are launched by visual-server.js (globalSetup): it
// builds the binaries, seeds a FRESH deterministic kanban board file through
// the e2e seed tool (cards in all three columns), starts one server on that
// db and one on a never-seeded empty db, both on FREE ports — the same
// self-contained pattern as the kw1..kw6 lane scripts, no webServer
// fixed-port assumption.
const path = require('path');
const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: __dirname,
  testMatch: 'visual.spec.js',
  globalSetup: path.join(__dirname, 'visual-server.js'),
  outputDir: path.join(__dirname, 'visual', '.results'),
  snapshotDir: path.join(__dirname, 'visual'),
  // One flat directory of committed baselines: state-board.png,
  // page-board-populated.png etc.
  snapshotPathTemplate: '{snapshotDir}/__snapshots__/{arg}{ext}',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'list',
  expect: {
    timeout: 15000,
    toHaveScreenshot: {
      animations: 'disabled',
      caret: 'hide',
      scale: 'css',
      maxDiffPixelRatio: 0.001,
    },
  },
  use: {
    viewport: { width: 1280, height: 900 },
    deviceScaleFactor: 1,
    reducedMotion: 'reduce',
    colorScheme: 'light',
  },
});
