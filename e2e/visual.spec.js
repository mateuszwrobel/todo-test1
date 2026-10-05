// W11 visual regression — element screenshots of every gallery anchor, via
// Playwright's own toHaveScreenshot. The gallery (/__components) renders
// every anchor from compile-time fixtures (ui/stories.go): fixed titles,
// fixed ids, no store, no clock, no randomness, no client script — and the
// focus state is static markup (.is-focus), so nothing is hovered or typed
// into during a run. The run settings (viewport, animations off, caret
// hidden, reduced motion) are pinned in playwright.visual.config.js;
// baselines are the committed PNGs under e2e/visual/__snapshots__/.
//
// Regenerate deliberately:
//   make e2e-visual PW_ARGS=--update-snapshots   (then review + commit the PNGs)
const { test, expect } = require('@playwright/test');

// Every named component example on the gallery's components block.
const COMPONENT_ANCHORS = [
  'c-btn-primary',
  'c-btn-secondary',
  'c-btn-disabled',
  'c-input-default',
  'c-input-focus',
  'c-checkbox-unchecked',
  'c-checkbox-checked',
  'c-checkbox-disabled',
  'c-row-not-done',
  'c-row-done',
  'c-error-text',
  'c-banner',
  'c-hint',
  'c-empty-state',
  'c-panel',
  'c-heading',
  'c-tokens',
];

// Every observable-state section on the gallery, rendered from the same
// compile-time fixtures (fixed row ids per section keep the DOM unique).
const STATE_ANCHORS = [
  'state-list-populated',
  'state-empty',
  'state-create-error',
  'state-edit-band',
  'state-load-failure',
  'state-missing-todo',
  'state-in-flight-disabled',
];

for (const id of [...COMPONENT_ANCHORS, ...STATE_ANCHORS]) {
  test(`anchor #${id} matches its committed baseline`, async ({ page }) => {
    const resp = await page.goto(`${process.env.VISUAL_BASE_URL}/__components`);
    expect(resp.status()).toBe(200);
    await expect(page.locator(`#${id}`)).toHaveScreenshot(`${id}.png`, {
      maxDiffPixelRatio: 0.001,
    });
  });
}
