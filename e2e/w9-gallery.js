// W9 e2e — styling + the /__components story gallery against the composed
// server in a real browser. Asserts: the gallery answers 200 with all seven
// state containers present AND visible, each labelled by its caption, its
// fixture content renders (the real templates, no live calls), the page
// styles are actually applied (style.css served + in effect), and the real
// page still works end-to-end — one create round-trip with reload
// persistence — proving the styling layer changed no behavior.
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

const STATE_IDS = [
  'state-list-populated',
  'state-empty',
  'state-create-error',
  'state-edit-band',
  'state-load-failure',
  'state-missing-todo',
  'state-in-flight-disabled',
];
const CAPTIONS = [
  'state: list populated',
  'state: empty',
  'state: create error',
  'state: edit band',
  'state: load failure',
  'state: missing todo',
  'state: in-flight disabled',
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

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);

  const browser = await chromium.launch();
  const page = await browser.newPage();

  try {
    const db = tmpDb('gallery.db');
    const port = await freePort();
    const srv = startServer(port, db);
    await waitForHTTP(`http://127.0.0.1:${port}/todos`);
    const base = `http://127.0.0.1:${port}`;

    // --- Scenario: the gallery answers 200 with every state container.
    const resp = await page.goto(`${base}/__components`);
    assert.strictEqual(resp.status(), 200, 'GET /__components did not answer 200');
    assert.strictEqual(
      await page.locator('h1').textContent(),
      'Todo UI — components',
      'gallery page heading wrong'
    );
    for (const id of STATE_IDS) {
      const box = page.locator(`#${id}`);
      assert.strictEqual(await box.count(), 1, `gallery is missing #${id}`);
      await box.scrollIntoViewIfNeeded();
      assert.ok(await box.isVisible(), `gallery container #${id} is not visible`);
    }
    for (const caption of CAPTIONS) {
      assert.ok(
        await page.locator('.state-caption', { hasText: caption }).first().isVisible(),
        `gallery caption "${caption}" missing or invisible`
      );
    }
    console.log('scenario 1 (gallery: 200 + all seven state containers visible): OK');

    // --- Scenario: fixtures render through the real templates.
    await assert
      .strictEqual(await page.locator('#state-list-populated #todo-list li').count(), 4,
        'populated fixture does not show the four mixed-state rows');
    assert.strictEqual(
      await page.locator('#state-list-populated #todo-list li[data-state="done"]').count(), 2,
      'populated fixture rows are not a done/not-done mix');
    assert.ok(await page.locator('#state-empty #empty-state').isVisible(),
      'empty fixture missing');
    assert.strictEqual(
      (await page.locator('#state-create-error #create-error').textContent()).trim(),
      'title is required', 'create-error fixture states the wrong reason');
    assert.strictEqual(
      await page.locator('#state-edit-band li.editing .edit-form input[name=title]').inputValue(),
      'Walk the dog in the park', 'edit-band fixture is not prefilled');
    assert.ok(await page.locator('#state-load-failure #load-error #retry').isVisible(),
      'load-failure fixture missing its retry control');
    assert.strictEqual(
      (await page.locator('#state-missing-todo #missing-todo-banner').textContent()).trim(),
      'no such todo', 'missing-todo fixture states the wrong reason');
    assert.ok(
      await page.locator('#state-in-flight-disabled #create-form-in-flight button[type=submit]').isDisabled(),
      'in-flight fixture: Add is not disabled');
    for (const sel of [
      '#state-in-flight-disabled #todo-list input[type=checkbox]',
      '#state-in-flight-disabled #todo-list button.delete',
    ]) {
      const n = await page.locator(sel).count();
      assert.ok(n > 0, `in-flight fixture: no controls for ${sel}`);
      for (let i = 0; i < n; i++) {
        assert.ok(await page.locator(sel).nth(i).isDisabled(),
          `in-flight fixture: ${sel} row ${i} is not disabled`);
      }
    }
    assert.ok(
      await page.locator('#state-in-flight-disabled li.editing .save').isDisabled(),
      'in-flight fixture: Save is not disabled');
    console.log('scenario 2 (gallery fixtures render the real fragments): OK');

    // --- Scenario: the stylesheet is served and applied.
    const css = await page.request.get(`${base}/static/style.css`);
    assert.strictEqual(css.status(), 200, 'GET /static/style.css did not answer 200');
    const styled = await page.evaluate(
      'getComputedStyle(document.body).backgroundColor'
    );
    assert.strictEqual(styled, 'rgb(243, 244, 247)',
      `body is not styled by style.css (bg = ${styled})`);
    console.log('scenario 3 (style.css served and in effect): OK');

    // --- Scenario: the real page still works — one create round-trip.
    await page.goto(`${base}/`);
    assert.strictEqual(await page.locator('h1').textContent(), 'My Todos',
      'page heading missing');
    await page.fill('#create-form input[name=title]', 'style the page');
    await page.click('#create-form button[type=submit]');
    await page.locator('#todo-list li').first().waitFor({ state: 'visible' });
    assert.strictEqual(
      await page.locator('#todo-list .title').first().textContent(), 'style the page',
      'create round-trip did not land the row');
    await page.reload();
    assert.strictEqual(
      await page.locator('#todo-list .title').first().textContent(), 'style the page',
      'create did not persist across reload');
    console.log('scenario 4 (real page create round-trip + reload): OK');

    await stopServer(srv);
  } finally {
    await browser.close();
  }

  console.log('W9 e2e: all scenarios OK');
  process.exit(0);
}

main().catch((err) => {
  console.error('W9 e2e FAILED:', err);
  process.exit(1);
});
