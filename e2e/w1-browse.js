// W1 e2e — the page opens against the composed server and renders each W1
// state truthfully. CommonJS so `require('playwright')` resolves via
// NODE_PATH=$(npm root -g).
//
// Flow: build binaries -> seed db -> start server -> assert populated list ->
// stop; fresh db -> empty state; db removed while stopped -> empty state.
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
    // --- Scenario 1: populated list renders rows, order, and states.
    const db1 = tmpDb('populated.db');
    run(seedBin, [
      '--db', db1,
      '--titles', 'read the spec,write the code,run the tests',
      '--done', '1',
    ]);
    const port1 = await freePort();
    const srv1 = startServer(port1, db1);
    await waitForHTTP(`http://127.0.0.1:${port1}/todos`);
    await page.goto(`http://127.0.0.1:${port1}/`);

    const titles = await page.locator('#todo-list .title').allTextContents();
    assert.deepStrictEqual(titles, ['read the spec', 'write the code', 'run the tests'],
      `rows not in creation order: ${JSON.stringify(titles)}`);
    assert.strictEqual(await page.locator('#todo-list li').count(), 3);

    assert.strictEqual(await page.locator('#todo-list li').nth(0).locator('input[type=checkbox]').isChecked(), false);
    assert.strictEqual(await page.locator('#todo-list li').nth(1).locator('input[type=checkbox]').isChecked(), true,
      'done todo must render its checkbox checked');
    assert.strictEqual(await page.locator('#todo-list li').nth(2).locator('input[type=checkbox]').isChecked(), false);

    for (const idx of [0, 1, 2]) {
      assert.ok(await page.locator('#todo-list li').nth(idx).locator('button.delete').count(),
        `row ${idx} has no delete control`);
    }
    assert.strictEqual(await page.locator('#todo-list li').nth(0).locator('button.edit').count(), 1);
    assert.strictEqual(await page.locator('#todo-list li').nth(1).locator('button.edit').count(), 0,
      'done row must not offer an edit control');
    assert.strictEqual(await page.locator('#todo-list li').nth(2).locator('button.edit').count(), 1);

    await stopServer(srv1);
    console.log('scenario 1 (populated list): OK');

    // --- Scenario 2: fresh (empty) db renders the empty state.
    const db2 = tmpDb('fresh.db');
    const port2 = await freePort();
    const srv2 = startServer(port2, db2);
    await waitForHTTP(`http://127.0.0.1:${port2}/todos`);
    await page.goto(`http://127.0.0.1:${port2}/`);
    assert.ok(await page.locator('#empty-state').count(), 'empty state not rendered on fresh db');
    assert.strictEqual(await page.locator('#todo-list').count(), 0);
    assert.strictEqual(await page.locator('#create-form input[name=title]').count(), 1,
      'create input not ready on the empty page');
    await stopServer(srv2);
    console.log('scenario 2 (empty state): OK');

    // --- Scenario 3: db removed while stopped -> start -> empty state shows.
    const db3 = tmpDb('removed.db');
    run(seedBin, ['--db', db3, '--titles', 'was here']);
    const port3 = await freePort();
    const srv3 = startServer(port3, db3); // starts and stops fine with the file present
    await waitForHTTP(`http://127.0.0.1:${port3}/todos`);
    await stopServer(srv3);
    fs.rmSync(db3);
    const port4 = await freePort();
    const srv4 = startServer(port4, db3);
    await waitForHTTP(`http://127.0.0.1:${port4}/todos`);
    await page.goto(`http://127.0.0.1:${port4}/`);
    assert.ok(await page.locator('#empty-state').count(),
      'empty state must show after the db file was removed while stopped');
    await stopServer(srv4);
    console.log('scenario 3 (db removed while stopped -> empty state): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('W1 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
