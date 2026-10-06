// KW1 e2e — the board page opens against the composed server and renders
// the KW1 states truthfully, re-executing the parent scenarios "Board shows
// fixed columns" and "Start without todo data" (workplan_kanban_application.md;
// wave-end contract: workplans/dependencies_kanban.md §KW1). CommonJS so
// `require('playwright')` resolves via NODE_PATH=$(npm root -g).
//
// Flow: build binaries -> seed board db -> start server -> assert the
// populated board (fixed columns, stored order, done treatment) -> stop;
// fresh temp dbs (no seeding) -> the three empty stated columns.
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

function getJSON(url) {
  return new Promise((resolve, reject) => {
    http
      .get(url, (res) => {
        let body = '';
        res.on('data', (d) => (body += d));
        res.on('end', () => {
          if (res.statusCode !== 200) return reject(new Error(`GET ${url} -> ${res.statusCode}`));
          try {
            resolve(JSON.parse(body));
          } catch (err) {
            reject(err);
          }
        });
      })
      .on('error', reject);
  });
}

// The composition root still opens both stores until the todo endpoints
// retire (KW4): point each flag at its own temp file so no run touches a
// file beside the repo.
function startServer(port, todoDb, boardDb) {
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--db', todoDb, '--board-db', boardDb], {
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

function tmpPaths(name) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kanban-e2e-'));
  return { todo: path.join(dir, name + '.todos.db'), board: path.join(dir, name + '.kanban.db'), dir };
}

