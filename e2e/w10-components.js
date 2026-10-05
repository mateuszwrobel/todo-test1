// W10 e2e — the design-system layer in a real browser: the /__components
// gallery carries a components block (above the state sections) with one
// visible example per named primitive at its exact #c-* id, the token
// chips in #c-tokens match the --color-* inventory parsed from the served
// tokens.css, and the token layer is provably live (var()-resolved
// computed colors, hex-free style.css). The real page still renders.
//
// Flow: build binary -> fresh temp db -> start server -> drive the gallery
// and the page -> stop. CommonJS so `require('playwright')` resolves via
// NODE_PATH.
const assert = require('assert');
const { chromium } = require('playwright');
const { spawn, spawnSync } = require('child_process');
const net = require('net');
const fs = require('fs');
const os = require('os');
const path = require('path');
const http = require('http');

const repoRoot = path.join(__dirname, '..');
const binDir = path.join(__dirname, 'bin');
const serverBin = process.env.BIN || path.join(binDir, 'todo');

const COMPONENT_IDS = [
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

function run(cmd, args) {
  const res = spawnSync(cmd, args, { cwd: repoRoot, encoding: 'utf8' });
  assert.strictEqual(res.status, 0, `${cmd} ${args.join(' ')} failed: ${res.stderr}`);
}

function freePort() {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.on('error', reject);
    srv.listen(0, '127.0.0.1', () => {
      const { port } = srv.address();
      srv.close(() => resolve(port));
    });
  });
}

function waitForHTTP(url, deadlineMs = 10000) {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    (function attempt() {
      http
        .get(url, (res) => {
          res.resume();
          resolve();
        })
        .on('error', () => {
          if (Date.now() - start > deadlineMs) reject(new Error(`timeout waiting for ${url}`));
          else setTimeout(attempt, 50);
        });
    })();
  });
}

function startServer(port, dbPath) {
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--db', dbPath], {
    stdio: ['ignore', 'ignore', 'pipe'],
  });
  let stderr = '';
  proc.stderr.on('data', (d) => (stderr += d));
  proc._stderr = () => stderr;
  return proc;
}

async function stopServer(proc) {
  proc.kill('SIGTERM');
  await new Promise((resolve) => proc.on('exit', resolve));
}

function tmpDb(name) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'todo-e2e-'));
  return path.join(dir, name);
}

// colorTokenNames parses the --color-* inventory out of tokens.css source
// (the same parse the Go drift gate runs, at browser level).
function colorTokenNames(css) {
  const names = [];
  for (const m of css.matchAll(/--color-[a-z0-9-]+(?=\s*:)/g)) names.push(m[0]);
  return names;
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);

  const browser = await chromium.launch();
  const page = await browser.newPage();

  try {
    const db = tmpDb('components.db');
    const port = await freePort();
    const srv = startServer(port, db);
    await waitForHTTP(`http://127.0.0.1:${port}/todos`);
    const base = `http://127.0.0.1:${port}`;

    // --- Scenario: every component example is present and visible, the
    // block sits above the state sections.
    const resp = await page.goto(`${base}/__components`);
    assert.strictEqual(resp.status(), 200, 'GET /__components did not answer 200');
    const block = page.locator('#components');
    assert.strictEqual(await block.count(), 1, 'components block missing');
    assert.ok(await block.isVisible(), 'components block not visible');
    const blockBox = await block.boundingBox();
    const statesBox = await page.locator('#state-list-populated').boundingBox();
    assert.ok(blockBox && statesBox && blockBox.y < statesBox.y,
      'components block does not sit above the state sections');
    for (const id of COMPONENT_IDS) {
      const loc = page.locator(`#${id}`);
      assert.strictEqual(await loc.count(), 1, `example #${id} missing`);
      await loc.scrollIntoViewIfNeeded();
      assert.ok(await loc.isVisible(), `example #${id} not visible`);
    }
    console.log('scenario 1 (components block above states, all 17 #c-* examples visible): OK');

    // --- Scenario: the frozen component states render.
    assert.ok(await page.locator('#c-btn-disabled').isDisabled(),
      '#c-btn-disabled is not disabled');
    assert.ok(await page.locator('#c-checkbox-disabled').isDisabled(),
      '#c-checkbox-disabled is not disabled');
    assert.ok(await page.locator('#c-checkbox-checked').isChecked(),
      '#c-checkbox-checked is not checked');
    assert.ok(await page.locator('#c-row-done').getAttribute('class').then((c) => c.includes('row--done')),
      '#c-row-done lacks its row--done state class');
    console.log('scenario 2 (frozen component states render): OK');

    // --- Scenario: drift gate — chips vs tokens.css, at browser level.
    const css = await page.request.get(`${base}/static/tokens.css`);
    assert.strictEqual(css.status(), 200, 'GET /static/tokens.css did not answer 200');
    const declared = colorTokenNames(await css.text());
    assert.ok(declared.length > 0, 'tokens.css declares no --color-* tokens');
    const chipLabels = await page.$$eval('#c-tokens code', (els) => els.map((e) => e.textContent));
    assert.deepStrictEqual(
      chipLabels.slice().sort(), declared.slice().sort(),
      'gallery token chips do not match the --color-* inventory in tokens.css'
    );
    console.log(`scenario 3 (${declared.length} color tokens: chips match tokens.css): OK`);

    // --- Scenario: the token layer is live — var()-resolved computed
    // colors, and the consumer sheet provably reads the token sheet.
    const btnBg = await page.evaluate(
      'getComputedStyle(document.querySelector("#c-btn-primary")).backgroundColor'
    );
    assert.strictEqual(btnBg, 'rgb(37, 99, 235)', `primary button not token-styled (bg = ${btnBg})`);
    const styleCss = await page.request.get(`${base}/static/style.css`);
    assert.match(await styleCss.text(), /var\(--color-/, 'style.css does not consume the tokens');
    console.log('scenario 4 (token layer live: computed colors resolve from var(), style.css consumes tokens): OK');

    // --- Scenario: the real page still renders through the token layer.
    await page.goto(`${base}/`);
    assert.strictEqual(resp.status(), 200);
    assert.strictEqual(await page.locator('h1').textContent(), 'My Todos',
      'page heading missing');
    const bodyBg = await page.evaluate('getComputedStyle(document.body).backgroundColor');
    assert.strictEqual(bodyBg, 'rgb(243, 244, 247)', 'page body not token-styled');
    await stopServer(srv);
  } finally {
    await browser.close();
  }

  console.log('W10 e2e: all scenarios OK');
  process.exit(0);
}

main().catch((err) => {
  console.error('W10 e2e FAILED:', err);
  process.exit(1);
});
