// W7 e2e — lifecycle in real processes: restart resumes state (parent
// scenario "Todos survive server restart") and SIGTERM completes in-flight
// work. Real binaries, real signals; chromium verifies the page view.
//
// Note: state changes mid-run go through the toggle PATCH over seeded rows —
// the same route the browser's checkbox uses (as in the W3/W5 e2e). POST
// /todos is mounted too, but the PATCH keeps the mix explicit.
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
  // Attach the exit waiter at start so no exit is ever missed.
  proc._exit = new Promise((resolve, reject) => {
    const deadline = setTimeout(() => {
      proc.kill('SIGKILL');
      reject(new Error(`server did not exit after 60s; stderr: ${stderr}`));
    }, 60000);
    proc.on('exit', (code, signal) => {
      clearTimeout(deadline);
      resolve({ code, signal });
    });
  });
  return proc;
}

function stopServer(proc) {
  proc.kill('SIGTERM');
  return proc._exit;
}

function tmpDb(name) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'todo-e2e-'));
  return path.join(dir, name);
}

function getJSON(url) {
  return new Promise((resolve, reject) => {
    http
      .get(url, (res) => {
        let body = '';
        res.on('data', (d) => (body += d));
        res.on('end', () => resolve(JSON.parse(body)));
      })
      .on('error', reject);
  });
}

// slowPatch sends a PATCH whose body is deliberately left incomplete: the
// server handler blocks reading the request body until finish() delivers the
// last bytes — real in-flight work to be caught by a shutdown signal.
function slowPatch(port, id, doneValue) {
  const body = JSON.stringify({ done: doneValue });
  const sock = net.connect(port, '127.0.0.1');
  const connected = new Promise((resolve, reject) => {
    sock.once('connect', () => {
      sock.write(
        `PATCH /todos/${id} HTTP/1.1\r\nHost: 127.0.0.1:${port}\r\n` +
          `Content-Type: application/json\r\nContent-Length: ${body.length}\r\n\r\n` +
          body.slice(0, body.length - 2),
      );
      resolve();
    });
    sock.once('error', reject);
  });
  const response = new Promise((resolve, reject) => {
    let buf = '';
    sock.on('data', (d) => {
      buf += d;
      const m = buf.match(/HTTP\/1\.1 (\d{3})/);
      if (m) resolve(Number(m[1]));
    });
    sock.once('error', reject);
  });
  return connected.then(() => ({
    finish: () => sock.write(body.slice(body.length - 2)),
    response,
  }));
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    // ---- Scenario 1: Todos survive server restart (parent J1).
    // The mixed done/not-done mix is seeded; one toggle goes through the
    // live page; the server is stopped and started again on the same file.
    const db = tmpDb('restart.db');
    run(seedBin, ['--db', db, '--titles', 'alpha task,beta task,gamma task', '--done', '1']);
    const port = await freePort();
    let srv = startServer(port, db);
    await waitForHTTP(`http://127.0.0.1:${port}/todos`);
    await page.goto(`http://127.0.0.1:${port}/`);

    // A live page operation: toggle the first row done through the browser.
    await page.locator('#todo-list li').first().locator('input[type=checkbox]').check();
    await page.waitForFunction(
      () => document.querySelector('#todo-list li')?.dataset.state === 'done',
      undefined,
      { timeout: 10000 },
    );

    const before = await getJSON(`http://127.0.0.1:${port}/todos`);
    assert.deepStrictEqual(
      before.map((t) => [t.title, t.done]),
      [['alpha task', true], ['beta task', true], ['gamma task', false]],
      'unexpected state before restart',
    );

    await stopServer(srv);
    srv = startServer(port, db);
    await waitForHTTP(`http://127.0.0.1:${port}/todos`);

    const after = await getJSON(`http://127.0.0.1:${port}/todos`);
    assert.deepStrictEqual(after, before, 'JSON contract changed across restart');

    await page.reload();
    const rows = await page.locator('#todo-list li').evaluateAll((lis) =>
      lis.map((li) => [li.querySelector('.title').textContent, li.dataset.state]),
    );
    assert.deepStrictEqual(
      rows,
      [
        ['alpha task', 'done'],
        ['beta task', 'done'],
        ['gamma task', 'not-done'],
      ],
      'page did not resume the same todos with the same states',
    );
    console.log('scenario 1 (restart resumes state): OK');
    await stopServer(srv);

    // ---- Scenario 2: SIGTERM completes in-flight work (server/04).
    const db2 = tmpDb('shutdown.db');
    run(seedBin, ['--db', db2, '--titles', 'in flight']);
    const port2 = await freePort();
    srv = startServer(port2, db2);
    await waitForHTTP(`http://127.0.0.1:${port2}/todos`);

    const patch = await slowPatch(port2, 1, true);
    await new Promise((r) => setTimeout(r, 100)); // the handler is now mid-flight
    const exitP = srv._exit;
    srv.kill('SIGTERM');
    patch.finish();
    const status = await patch.response;
    assert.strictEqual(status, 200, 'in-flight request did not finish its response');
    const exit = await exitP;
    assert.strictEqual(exit.code, 0, `server exited ${exit.code}/${exit.signal}, want 0`);

    const stillListening = await getJSON(`http://127.0.0.1:${port2}/todos`).then(
      () => true,
      () => false,
    );
    assert.ok(!stillListening, 'listener still accepts after shutdown');

    // Durability: start again on the same file — the completed operation is
    // there.
    const port3 = await freePort();
    const srv2 = startServer(port3, db2);
    await waitForHTTP(`http://127.0.0.1:${port3}/todos`);
    const after2 = await getJSON(`http://127.0.0.1:${port3}/todos`);
    assert.deepStrictEqual(
      after2.map((t) => [t.title, t.done]),
      [['in flight', true]],
      'in-flight change was not durable across shutdown',
    );
    await stopServer(srv2);
    console.log('scenario 2 (SIGTERM completes in-flight work, exit 0): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('W7 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
