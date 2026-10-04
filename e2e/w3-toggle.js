// W3 e2e — toggle done in a real browser against the composed server.
// Parent scenario "Mark todo done": check a checkbox → the row shows done
// WITHOUT a full reload; uncheck → not-done; text/position never change;
// after a reload the done state matches the store in both directions.
//
// CommonJS so `require('playwright')` resolves via NODE_PATH=$(npm root -g).
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
const seedBin = process.env.SEED || path.join(binDir, 'seed');

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

// First row of the list, as a locator.
function firstRow(page) {
  return page.locator('#todo-list li').first();
}

function firstCheckbox(page) {
  return firstRow(page).locator('input[type=checkbox]');
}

// Wait until the first row carries the wanted done state (the htmx swap).
function waitRowState(page, state) {
  return page.waitForFunction(
    (want) => document.querySelector('#todo-list li')?.dataset.state === want,
    state,
    { timeout: 10000 },
  );
}

async function titles(page) {
  return (await page.locator('#todo-list .title').allTextContents()).slice();
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    const db = tmpDb('toggle.db');
    run(seedBin, ['--db', db, '--titles', 'first task,second task']);
    const port = await freePort();
    const srv = startServer(port, db);
    await waitForHTTP(`http://127.0.0.1:${port}/todos`);
    await page.goto(`http://127.0.0.1:${port}/`);

    // A marker on the live document: it survives an htmx swap but not a
    // full reload — proof the update happens without one.
    await page.evaluate(() => {
      window.__w3noReload = 'live';
    });

    const titlesBefore = await titles(page);
    assert.deepStrictEqual(titlesBefore, ['first task', 'second task']);
    assert.strictEqual(await firstCheckbox(page).isChecked(), false);

    // --- Mark done: check the box → the row shows done without a reload.
    await firstCheckbox(page).check();
    await waitRowState(page, 'done');
    assert.strictEqual(await firstCheckbox(page).isChecked(), true, 'row does not show done');
    assert.strictEqual(await firstRow(page).locator('button.edit').count(), 0,
      'done row must not carry an edit control');
    assert.strictEqual(await page.evaluate(() => window.__w3noReload), 'live',
      'a full reload happened — the row must update via htmx swap');
    assert.deepStrictEqual(await titles(page), titlesBefore, 'row texts changed');
    assert.strictEqual(await page.locator('#todo-list li').count(), 2, 'row count changed');
    assert.strictEqual(await page.locator('#todo-list li').nth(1).getAttribute('data-state'), 'not-done',
      'the untouched row changed state');
    console.log('scenario 1 (check → done without reload): OK');

    // --- Reopen: the same checkbox → not-done, still no reload.
    await firstCheckbox(page).uncheck();
    await waitRowState(page, 'not-done');
    assert.strictEqual(await firstCheckbox(page).isChecked(), false, 'row still shows done');
    assert.strictEqual(await firstRow(page).locator('button.edit').count(), 1,
      'reopened row carries no edit control');
    assert.strictEqual(await page.evaluate(() => window.__w3noReload), 'live',
      'a full reload happened on reopen');
    assert.deepStrictEqual(await titles(page), titlesBefore, 'row texts changed');
    console.log('scenario 2 (uncheck → not-done without reload): OK');

    // --- Persistence: done survives a full reload (store truth).
    await firstCheckbox(page).check();
    await waitRowState(page, 'done');
    await page.reload();
    assert.strictEqual(await firstRow(page).getAttribute('data-state'), 'done',
      'done state did not survive reload');
    assert.strictEqual(await firstCheckbox(page).isChecked(), true);
    assert.deepStrictEqual(await titles(page), titlesBefore, 'row texts changed across reload');
    console.log('scenario 3 (done persists across reload): OK');

    // --- Persistence, other direction: not-done survives a full reload.
    await firstCheckbox(page).uncheck();
    await waitRowState(page, 'not-done');
    await page.reload();
    assert.strictEqual(await firstRow(page).getAttribute('data-state'), 'not-done',
      'reopened state did not survive reload');
    assert.strictEqual(await firstCheckbox(page).isChecked(), false);
    console.log('scenario 4 (reopen persists across reload): OK');

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
  console.log('W3 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