// The fixed columns as the contract states them — the render mirrors this
// order; the page must show exactly it.
const COLUMN_TITLES = ['To Do', 'In Progress', 'Done'];
const COLUMN_ANCHORS = ['to-do', 'in-progress', 'done'];

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    // --- Scenario 1: "Board shows fixed columns" — seeded board with
    // cards in all three columns; the page shows the three fixed columns
    // in order, cards top-to-bottom in stored position order, and the
    // done treatment carried purely by Done-column membership.
    const seededTitles = [
      'Write the weekly report', // -> To Do
      'Call the plumber',        // -> To Do
      'Draft the launch note',   // -> In Progress
      'Review the migration plan', // -> In Progress
      'Ship v1.2',               // -> Done
      'Archive the old site',    // -> Done
    ];
    const expectedByAnchor = {
      'to-do': [seededTitles[0], seededTitles[1]],
      'in-progress': [seededTitles[2], seededTitles[3]],
      'done': [seededTitles[4], seededTitles[5]],
    };
    const dbs1 = tmpPaths('populated');
    run(seedBin, [
      '--db', dbs1.board,
      '--titles', seededTitles.join(','),
      '--in-progress', '2,3',
      '--done', '4,5',
    ]);
    const port1 = await freePort();
    const srv1 = startServer(port1, dbs1.todo, dbs1.board);
    await waitForHTTP(`http://127.0.0.1:${port1}/board`);
    await page.goto(`http://127.0.0.1:${port1}/`);

    const columns = page.locator('#board > section.column');
    assert.strictEqual(await columns.count(), 3, 'the board must show exactly three columns');
    const shownTitles = await columns.locator('h2.column__title').allTextContents();
    assert.deepStrictEqual(shownTitles, COLUMN_TITLES, `columns not in the fixed order: ${JSON.stringify(shownTitles)}`);
    const shownAnchors = await columns.evaluateAll((els) => els.map((el) => el.id));
    assert.deepStrictEqual(shownAnchors, COLUMN_ANCHORS.map((a) => `column-${a}`),
      `column elements not in the fixed order: ${JSON.stringify(shownAnchors)}`);

    // Cards top-to-bottom in stored position order: the DOM order per
    // column must equal the contract's array order (the stored order),
    // and the texts must match the seeded order.
    const boardJSON = await getJSON(`http://127.0.0.1:${port1}/board`);
    assert.deepStrictEqual(boardJSON.columns.map((c) => c.title), COLUMN_TITLES,
      'GET /board columns not in the fixed order');
    for (const anchor of COLUMN_ANCHORS) {
      const colTitle = COLUMN_TITLES[COLUMN_ANCHORS.indexOf(anchor)];
      const colJSON = boardJSON.columns.find((c) => c.title === colTitle);
      assert.deepStrictEqual(
        colJSON.cards.map((c) => c.position),
        colJSON.cards.map((_, i) => i),
        `stored positions in "${colTitle}" are not 0..n-1: ${JSON.stringify(colJSON.cards)}`
      );
      const domTitles = await page.locator(`#column-${anchor} ul.column__cards > li .card__title`).allTextContents();
      assert.deepStrictEqual(domTitles, expectedByAnchor[anchor],
        `cards in "${colTitle}" not top-to-bottom in stored order: ${JSON.stringify(domTitles)}`);
      const domIds = await page.locator(`#column-${anchor} ul.column__cards > li.card`).evaluateAll(
        (els) => els.map((el) => Number(el.dataset.card)));
      assert.deepStrictEqual(domIds, colJSON.cards.map((c) => c.id),
        `DOM card order in "${colTitle}" does not mirror the stored order`);
    }

    // Done treatment: exactly the Done column's cards carry it, nowhere
    // else — and no per-card done control exists anywhere on the page.
    for (const anchor of COLUMN_ANCHORS) {
      const doneClass = await page
        .locator(`#column-${anchor} ul.column__cards > li.card`)
        .evaluateAll((els) => els.map((el) => el.classList.contains('card--done')));
      const wantDone = anchor === 'done';
      for (const [i, has] of doneClass.entries()) {
        assert.strictEqual(has, wantDone,
          `card ${i} in "${anchor}" done treatment ${has ? 'present' : 'missing'} — membership of the Done column is the only done state`);
      }
    }
    assert.strictEqual(await page.locator('input[type=checkbox]').count(), 0,
      'no done checkbox or toggle may exist anywhere on the page');

    await stopServer(srv1);
    console.log('scenario 1 (board shows fixed columns): OK');

    // --- Scenario 2: "Start without todo data" — no data store exists at
    // all; the server starts, the board is created empty, and the page
    // shows the three fixed columns each stating its emptiness.
    const dbs2 = tmpPaths('fresh'); // both files absent — nothing seeded, nothing opened before start
    const port2 = await freePort();
    const srv2 = startServer(port2, dbs2.todo, dbs2.board);
    await waitForHTTP(`http://127.0.0.1:${port2}/board`);
    await page.goto(`http://127.0.0.1:${port2}/`);

    const freshColumns = page.locator('#board > section.column');
    assert.strictEqual(await freshColumns.count(), 3, 'a fresh board must still show exactly three fixed columns');
    const freshTitles = await freshColumns.locator('h2.column__title').allTextContents();
    assert.deepStrictEqual(freshTitles, COLUMN_TITLES,
      `fresh board columns not in the fixed order: ${JSON.stringify(freshTitles)}`);
    for (const anchor of COLUMN_ANCHORS) {
      const empties = page.locator(`#column-${anchor} p.column__empty`);
      assert.strictEqual(await empties.count(), 1, `column "${anchor}" must state its emptiness`);
      assert.ok(await empties.isVisible(), `column "${anchor}" emptiness statement not visible`);
    }
    assert.strictEqual(await page.locator('#board li.card').count(), 0, 'a fresh board shows no cards');
    const freshJSON = await getJSON(`http://127.0.0.1:${port2}/board`);
    assert.deepStrictEqual(freshJSON.columns.map((c) => [c.title, c.cards]), COLUMN_TITLES.map((t) => [t, []]),
      'GET /board on a fresh board must answer the three fixed columns, each empty');

    await stopServer(srv2);
    console.log('scenario 2 (start without todo data): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW1 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
