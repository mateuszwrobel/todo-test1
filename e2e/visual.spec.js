// KW7 visual regression — committed baselines for the kanban surfaces, via
// Playwright's own toHaveScreenshot (the pattern the W11 lane set before the
// pivot retired it; card ui/13, baselines half). Run settings (viewport,
// animations, caret, reduced motion) are pinned in
// playwright.visual.config.js; baselines are the committed PNGs under
// e2e/visual/__snapshots__/.
//
// Captured surfaces:
// - the main board page over the seeded db — three populated columns, the
//   mockup layout, hover-gated card controls at rest: they are opacity-gated
//   (always in the DOM, paint gated), so the page renders with the buttons
//   invisible-but-laid-out and the baseline captures that as-rendered state
//   — no hover is ever synthesized;
// - the stated-empty board page over a never-seeded db — three columns each
//   stating emptiness;
// - the done treatment on the live surface: the seeded Done card element
//   (green check + muted title derived from column membership);
// - the create rejection on the live surface — deterministic by construction:
//   the whitespace create gesture (fill + submit, the kw2 lane's proven
//   gesture — no hovers), then #create-area screenshotted with the cursor
//   parked off-element so no :hover paint enters the frame;
// - every anchor on the gallery page (/__components): each #c-* component
//   example and each #state-* section, rendered from compile-time fixtures
//   — fixed titles, fixed ids, no store, no clock, no randomness, no client
//   script — so the gallery halves of the run cannot drift between runs.
//   KW10 (card ui/19) adds two: #state-assigned (assigned chip, unassigned
//   card, Done chip-only, band select all in one fixture board) and
//   #state-filtered (a filtered column stating its filter-named empty).
//   The live full-page anchors (page-board-populated, page-board-empty)
//   also carry the new filter dropdown chrome; the seeded fixtures stay
//   unassigned, so no chip enters those live shots.
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
  'c-error-text',
  'c-panel',
  'c-heading',
  'c-card',
  'c-card-done',
  'c-source-slot',
  'c-drop-indicator',
  'c-drag-chip',
  'c-banner',
  'c-tokens',
];

// Every observable-state section on the gallery, rendered from the same
// compile-time fixtures (fixed card ids per section keep the DOM unique).
const STATE_ANCHORS = [
  'state-board',
  'state-board-empty',
  'state-load-failure',
  'state-create',
  'state-create-error',
  'state-edit-band',
  'state-edit-error',
  'state-stale',
  'state-drag',
  'state-assigned',
  'state-filtered',
];

// --- live surfaces over the seeded, three-columns-populated board ---

test('main board page (three populated columns) matches its baseline', async ({ page }) => {
  const resp = await page.goto(`${process.env.VISUAL_BASE_URL}/`);
  expect(resp.status()).toBe(200);
  // The whole page: header line (heading + create band) and the three
  // populated columns beneath — the mockup k1 layout on real seeded data.
  await expect(page).toHaveScreenshot('page-board-populated.png', { fullPage: true });
});

test('done treatment on the live board matches its baseline', async ({ page }) => {
  const resp = await page.goto(`${process.env.VISUAL_BASE_URL}/`);
  expect(resp.status()).toBe(200);
  // Exactly one seeded Done card (strict locator), captured as the element:
  // the green-check done treatment, unicode title, controls at rest.
  await expect(page.locator('.card--done')).toHaveScreenshot('live-card-done.png');
});

test('create rejection on the live page matches its baseline', async ({ page }) => {
  const resp = await page.goto(`${process.env.VISUAL_BASE_URL}/`);
  expect(resp.status()).toBe(200);
  // The kw2 lane's proven rejection gesture: whitespace text, submit. The
  // refusal swaps the create-area fragment in place (OOB) — the same markup
  // the gallery's #state-create-error anchors, on the live page frame.
  await page.fill('#create-form input.input[name=title]', '   ');
  await page.click('#create-form button[type=submit]');
  await expect(page.locator('#create-area #create-error')).toBeVisible();
  // Park the cursor where no :hover paint can apply, then capture the band:
  // the outlined input + stated reason (mockup k2), hover-free by position.
  await page.mouse.move(0, 0);
  await expect(page.locator('#create-area')).toHaveScreenshot('live-create-error.png');
});

// --- live surface over a never-seeded db: the stated-empty board ---

test('stated-empty board page matches its baseline', async ({ page }) => {
  const resp = await page.goto(`${process.env.VISUAL_EMPTY_URL}/`);
  expect(resp.status()).toBe(200);
  await expect(page).toHaveScreenshot('page-board-empty.png', { fullPage: true });
});

// --- the gallery: every component example and every observable state ---

for (const id of [...COMPONENT_ANCHORS, ...STATE_ANCHORS]) {
  test(`anchor #${id} matches its committed baseline`, async ({ page }) => {
    const resp = await page.goto(`${process.env.VISUAL_BASE_URL}/__components`);
    expect(resp.status()).toBe(200);
    await expect(page.locator(`#${id}`)).toHaveScreenshot(`${id}.png`, {
      maxDiffPixelRatio: 0.001,
    });
  });
}
