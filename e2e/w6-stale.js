// W6 e2e — stale page / missing todo, real browser against the composed
// server. Card ui/14: a page stale about a todo that no longer exists must
// STATE the failure for any operation — toggle, edit-save, and delete of the
// missing id each surface the missing-todo banner (the contract's "no such
// todo"), never a silent no-op, never a row left looking like the failed
// operation succeeded. A reload then shows exactly the server truth — no
// ghost rows, no banner outliving the failure.
//
// The staleness is produced the way the card frames it: a second surface
// (direct api call = another page) DELETEs the todo behind the page's back.
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

async function step(name, fn) {
  try {
    await fn();
    console.log(`ok   ${name}`);
  } catch (err) {
    console.log(`FAIL ${name}\n     ${String(err).split('\n')[0]}`);
    throw err;
  }
}

// serverJSON reads the api contract's full truth (GET /todos) from inside
// the page context, so staleness is created and verified on the very
// contract the composed server serves.
const serverJSON = (page) =>
  page.evaluate(async () => {
    const r = await fetch('/todos');
    return { status: r.status, body: await r.json() };
  });

// apiDelete = "the todo was deleted meanwhile" — the second surface removing
// one todo behind the open page's back.
const apiDelete = (page, id) =>
  page.evaluate(async (todoId) => {
    const r = await fetch(`/todos/${todoId}`, { method: 'DELETE' });
    return r.status;
  }, id);

// assertMissingStated waits for the stated failure and checks it is the
// missing-todo banner, rendered exactly once, saying the todo does not
// exist ("no such todo" — the contract's own reason).
async function assertMissingStated(page) {
  const banner = page.locator('#missing-todo-banner');
  await page.waitForSelector('#missing-todo-banner', { timeout: 5000 });
  assert.strictEqual(await banner.count(), 1, 'the failure banner must render exactly once');
  assert.ok(await banner.isVisible(), 'the failure banner must be visible');
  const text = (await banner.textContent()) || '';
  assert.ok(text.includes('no such todo'),
    `banner does not state the missing todo (contract reason), says: ${JSON.stringify(text)}`);
}

// assertTruthful checks the page re-rendered from server truth: the stale
// row is not left faking the operation, and the survivors are untouched —
// no change occurred anywhere.
async function assertTruthful(page, goneId, survivorsBefore) {
  assert.strictEqual(await page.locator(`#todo-${goneId}`).count(), 0,
    `stale row todo-${goneId} must not be left on the page after the stated failure`);
  const rows = page.locator('#todo-list li');
  const ids = await rows.evaluateAll((els) => els.map((e) => e.id));
  assert.deepStrictEqual(ids, survivorsBefore.map((t) => `todo-${t.id}`),
    'surviving rows lost their presence or order — the failed operation changed the page');
  for (let i = 0; i < survivorsBefore.length; i++) {
    const t = survivorsBefore[i];
    assert.strictEqual(await rows.nth(i).locator('.title').textContent(), t.title,
      `survivor ${t.id} lost its text`);
    assert.strictEqual(await rows.nth(i).locator('input[type=checkbox]').isChecked(), t.done,
      `survivor ${t.id} lost its done state — a row looks like it succeeded`);
  }
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const db = tmpDb('stale.db');
  run(seedBin, ['--db', db, '--titles', 'read the spec,write the code,run the tests']);
  const port = await freePort();
  const srv = startServer(port, db);
  await waitForHTTP(`http://127.0.0.1:${port}/todos`);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  try {
    await page.goto(`http://127.0.0.1:${port}/`);
    const baseURL = page.url();
    assert.strictEqual(await page.locator('#todo-list li').count(), 3, 'seeded three rows expected');

    // --- ui/14 · toggle on a missing todo states the failure.
    await step('toggle on missing todo -> banner states it, no fake success', async () => {
      const rowA = page.locator('#todo-list li').nth(0);
      const idA = Number(await rowA.evaluate((el) => el.id.replace('todo-', '')));
      assert.strictEqual(await apiDelete(page, idA), 204, 'behind-the-back delete failed');
      const truth = (await serverJSON(page)).body;

      await rowA.locator('input[type=checkbox]').click();
      await assertMissingStated(page);
      assert.strictEqual(page.url(), baseURL, 'stated failure must not navigate');
      await assertTruthful(page, idA, truth);
      assert.deepStrictEqual((await serverJSON(page)).body, truth,
        'the failed toggle changed the server — no change must occur anywhere');
    });

    // Reload shows the same truth as the server — the failure statement (and
    // the ghost row it replaced) never outlive a reload.
    await step('reload after the failed toggle -> server truth, no banner', async () => {
      await page.reload();
      assert.strictEqual(await page.locator('#missing-todo-banner').count(), 0,
        'the stated failure must not survive a reload');
      assert.strictEqual(await page.locator('#todo-list li').count(), 2);
    });

    // --- ui/14 · edit-save on a missing todo states the failure.
    await step('edit-save on missing todo -> same banner, todo stays absent', async () => {
      const rowB = page.locator('#todo-list li').nth(0);
      const idB = Number(await rowB.evaluate((el) => el.id.replace('todo-', '')));
      assert.strictEqual(await apiDelete(page, idB), 204, 'behind-the-back delete failed');
      const truth = (await serverJSON(page)).body;

      await rowB.locator('button.edit').click();
      const band = rowB.locator('form.edit-form');
      await band.waitFor({ state: 'visible' });
      await band.locator('input').evaluate((el) => { el.value = 'edited into the void'; });
      await band.locator('button[type=submit]').click();

      await assertMissingStated(page);
      assert.strictEqual(page.url(), baseURL, 'stated failure must not navigate');
      await assertTruthful(page, idB, truth);
      assert.deepStrictEqual((await serverJSON(page)).body, truth,
        'the failed edit changed the server — no change must occur anywhere');
    });

    await step('reload after the failed edit -> server truth, no banner', async () => {
      await page.reload();
      assert.strictEqual(await page.locator('#missing-todo-banner').count(), 0,
        'the stated failure must not survive a reload');
      assert.strictEqual(await page.locator('#todo-list li').count(), 1);
    });

    // --- ui/14 · delete of a missing todo states the failure.
    await step('delete of missing todo -> same banner, not a silent fake success', async () => {
      const rowC = page.locator('#todo-list li').nth(0);
      const idC = Number(await rowC.evaluate((el) => el.id.replace('todo-', '')));
      assert.strictEqual(await apiDelete(page, idC), 204, 'behind-the-back delete failed');
      const truth = (await serverJSON(page)).body;

      // The stale page still offers Delete for a todo the server has lost.
      await rowC.locator('button.delete').click();
      await assertMissingStated(page);
      assert.strictEqual(page.url(), baseURL, 'stated failure must not navigate');
      // Nothing left faking a removal: the page shows the truth (an empty
      // stated list next to the banner), not a silently-shrunk list.
      assert.strictEqual(await page.locator(`#todo-${idC}`).count(), 0);
      assert.ok(await page.locator('#empty-state').isVisible(),
        'after the failed delete the truth is the stated empty state');
      assert.deepStrictEqual((await serverJSON(page)).body, truth,
        'the failed delete changed the server — no change must occur anywhere');
    });

    await step('reload after the failed delete -> server truth, no banner', async () => {
      await page.reload();
      assert.strictEqual(await page.locator('#missing-todo-banner').count(), 0,
        'the stated failure must not survive a reload');
      assert.ok(await page.locator('#empty-state').isVisible(),
        'the page reloads to the server truth: nothing exists anymore');
    });
  } finally {
    await browser.close();
    await stopServer(srv);
  }

  console.log('W6 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
