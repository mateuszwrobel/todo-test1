// W8 e2e — in-flight control serialization (card ui/16) in a real browser.
// While the operation triggered by a control is in flight, repeat activation
// of THAT control causes no request; the control is live again once the
// response arrives — success AND failure. The in-flight window is made
// observable with a Playwright route delay on the ui fragment endpoints:
// every control request is counted at the proxy route, held in flight
// (~400ms, XHR pending → htmx keeps the control disabled), then forwarded.
// Exactly one routed request per double activation + the server-truth check
// via GET /todos together prove "no additional effect beyond the single
// operation" (parent scenario wording).
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
const IN_FLIGHT_MS = 400;

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

function httpJSON(port, p) {
  return new Promise((resolve, reject) => {
    http
      .get({ host: '127.0.0.1', port, path: p }, (res) => {
        let body = '';
        res.on('data', (d) => (body += d));
        res.on('end', () => {
          if (res.statusCode !== 200) return reject(new Error(`GET ${p} -> ${res.statusCode}`));
          try {
            resolve(JSON.parse(body));
          } catch (e) {
            reject(e);
          }
        });
      })
      .on('error', reject);
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

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  // Counting proxy for the ui fragment endpoints: every control activation
  // is counted, held in flight (delay BEFORE continue keeps the browser-side
  // request pending, so the disabled state is observable), then forwarded to
  // the real server. failSpec, when set, fails matching requests with 500
  // after the same delay (the failure path).
  let counts = {};
  let delayMs = IN_FLIGHT_MS;
  let failSpec = null;
  const snapshot = () => ({ ...counts });
  const callsSince = (snap, key) => (counts[key] || 0) - (snap[key] || 0);

  // Poll a predicate (retry until true or deadline).
  async function poll(fn, label, timeoutMs = 10000) {
    const start = Date.now();
    for (;;) {
      if (await fn()) return;
      if (Date.now() - start > timeoutMs) throw new Error(`timeout waiting for: ${label}`);
      await sleep(25);
    }
  }

  // Wait until the control carries (or clears) the disabled state.
  function waitDisabled(selector, want) {
    return page.waitForFunction(
      ([sel, w]) => {
        const el = document.querySelector(sel);
        return !!el && el.disabled === w;
      },
      [selector, want],
      { timeout: 10000 },
    );
  }

  try {
    const db = tmpDb('inflight.db');
    run(seedBin, ['--db', db, '--titles', 'alpha task,beta task,gamma task']);
    const port = await freePort();
    const srv = startServer(port, db);
    const base = `http://127.0.0.1:${port}`;
    await waitForHTTP(`${base}/todos`);

    await page.route('**/ui/**', async (route) => {
      const req = route.request();
      const method = req.method();
      const url = new URL(req.url());
      const key = `${method} ${url.pathname.replace(/\/\d+$/, '/:id')}`;
      counts[key] = (counts[key] || 0) + 1;
      await sleep(delayMs);
      if (failSpec && failSpec.method === method && failSpec.pathRe.test(url.pathname)) {
        await route.fulfill({ status: 500, contentType: 'text/plain', body: 'forced in-flight failure' });
        return;
      }
      await route.continue();
    });

    await page.goto(`${base}/`);
    assert.strictEqual(await page.locator('#todo-list li').count(), 3);

    // --- ui/16 · row checkbox toggle: second activation while in flight
    // causes no request; the row ends from exactly ONE toggle; the control
    // is live again after the response; other rows' controls stay usable.
    {
      const cb = '#todo-1 input[type=checkbox]';
      const otherCb = '#todo-2 input[type=checkbox]';
      const otherDel = '#todo-2 button.delete';
      const snap = snapshot();
      await page.click(cb);
      await waitDisabled(cb, true);
      // Per-control scope: only the activating control is blocked.
      assert.strictEqual(await page.evaluate((s) => document.querySelector(s).disabled, otherCb), false,
        'a checkbox on another row was blocked — the card blocks the control in flight, not the page');
      assert.strictEqual(await page.evaluate((s) => document.querySelector(s).disabled, otherDel), false,
        'a delete control on another row was blocked');
      await page.click(cb, { force: true }); // trusted click on a disabled control
      await poll(async () => (await httpJSON(port, '/todos'))[0].done === true, 'toggle landed');
      assert.strictEqual(callsSince(snap, 'PATCH /ui/todos/:id'), 1,
        'repeat activation of the checkbox caused an additional request');
      assert.strictEqual((await httpJSON(port, '/todos'))[0].done, true,
        'the list must end in the state produced by exactly one toggle');
      await waitDisabled(cb, false);
      console.log('scenario 1 (checkbox: double activation -> one toggle, re-enabled): OK');
    }

    // --- ui/16 · create Add button: double activation posts exactly one
    // create; the new title exists exactly once; the button re-enables.
    {
      await page.fill('#create-form input[name=title]', 'one-shot task');
      const add = '#create-form button[type=submit]';
      const snap = snapshot();
      await page.click(add);
      await waitDisabled(add, true);
      await page.click(add, { force: true });
      await poll(async () => (await httpJSON(port, '/todos')).length === 4, 'fourth todo stored');
      assert.strictEqual(callsSince(snap, 'POST /ui/todos'), 1,
        'repeat activation of Add caused an additional request');
      const stored = await httpJSON(port, '/todos');
      assert.strictEqual(stored.filter((t) => t.title === 'one-shot task').length, 1,
        'the list ends with exactly one created todo from the double activation');
      await waitDisabled(add, false);
      console.log('scenario 2 (create Add: double activation -> one create, re-enabled): OK');
    }

    // --- ui/16 · edit-save: double activation of Save sends exactly one
    // PATCH; the title lands edited once; Save re-enables.
    {
      await page.click('#todo-2 button.edit'); // opens the band, no request
      await page.fill('#todo-2 .edit-form input[name=title]', 'beta edited');
      const save = '#todo-2 .edit-form button.save';
      const snap = snapshot();
      await page.click(save);
      await waitDisabled(save, true);
      await page.click(save, { force: true });
      await poll(async () => (await httpJSON(port, '/todos')).find((t) => t.id === 2).title === 'beta edited',
        'edit landed');
      assert.strictEqual(callsSince(snap, 'PATCH /ui/todos/:id'), 1,
        'repeat activation of Save caused an additional request');
      const stored = await httpJSON(port, '/todos');
      assert.strictEqual(stored.find((t) => t.id === 2).title, 'beta edited');
      assert.ok(!stored.some((t) => t.title === 'beta task'), 'the double activation produced more than one edit effect');
      await waitDisabled(save, false);
      console.log('scenario 3 (edit Save: double activation -> one edit, re-enabled): OK');
    }

    // --- ui/16 · row delete: double activation sends exactly one DELETE;
    // exactly one row is gone; the button re-enables.
    {
      const del = '#todo-3 button.delete';
      const snap = snapshot();
      await page.click(del);
      await waitDisabled(del, true);
      await page.click(del, { force: true });
      await poll(async () => (await httpJSON(port, '/todos')).length === 3, 'row deleted');
      assert.strictEqual(callsSince(snap, 'DELETE /ui/todos/:id'), 1,
        'repeat activation of Delete caused an additional request');
      const stored = await httpJSON(port, '/todos');
      assert.ok(!stored.some((t) => t.id === 3), 'the double activation deleted more than the one todo');
      assert.strictEqual(stored.length, 3, 'row count changed beyond the single delete');
      console.log('scenario 4 (delete: double activation -> one delete, re-enabled): OK');
    }

    // --- failure path: the in-flight failure must not leave the control
    // dead. A forced 500 on the toggle PATCH: the double activation still
    // sends one request, the failure has NO server-side effect, the
    // checkbox re-enables, and a retry afterwards works.
    {
      const cb = '#todo-4 input[type=checkbox]';
      failSpec = { method: 'PATCH', pathRe: /\/ui\/todos\/\d+$/ };
      const snap = snapshot();
      await page.click(cb);
      await waitDisabled(cb, true);
      await page.click(cb, { force: true });
      await waitDisabled(cb, false); // re-enabled by the failed response
      assert.strictEqual(callsSince(snap, 'PATCH /ui/todos/:id'), 1,
        'repeat activation during a failing in-flight window caused an additional request');
      assert.strictEqual((await httpJSON(port, '/todos')).find((t) => t.id === 4).done, false,
        'the failed operation produced a server-side effect');
      // The control is live: a retry (no delay, no failure) succeeds.
      failSpec = null;
      delayMs = 20;
      await page.click(cb);
      await poll(async () => (await httpJSON(port, '/todos')).find((t) => t.id === 4).done === true,
        'retry after failure landed');
      console.log('scenario 5 (failure path: control re-enables, retry succeeds): OK');
    }

    // Final server truth: exactly the effects of the operations run once.
    const stored = await httpJSON(port, '/todos');
    assert.deepStrictEqual(
      stored.map((t) => [t.title, t.done]),
      [
        ['alpha task', true],
        ['beta edited', false],
        ['one-shot task', true],
      ],
      'final store does not match exactly-one-effect history',
    );

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
  console.log('W8 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
