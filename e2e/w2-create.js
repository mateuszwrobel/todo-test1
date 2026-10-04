// W2 e2e — the create feature against the composed server in a real browser.
// Parent scenarios "Create todo" + "Reject empty todo text" re-executed:
// create via form appends the row with NO page reload (a window marker proves
// the JS context survived — navigation or a full reload would clear it), a
// blank submit states the required-text reason, an over-limit submit states
// the limit, and a reload shows the persistence.
//
// Flow: build binary -> fresh temp db -> start server -> drive the page ->
// stop. CommonJS so `require('playwright')` resolves via NODE_PATH.
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
const LIMIT = 500; // the stated title limit

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
  const failures = [];

  try {
    const db = tmpDb('create.db');
    const port = await freePort();
    const srv = startServer(port, db);
    await waitForHTTP(`http://127.0.0.1:${port}/todos`);
    const url = `http://127.0.0.1:${port}/`;
    await page.goto(url);

    // Marker: survives htmx swaps, cleared by any navigation/reload.
    await page.evaluate('window.__w2_no_nav = 1');
    const noNav = async () =>
      assert.strictEqual(await page.evaluate('window.__w2_no_nav || 0'), 1,
        'page navigated or reloaded during an htmx operation');

    const input = page.locator('#create-form input[name=title]');
    const addBtn = page.locator('#create-form button[type=submit]');
    const rows = page.locator('#todo-list li');

    // --- Scenario: create appends without reload (from the empty page up).
    await input.fill('write the spec');
    await addBtn.click();
    await rows.waitFor({ state: 'visible' });
    assert.strictEqual(await rows.count(), 1, 'first create did not append a row');
    await noNav();

    await input.fill('Buy milk');
    await addBtn.click();
    await page.waitForFunction(() => document.querySelectorAll('#todo-list li').length === 2);
    await noNav();
    assert.strictEqual(await page.locator('#todo-list .title').nth(1).textContent(), 'Buy milk',
      'new todo is not the last row');
    assert.strictEqual(await rows.nth(1).locator('input[type=checkbox]').isChecked(), false,
      'new row is not marked not-done');
    assert.strictEqual(await input.inputValue(), '', 'create input not ready for the next todo');
    await noNav();
    console.log('scenario 1 (create appends without reload): OK');

    // --- Scenario: rejected create states the reason.
    await input.fill('   ');
    await addBtn.click();
    await page.locator('#create-error').waitFor({ state: 'visible' });
    await noNav();
    const errBlank = await page.locator('#create-error').textContent();
    assert.match(errBlank, /title is required/i, `blank submit stated "${errBlank}"`);
    assert.strictEqual(await rows.count(), 2, 'blank submit created a todo');
    assert.strictEqual(await input.inputValue(), '   ', 'typed text did not stay in the input');
    console.log('scenario 2 (rejected create states the reason): OK');

    // --- Scenario: over-limit create states the limit.
    await input.fill('x'.repeat(LIMIT + 1));
    await addBtn.click();
    await page.waitForFunction(() => {
      const el = document.getElementById('create-error');
      return el && /limit/.test(el.textContent);
    });
    await noNav();
    const errLimit = await page.locator('#create-error').textContent();
    assert.match(errLimit, new RegExp(String(LIMIT)), `limit number not stated: "${errLimit}"`);
    assert.strictEqual(await rows.count(), 2, 'over-limit submit created a todo');
    console.log('scenario 3 (over-limit create states the limit): OK');

    // --- Persistence: reload shows the created todos (a real reload now —
    // the marker clears, proving the reload actually re-read the server).
    await page.reload();
    assert.strictEqual(await page.evaluate('window.__w2_no_nav || 0'), 0,
      'reload did not actually reload the page');
    const titles = await page.locator('#todo-list .title').allTextContents();
    assert.deepStrictEqual(titles, ['write the spec', 'Buy milk'],
      `after reload the list is ${JSON.stringify(titles)}`);
    console.log('scenario 4 (reload shows persistence): OK');

    await stopServer(srv);
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('W2 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
