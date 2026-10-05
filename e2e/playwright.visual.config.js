// W11 visual regression — config for Playwright's own screenshot testing
// (@playwright/test, the global install; the Makefile target runs it with
// NODE_PATH like the library-playwright lanes run their scripts).
//
// Everything here exists to make pixels reproducible:
// - fixed 1280x900 viewport, deviceScaleFactor 1 (scale: 'css') — no DPI drift;
// - animations disabled, caret hidden, reduced motion, light color scheme —
//   no transitions, no blinking caret, no prefers-dark drift;
// - retries 0 and a single worker — a red run is a real diff, never noise;
// - baselines live in e2e/visual/__snapshots__/ via snapshotPathTemplate,
//   committed to git; mode stays the default pixelmatch ('pk'), CI-style
//   external comparison is off.
//
// The server itself is launched by visual-server.js (globalSetup): it builds
// the binaries, seeds a fresh deterministic data file through the existing
// seed tool, and serves on a free port — the same self-contained pattern as
// the w1..w10 lane scripts, no webServer fixed-port assumption.
const path = require('path');
const { defineConfig } = require('@playwright/test');

module.exports = defineConfig({
  testDir: __dirname,
  testMatch: 'visual.spec.js',
  globalSetup: path.join(__dirname, 'visual-server.js'),
  outputDir: path.join(__dirname, 'visual', '.results'),
  snapshotDir: path.join(__dirname, 'visual'),
  // One flat directory of committed baselines: c-btn-primary.png etc.
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
