// KW2 e2e — create end-to-end in the browser, re-executing the parent
// scenarios "Create card" + "Reject empty card text" + "Reject over-long
// card text" (workplan_kanban_application.md; wave-end contract:
// workplans/dependencies_kanban.md §KW2). CommonJS so
// `require('playwright')` resolves via NODE_PATH=$(npm root -g).
//
// Flow: build binaries -> seed a board with cards in To Do (plus one in
// In Progress) -> start server -> create via the create band and assert the
// no-reload append at To Do's bottom, then the two stated rejections leave
// the board untouched -> SIGTERM teardown. One server serves all three
// scenarios (every scenario's Given is "the app is running" / the seeded
// board — continuity is part of what the lane proves).
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

// The composition root opens only the board data file — the todo
// store retired with the last todo endpoint at KW4. Point the flag at a
// temp file so no run touches a file beside the repo; the KW6 migration flag (--todo-db) points at a never-created path — these lanes state no todo data file, and a superseded todos.db beside the repo must not leak in.
function startServer(port, boardDb) {
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--board-db', boardDb, '--todo-db', boardDb + '.todos-absent'], {
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
  return { board: path.join(dir, name + '.kanban.db'), dir };
}

// Contract truths this lane asserts against: the create input, the submit
// control, the refusal statement at the create control, the To Do column,
// the swap target — the surface cards ui/04 + ui/05 render.
const CREATE_INPUT = '#create-form input.input[name=title]';
const CREATE_SUBMIT = '#create-form button[type=submit]';
const CREATE_ERROR = '#create-area #create-error';
const TODO_CARDS = '#column-to-do ul.column__cards > li.card';

// Snapshot of what the page shows: To Do's card list (title + id per DOM
// card) and every card count on the board.
async function snapshot(page) {
  const todo = await page.locator(TODO_CARDS).evaluateAll((els) =>
    els.map((el) => ({ title: el.querySelector('.card__title').textContent, id: Number(el.dataset.card) }))
  );
  const total = await page.locator('#board li.card').count();
  return { todo, total };
}

// The board as the contract answers it: To Do's card order (title + id) and
// the total card count across columns.
function boardView(json) {
  const todo = json.columns.find((c) => c.title === 'To Do');
  return {
    todo: todo.cards.map((c) => ({ title: c.title, id: c.id })),
    total: json.columns.reduce((n, c) => n + c.cards.length, 0),
  };
}

