// W5 e2e — delete end-to-end in a real browser against the composed server.
// CommonJS so `require('playwright')` resolves via NODE_PATH=$(npm root -g).
//
// Flow: seed mixed-state db -> open page -> click delete on one row ->
// assert row gone WITHOUT any navigation event, other rows keep text, done
// state, order -> reload -> deletion persisted -> delete rows down to the
// last -> empty state arrives without navigation.
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

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    // --- Scenario 1: click delete -> row removed by swap, no navigation.
    const db1 = tmpDb('delete.db');
    run(seedBin, [
      '--db', db1,
      '--titles', 'read the spec,write the code,run the tests',
      '--done', '1',
    ]);
    const port1 = await freePort();
    const srv1 = startServer(port1, db1);
    await waitForHTTP(`http://127.0.0.1:${port1}/todos`);

    const navigations = [];
    const onNav = (frame) => { if (frame === page.mainFrame()) navigations.push(frame.url()); };
    page.on('framenavigated', onNav);

    await page.goto(`http://127.0.0.1:${port1}/`);
    const urlAfterLoad = page.url();
    assert.strictEqual(await page.locator('#todo-list li').count(), 3,
      'expected three seeded rows before deleting');

    await page.locator('#todo-list li').nth(1).locator('button.delete').click();

    // The swap, not a reload: row count settles without a navigation event.
    await page.waitForFunction(
      () => document.querySelectorAll('#todo-list li').length === 2, undefined,
      { timeout: 5000 },
    );
    assert.strictEqual(page.url(), urlAfterLoad, 'delete must not navigate away');
    assert.strictEqual(navigations.length, 1,
      `only the initial goto may navigate, saw: ${JSON.stringify(navigations)}`);

    // Remaining rows keep text, done state, and relative order.
    const rows = page.locator('#todo-list li');
    assert.deepStrictEqual(await rows.locator('.title').allTextContents(),
      ['read the spec', 'run the tests'], 'remaining rows lost text or order');
    assert.strictEqual(await rows.nth(0).locator('input[type=checkbox]').isChecked(), false);
    assert.strictEqual(await rows.nth(1).locator('input[type=checkbox]').isChecked(), false);

    // --- Scenario 2: reload -> the deletion persisted server-side.
    await page.reload();
    assert.deepStrictEqual(
      await page.locator('#todo-list li').locator('.title').allTextContents(),
      ['read the spec', 'run the tests'],
      'deletion did not survive a reload',
    );

    // --- Scenario 3: delete down to the last row -> empty state, no navigation.
    navigations.length = 0;
    await page.locator('#todo-list li').nth(0).locator('button.delete').click();
    await page.waitForFunction(
      () => document.querySelectorAll('#todo-list li').length === 1, undefined,
      { timeout: 5000 },
    );
    await page.locator('#todo-list li').nth(0).locator('button.delete').click();
    await page.waitForSelector('#empty-state', { timeout: 5000 });
    assert.strictEqual(await page.locator('#todo-list').count(), 0,
      'the list must be gone once the last todo is deleted');
    assert.ok(await page.locator('#create-form input[name=title]').count(),
      'create control stays ready on the empty page');
    assert.strictEqual(navigations.length, 0,
      `post-reload deletes must not navigate, saw: ${JSON.stringify(navigations)}`);
    await page.reload();
    assert.ok(await page.locator('#empty-state').count(),
      'empty page after deleting every todo must persist across reload');

    page.off('framenavigated', onNav);
    await stopServer(srv1);
    console.log('scenario 1+2 (row delete without navigation + reload persistence): OK');
    console.log('scenario 3 (last delete lands empty state): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('W5 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