// Type into the create input and submit — the user's create gesture.
async function create(page, text) {
  await page.fill(CREATE_INPUT, text);
  await page.click(CREATE_SUBMIT);
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    // Seeded board: cards in To Do (the scenario's board "does not contain"
    // the created text), one card in In Progress so the swap re-renders a
    // multi-column board, none carrying the texts the lane creates.
    const seededTitles = [
      'Write the weekly report', // -> To Do
      'Call the plumber',        // -> To Do
      'Draft the launch note',   // -> In Progress
    ];
    const dbs = tmpPaths('create');
    run(seedBin, ['--db', dbs.board, '--titles', seededTitles.join(','), '--in-progress', '2']);
    const port = await freePort();
    const srv = startServer(port, dbs.board);
    await waitForHTTP(`http://127.0.0.1:${port}/board`);
    await page.goto(`http://127.0.0.1:${port}/`);

    // --- Scenario 1: "Create card" — creating a card not on the board puts
    // it at the BOTTOM of To Do without a full page reload (the fragment
    // swap lands in place: the DOM node is added while page-owned window
    // state survives, which no navigation would allow), it is not done, it
    // carries a stable identifier, and a later reload shows it still — in
    // agreement with a fresh GET /board.
    const before = await snapshot(page);
    assert.ok(!before.todo.some((c) => c.title === 'Buy milk'),
      'the seeded board must not already contain the created text');
    // Reload marker: lives on the page's window object only as long as no
    // navigation happens — a full reload would wipe it.
    await page.evaluate(() => { window.__kw2NoReloadMarker = 'alive'; });

    await create(page, 'Buy milk');
    const newCard = page.locator(TODO_CARDS, { hasText: 'Buy milk' });
    await newCard.waitFor(); // the swap landed

    assert.strictEqual(await page.evaluate(() => window.__kw2NoReloadMarker), 'alive',
      'the create must not reload the page — the append is a fragment swap onto the existing render');
    assert.strictEqual((await page.locator(TODO_CARDS).count()), before.todo.length + 1,
      'exactly one card was added to To Do');
    const after = await snapshot(page);
    assert.deepStrictEqual(after.todo.slice(0, -1), before.todo,
      'the previously stored cards keep their order above the new one');
    const created = after.todo[after.todo.length - 1];
    assert.strictEqual(created.title, 'Buy milk', 'the new card must sit at the bottom of To Do');
    assert.ok(!before.todo.some((c) => c.id === created.id),
      'the new card carries an identifier the board did not have before');
    assert.ok(!(await newCard.evaluate((el) => el.classList.contains('card--done'))),
      'a newly created card is not done (it sits in To Do)');

    const boardJSON = await getJSON(`http://127.0.0.1:${port}/board`);
    const stored = boardView(boardJSON).todo;
    assert.deepStrictEqual(stored, after.todo,
      'the DOM order in To Do does not mirror GET /board after the create');
    const storedCreated = stored.find((c) => c.title === 'Buy milk');
    assert.ok(storedCreated, 'GET /board does not contain the created card');
    assert.strictEqual(storedCreated.id, created.id,
      'the card identifier shown in the DOM is not the stored (stable) one');

    // Later reload: the card is still there, still at the bottom — the swap
    // persisted server state, not a client-side guess.
    await page.reload();
    const reloaded = await snapshot(page);
    assert.deepStrictEqual(reloaded.todo, after.todo,
      'after a reload To Do must show the created card at its bottom, same order and ids');
    const freshJSON = await getJSON(`http://127.0.0.1:${port}/board`);
    assert.deepStrictEqual(boardView(freshJSON).todo, reloaded.todo,
      'the reloaded page does not agree with a fresh GET /board');

    await stopServer(srv);
    console.log('scenario 1 (create card): OK');
    // NOTE: server restart for scenario isolation is below — scenarios 2/3
    // run against their own fresh server ("the app is running"), so a
    // rejection lane never inherits create-lane state.
    // (kw1 lanes likewise per-scenario their own dbs/ports.)

    // --- Scenario 2: "Reject empty card text" — submitting blank/whitespace
    // text at the create control states the contract's refusal there, adds
    // no card, leaves the board unchanged, and the input is ready again: a
    // valid submit right after succeeds.
    const dbs2 = tmpPaths('reject-empty');
    run(seedBin, ['--db', dbs2.board, '--titles', 'Write the weekly report,Call the plumber']);
    const port2 = await freePort();
    const srv2 = startServer(port2, dbs2.board);
    await waitForHTTP(`http://127.0.0.1:${port2}/board`);
    await page.goto(`http://127.0.0.1:${port2}/`);

    const before2 = await snapshot(page);
    const jsonBefore2 = boardView(await getJSON(`http://127.0.0.1:${port2}/board`));

    await create(page, '   '); // whitespace-only
    const err = page.locator(CREATE_ERROR);
    await err.waitFor({ state: 'visible' });
    assert.strictEqual((await err.textContent()).trim(), 'title is required',
      'the create control must state the contract\'s empty-text refusal verbatim');
    const after2 = await snapshot(page);
    assert.deepStrictEqual(after2, before2, 'a rejected create must add no card and change nothing');
    assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port2}/board`)), jsonBefore2,
      'a rejected create must leave GET /board unchanged');

    // Ready again: the very next valid submit goes through.
    await create(page, 'Water the plants');
    const readyCard = page.locator(TODO_CARDS, { hasText: 'Water the plants' });
    await readyCard.waitFor();
    const afterReady2 = await snapshot(page);
    assert.strictEqual(afterReady2.todo.length, before2.todo.length + 1,
      'after a rejection the create control must accept a valid card again');
    assert.strictEqual(afterReady2.todo[afterReady2.todo.length - 1].title, 'Water the plants',
      'the post-rejection create must land at the bottom of To Do');

    await stopServer(srv2);
    console.log('scenario 2 (reject empty card text): OK');

    // --- Scenario 3: "Reject over-long card text" — 501 characters is
    // refused at the create control stating the character limit, no card is
    // added, the board is untouched.
    const dbs3 = tmpPaths('reject-long');
    run(seedBin, ['--db', dbs3.board, '--titles', 'Write the weekly report,Call the plumber']);
    const port3 = await freePort();
    const srv3 = startServer(port3, dbs3.board);
    await waitForHTTP(`http://127.0.0.1:${port3}/board`);
    await page.goto(`http://127.0.0.1:${port3}/`);

    const before3 = await snapshot(page);
    const jsonBefore3 = boardView(await getJSON(`http://127.0.0.1:${port3}/board`));

    await create(page, 'a'.repeat(501));
    await err.waitFor({ state: 'visible' });
    const limitText = (await err.textContent()).trim();
    assert.match(limitText, /500/,
      `the create control must state the character limit, got: ${JSON.stringify(limitText)}`);
    assert.match(limitText, /character/i,
      `the refusal must name the character limit, got: ${JSON.stringify(limitText)}`);
    const after3 = await snapshot(page);
    assert.deepStrictEqual(after3, before3, 'a rejected over-long create must add no card');
    assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port3}/board`)), jsonBefore3,
      'a rejected over-long create must leave GET /board unchanged');

    await stopServer(srv3);
    console.log('scenario 3 (reject over-long card text): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW2 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
