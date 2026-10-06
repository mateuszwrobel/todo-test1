// KW6 e2e — lifecycle, migration and in-flight serialization end-to-end,
// re-executing the parent scenarios "Board survives server restart",
// "Migrate existing todos on first start" and "Repeat activation while an
// operation is in flight" (workplan_kanban_application.md; wave-end contract
// workplans/dependencies_kanban.md §KW6). It also lands the browser-bound
// proofs ui/inflight_test.go explicitly deferred to this lane: a
// double-clicked Delete sending exactly ONE DELETE, a dragged card refusing
// a second drag while its PATCH is in flight (one PATCH per gesture when a
// double-drag was attempted), silent Enter re-submit against an in-flight
// create, and controls restored and usable after a 4xx rejection leg.
//
// Scenario shapes:
//  1. RESTART — seed a mixed board, mutate through the page (create, edit,
//     drag), SIGTERM the server, assert the process exited (code 0), respawn
//     on the same board data file, and show the page carries the exact truth
//     (DOM ≡ fresh GET /board, positions contiguous) with the window still
//     working: one more operation succeeds.
//  2. FIRST-START MIGRATION — a throwaway superseded todo data file is built
//     with the sqlite3 CLI, its schema quoted verbatim from git
//     48a4ca5^:todos/store.go (the same verbatim quote cmd/todo's
//     migration_test.go seedTodoSource carries), with creation order
//     deliberately interleaved across done states and two rows deleted
//     mid-sequence to leave id gaps. First start with --todo-db at the
//     fixture and a never-created --board-db: the page shows the not-done
//     todos in To Do in creation order, the done ones in Done, In Progress
//     empty, cards under FRESH identifiers (the fixture's gapped ids do not
//     survive), board.db now holds the cards, and the fixture's sha256 is
//     unchanged — the reader is mode=ro. A restart re-shows the identical
//     payload (ids included): the import does not run twice.
//  3. REPEAT ACTIVATION — four legs on four seeded boards, each holding the
//     endpoint in flight with a delayed route so the second activation
//     genuinely lands before the first response: double-clicked Delete →
//     exactly ONE DELETE request and the card gone once; a create submitted
//     by Enter, Enter pressed again mid-flight → silent, exactly ONE POST,
//     input and button disabled while in flight; a drag followed by a second
//     drag attempted on the same card during its PATCH → ONE PATCH for the
//     pair (the card carries draggable=false and dragstart refuses it), and
//     a third legit drag after the response succeeds — controls ready again;
//     a stale-card Delete answered 404 states its failure, no control is
//     left disabled, and a further legit delete succeeds.
//  4. POISONED MIGRATION (loud-fail leg, process-level, no browser) — a
//     fixture holding a blank-title row fails startup: exit code non-zero,
//     stderr names the import, the board file holds zero cards. This mirrors
//     cmd/todo/interrupted_import_test.go at the composed-process level
//     inside the lane idiom (spawn + exit + stderr), cheap and honest.
//
// In-flight instrumentation: a page.on('request')/page.on('response')
// listener pair counts every MUTATION request the browser issues
// (POST/PATCH/DELETE — GET reads never count) with response statuses.
// delayRoute holds one endpoint's request in flight by deferring
// route.continue — the request still goes to the real server, only its
// departure is staged — so "before the first response arrives" is a staged
// fact, not a race. The counters are the proof, not the DOM aftermath alone.
//
// Data discipline: every run works in mkdtemp dirs; the repo-root todos.db
// is NEVER opened — the --todo-db flag always gets either the lane's own
// fixture or a path that is never created.
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

// How long a held request stays in flight. Generously above the few-hundred
// ms a dblclick burst or a refused-drag choreography needs, so the repeat
// activation provably lands mid-flight.
const HOLD_MS = 1500;

function run(cmd, args) {
  const res = spawnSync(cmd, args, { cwd: repoRoot, encoding: 'utf8' });
  assert.strictEqual(res.status, 0, `${cmd} ${args.join(' ')} failed: ${res.stderr}`);
}

// capture runs a command and returns stdout — for the sqlite3 fixture reads
// and the sha256sum hash pins.
function capture(cmd, args) {
  const res = spawnSync(cmd, args, { encoding: 'utf8' });
  assert.strictEqual(res.status, 0, `${cmd} ${args.join(' ')} failed: ${res.stderr}`);
  return res.stdout;
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

// startServer spawns the composed server on an explicit board file and an
// explicit todo source — the migration is this lane's subject, so the flag
// is never left at its default: it points at the lane's fixture or at a
// path that is never created. No run ever reads a file beside the repo.
function startServer(port, boardDb, todoDb) {
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--board-db', boardDb, '--todo-db', todoDb], {
    stdio: ['ignore', 'ignore', 'pipe'],
  });
  let stderr = '';
  proc.stderr.on('data', (d) => (stderr += d));
  proc._stderr = () => stderr;
  return proc;
}

// stopServer SIGTERMs and resolves with the exit {code, signal} — the
// process-exit observation the restart scenario asserts on.
function stopServer(proc) {
  proc.kill('SIGTERM');
  return new Promise((resolve) => proc.on('exit', (code, signal) => resolve({ code, signal })));
}

function tmpPaths(name) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kanban-e2e-'));
  return { board: path.join(dir, name + '.kanban.db'), dir };
}

// ————— the superseded todo fixture (migration scenarios) —————
//
// The schema is the superseded store's table quoted verbatim from git
// 48a4ca5^:todos/store.go — the same verbatim quote cmd/todo's
// migration_test.go seedTodoSource carries; the sqlite3 CLI is the builder,
// the precedent kw3 set for out-of-band data-file work. Titles go in raw:
// the fixtures must be able to hand the migration material the old app
// itself would have refused (the poisoned blank title).
function seedTodoFixture(dbPath, rows) {
  run('sqlite3', [dbPath, `CREATE TABLE IF NOT EXISTS todos (
  id integer primary key autoincrement,
  title text not null,
  done integer not null default 0
);`]);
  for (const row of rows) {
    const title = row.title.replace(/'/g, "''");
    run('sqlite3', [dbPath, `INSERT INTO todos (title, done) VALUES ('${title}', ${row.done ? 1 : 0});`]);
  }
}

function deleteTodoFixtureRows(dbPath, ...titles) {
  for (const title of titles) {
    run('sqlite3', [dbPath, `DELETE FROM todos WHERE title = '${title.replace(/'/g, "''")}';`]);
  }
}

function todoFixtureIds(dbPath) {
  return capture('sqlite3', [dbPath, 'SELECT id FROM todos ORDER BY id;'])
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
    .map(Number);
}

function fileSHA256(p) {
  return capture('sha256sum', [p]).trim().split(/\s+/)[0];
}

function boardCardCount(dbPath) {
  return Number(capture('sqlite3', [dbPath, 'SELECT count(*) FROM cards;']).trim());
}

// Out-of-band card removal (the 4xx leg's Given): kw4's sqlite3 DELETE
// against the board data file, with a busy timeout.
function deleteCardOutOfBand(dbPath, id) {
  run('sqlite3', [dbPath, `PRAGMA busy_timeout=5000; DELETE FROM cards WHERE id = ${id};`]);
}

// ————— shared DOM surfaces (kw1–kw5's selectors) —————
const MISSING_BANNER = '#missing-card';
const CREATE_INPUT = '#create-form .input';
const CREATE_SUBMIT = '#create-form button[type=submit]';
const EMPTY_STATEMENT = (anchor) => `#column-${anchor} p.column__empty[data-empty="true"]`;

// The whole board as the DOM shows it: per fixed column, its cards
// top-to-bottom as {title, id, done} — the kw1–kw5 assertion surface.
async function snapshot(page) {
  return page.locator('#board > section.column').evaluateAll((sections) =>
    sections.map((section) => ({
      column: section.dataset.column,
      cards: [...section.querySelectorAll('ul.column__cards > li.card')].map((li) => ({
        title: li.querySelector('.card__title').textContent,
        id: Number(li.dataset.card),
        done: li.classList.contains('card--done'),
      })),
    }))
  );
}

// The board as the contract answers it, shaped like snapshot() so DOM and
// GET /board can be compared directly — "DOM ≡ fresh GET /board" pins.
function boardView(json) {
  return json.columns.map((c) => ({
    column: c.title,
    cards: c.cards.map((card) => ({
      title: card.title,
      id: card.id,
      done: c.title === 'Done',
    })),
  }));
}

function assertContiguousPositions(json, where) {
  for (const col of json.columns) {
    col.cards.forEach((card, i) => {
      assert.strictEqual(card.position, i,
        `${where}: ${col.title} card ${card.id} carries position ${card.position}, not the contiguous index ${i}`);
    });
  }
}

function cardById(page, id) {
  return page.locator(`li.card[data-card="${id}"]`);
}

function titlesOf(view, columnTitle) {
  const col = view.find((c) => c.column === columnTitle);
  return col.cards.map((c) => c.title);
}

function allCardIds(view) {
  return view
    .flatMap((col) => col.cards.map((c) => c.id))
    .sort((a, b) => a - b);
}

// ————— in-flight instrumentation —————
//
// One listener pair per page counts every MUTATION request the browser
// issues (POST/PATCH/DELETE — GET reads never count) and every mutation
// response with its status. The counters, not the DOM aftermath alone, are
// the repeat-activation proof.
function instrumentMutations(page) {
  const log = { requests: [], responses: [] };
  log.reset = () => {
    log.requests.length = 0;
    log.responses.length = 0;
  };
  const hits = (r, method, id) =>
    r.method === method &&
    (!id || (() => {
      const path = new URL(r.url).pathname; // /ui/cards/{id} (edit/delete) or /ui/cards/{id}/move
      const base = `/ui/cards/${id}`;
      return path === base || path.startsWith(base + '/');
    })());
  log.for = (method, id) => log.requests.filter((r) => hits(r, method, id));
  log.statuses = (method, id) => log.responses.filter((r) => hits(r, method, id)).map((r) => r.status);
  page.on('request', (req) => {
    if (['POST', 'PATCH', 'DELETE'].includes(req.method())) {
      log.requests.push({ method: req.method(), url: req.url(), body: req.postData() });
    }
  });
  page.on('response', (res) => {
    const method = res.request().method();
    if (['POST', 'PATCH', 'DELETE'].includes(method)) {
      log.responses.push({ method, url: res.url(), status: res.status() });
    }
  });
  return log;
}

// delayRoute registers the flight-hold on a URL glob for one verb and
// returns the held-URL list — its length is the lane's witness that the
// request really was in flight when the repeat activation fired. Deferred
// route.continue keeps it a REAL request to the REAL server; only the
// departure is staged.
async function delayRoute(page, urlGlob, method) {
  const held = [];
  await page.route(urlGlob, async (route) => {
    if (route.request().method() === method) {
      held.push(route.request().url());
      await new Promise((resolve) => setTimeout(resolve, HOLD_MS));
    }
    await route.continue();
  });
  return held;
}

async function unrout(page, urlGlob) {
  await page.unroute(urlGlob);
}

// waitUntil polls a node-side predicate — for the log/held arrays, which
// live outside the page.
async function waitUntil(predicate, what, timeoutMs = 8000) {
  const start = Date.now();
  while (!predicate()) {
    if (Date.now() - start > timeoutMs) throw new Error(`timeout waiting for ${what}`);
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
}

async function waitForCardGone(page, id) {
  await page.waitForFunction(
    (cardId) => !document.querySelector(`li.card[data-card="${cardId}"]`),
    id,
    { timeout: 10000 }
  );
}

async function waitForCardIn(page, id, columnTitle) {
  await page.waitForFunction(
    ([cardId, title]) => {
      const li = document.querySelector(`li.card[data-card="${cardId}"]`);
      return !!li && li.closest('.column').dataset.column === title;
    },
    [id, columnTitle],
    { timeout: 10000 }
  );
}

async function waitForTitleInTodo(page, title) {
  await page.waitForFunction(
    (t) => [...document.querySelectorAll('#column-to-do .card__title')].some((el) => el.textContent === t),
    title,
    { timeout: 10000 }
  );
}

// ————— gestures (kw2–kw5's choreographies) —————
async function createCard(page, title) {
  await page.locator(CREATE_INPUT).fill(title);
  await page.locator(CREATE_SUBMIT).click();
}

async function editCardTitle(page, card, newTitle) {
  await card.locator('.card__edit').click();
  await card.locator('.edit-form .input').fill(newTitle);
  await card.locator('.edit-form .save').click();
  await card.locator('.card__title').waitForFunction(
    (el, t) => el.textContent === t,
    newTitle,
    { timeout: 10000 }
  );
}

async function deleteCard(page, card) {
  await card.locator('.card__delete').click();
}


// dragLive is dragTo with the landing measured MID-DRAG: until KW7 the
// stacked-column layout shifts between a pre-drag box and the park point
// (indicator insertion, source slot, re-render pagination), and a park
// point that fell into the gap between columns reads as an abandoned drop
// — zero requests. The landing callback runs with live geometry, then a
// two-pixel nudge re-fires dragover exactly at the park point, so the
// accepted target at release is the one the lane measured.
async function dragLive(page, source, { via = [], landing }) {
  await source.hover();
  await page.mouse.down();
  const src = await source.boundingBox();
  await page.mouse.move(src.x + 16, src.y + 4, { steps: 4 });
  for (const p of via) await page.mouse.move(p.x, p.y, { steps: 6 });
  const p = await landing(page);
  await page.mouse.move(p.x, p.y, { steps: 8 });
  await page.mouse.move(p.x + 2, p.y, { steps: 2 });
  await page.mouse.move(p.x, p.y, { steps: 2 });
  return p;
}

// The upper quarter of a live card — above its midpoint, so the insertion
// line parks BEFORE the card (the dropPosition index the gap between the
// neighbors carries) in every indicator-reflow layout.
async function liveUpperQuarter(page, id) {
  const box = await cardById(page, id).boundingBox();
  return { x: box.x + box.width / 2, y: box.y + box.height * 0.25 };
}

// Just past the live last card's bottom, clamped inside the column — past
// every midpoint, so the line parks after the whole list.
async function liveColumnBottom(page, columnSelector) {
  return page.locator(columnSelector).evaluate((sec) => {
    const ul = sec.querySelector('.column__cards');
    const lastBox = ul.lastElementChild.getBoundingClientRect();
    const secBox = sec.getBoundingClientRect();
    return {
      x: lastBox.x + lastBox.width / 2,
      y: Math.min(lastBox.bottom + 8, secBox.bottom - 4),
    };
  });
}

// refusedDrag attempts a drag on a card that must not start one: the same
// mouse choreography as a real gesture (hover, down, moves, up) — on a card
// the page has blocked (draggable=false since its drop request left, and
// the dragstart guard) it produces no dragstart, hence no drop, no fetch.
// This is the second half of a double-drag attempted while the first
// drop's PATCH is in flight.
async function refusedDrag(page, card) {
  await card.hover();
  await page.mouse.down();
  const box = await card.boundingBox();
  await page.mouse.move(box.x + 24, box.y + 6, { steps: 4 });
  await page.mouse.move(box.x + 60, box.y + 48, { steps: 6 });
  await page.mouse.up();
}




// ————— scenario 1: "Board survives server restart" —————
// A mixed seeded board takes three page mutations (create, edit, drag);
// the server is SIGTERMed and must exit 0; a respawn on the SAME board data
// file serves the exact truth — the fresh page mirrors the pre-shutdown
// board and a fresh GET /board, positions contiguous — and the window still
// works: one more create succeeds after the restart.
async function scenarioRestart(browser) {
  const dbs = tmpPaths('restart');
  const todoAbsent = dbs.board + '.todos-absent';
  run(seedBin, [
    '--db', dbs.board,
    '--titles', [
      'Write the weekly report',  // To Do (this card gets dragged to In Progress)
      'Call the plumber',         // To Do (this middle card gets edited)
      'File the expense claim',   // To Do
      'Draft the launch note',    // In Progress
      'Review the security scan', // In Progress
      'Ship v1.2',                // Done
    ].join(','),
    '--in-progress', '3,4',
    '--done', '5',
  ]);
  const port = await freePort();
  let srv = startServer(port, dbs.board, todoAbsent);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  // TALL viewport: until KW7 the columns stack vertically (no .board layout
  // CSS yet), so In Progress/Done sit near the page's bottom edge — the
  // zone kw5 saw chromium auto-scroll mid-drag, stale-ing every box measured
  // before the scroll. A viewport that fits the whole stacked page keeps
  // every drag point stable.
  const page = await browser.newPage({ viewport: { width: 1280, height: 1400 } });
  await page.goto(`http://127.0.0.1:${port}/`);

  // Mutation 1 — create through the page: appended at To Do's bottom.
  await createCard(page, 'Rotate the audit keys');
  await waitForTitleInTodo(page, 'Rotate the audit keys');

  // Mutation 2 — edit a middle To Do card: text changes, place is kept.
  const viewA = await snapshot(page);
  const edited = viewA.find((c) => c.column === 'To Do').cards[1];
  await editCardTitle(page, cardById(page, edited.id), 'Call the plumber and the electrician');

  // Mutation 3 — drag the top To Do card onto In Progress, landing between
  // its two cards (drop position 1, between cards 0 and 1).
  const viewB = await snapshot(page);
  const dragged = viewB.find((c) => c.column === 'To Do').cards[0];
  const ipCards = viewB.find((c) => c.column === 'In Progress').cards;
  assert.strictEqual(ipCards.length, 2, 'the seeded In Progress column must hold the two cards the drop lands between');
  // Land between the two cards by parking ABOVE the second card's midpoint
  // with LIVE geometry (the layout shifts mid-drag on the stacked page) —
  // the insertion line parks before the second card, carrying position 1.
  const gutter = await page.locator('#column-to-do').evaluate((sec) => {
    const box = sec.getBoundingClientRect();
    return { x: box.right + 8, y: box.top + box.height / 2 };
  });
  await dragLive(page, cardById(page, dragged.id), {
    via: [gutter],
    landing: (p) => liveUpperQuarter(p, ipCards[1].id),
  });
  await page.mouse.up();
  await waitForCardIn(page, dragged.id, 'In Progress');

  // The whole-board truth the three page operations must have produced.
  const expected = viewB.map((col) => {
    if (col.column === 'To Do') return { ...col, cards: col.cards.filter((c) => c.id !== dragged.id) };
    if (col.column === 'In Progress') {
      return { ...col, cards: [col.cards[0], dragged, ...col.cards.slice(1)] };
    }
    return col;
  });
  const lastTruth = await snapshot(page);
  assert.deepStrictEqual(lastTruth, expected,
    'the create + edit + drag mutations did not land as the stated single operations (before the restart, so the restart cannot be blamed)');
  assert.deepStrictEqual(lastTruth, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'before the restart the page does not mirror GET /board');

  // SIGTERM: the process must EXIT — graceful shutdown, exit code 0 — not
  // merely stop answering. This is the restart's stop step.
  const exit = await stopServer(srv);
  assert.deepStrictEqual(exit, { code: 0, signal: null },
    `SIGTERM shutdown must exit the process with code 0, saw ${JSON.stringify(exit)}; stderr: ${srv._stderr()}`);

  // Respawn on the SAME board data file (and the same address): the restart.
  srv = startServer(port, dbs.board, todoAbsent);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);

  // The page shows the exact truth: the same cards with the same texts,
  // columns and identifiers, in the same positions — DOM card-for-card, and
  // in agreement with a fresh GET /board with contiguous positions.
  const page2 = await browser.newPage();
  await page2.goto(`http://127.0.0.1:${port}/`);
  const revived = await snapshot(page2);
  assert.deepStrictEqual(revived, expected,
    'after the restart the page does not show the same cards with the same texts, columns, and positions');
  const revivedJSON = await getJSON(`http://127.0.0.1:${port}/board`);
  assertContiguousPositions(revivedJSON, 'after the restart');
  assert.deepStrictEqual(revived, boardView(revivedJSON),
    'after the restart the page does not mirror a fresh GET /board');

  // The window works: one more operation succeeds against the revived
  // process — a create appends, answers the contract's 201, and the DOM
  // tracks the truth.
  const log2 = instrumentMutations(page2);
  await createCard(page2, 'Renew the SSL certificate');
  await waitForTitleInTodo(page2, 'Renew the SSL certificate');
  assert.strictEqual(log2.for('POST').length, 1, 'the first post-restart create must be exactly one POST');
  assert.deepStrictEqual(log2.statuses('POST'), [201],
    `the first post-restart create must succeed, saw statuses ${log2.statuses('POST')}`);
  const afterCreate = await snapshot(page2);
  assert.deepStrictEqual(
    titlesOf(afterCreate, 'To Do'),
    [...titlesOf(revived, 'To Do'), 'Renew the SSL certificate'],
    'the post-restart create did not append at To Do bottom over the revived board'
  );
  assert.deepStrictEqual(afterCreate, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'after the post-restart create the page does not mirror GET /board');

  await stopServer(srv);
  await page.close();
  await page2.close();
}

// ————— scenario 2: "Migrate existing todos on first start" —————
// Creation order is deliberately interleaved across done states, with two
// rows deleted mid-sequence so the surviving source ids are GAPPED
// (1,2,4,5,7): the imported cards must carry fresh contiguous board
// identifiers instead. The fixture file is hash-pinned before and after
// every start: the reader is mode=ro. A restart never re-imports.
async function scenarioMigration(browser) {
  const dbs = tmpPaths('migration');
  const fixture = path.join(dbs.dir, 'todos.fixture.db');
  const notDoneWant = ['Pay the electricity bill', 'Water the office plants', 'Read the standards doc'];
  const doneWant = ['Renew the passport', 'Clear the email inbox'];
  seedTodoFixture(fixture, [
    { title: 'Pay the electricity bill' },                // id 1 — not done
    { title: 'Renew the passport', done: true },          // id 2 — done
    { title: 'Scratch the old lottery ticket' },          // id 3 — deleted below
    { title: 'Water the office plants' },                 // id 4 — not done
    { title: 'Clear the email inbox', done: true },       // id 5 — done
    { title: 'Defrost the old spreadsheet', done: true }, // id 6 — deleted below
    { title: 'Read the standards doc' },                  // id 7 — not done
  ]);
  deleteTodoFixtureRows(fixture, 'Scratch the old lottery ticket', 'Defrost the old spreadsheet');
  assert.deepStrictEqual(todoFixtureIds(fixture), [1, 2, 4, 5, 7],
    'the fixture must carry gapped surviving ids — the fresh-identifier proof needs them');
  const hashBefore = fileSHA256(fixture);

  const port = await freePort();
  let srv = startServer(port, dbs.board, fixture); // dbs.board: never created
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);

  // The page shows the import: not-done todos in To Do in CREATION ORDER
  // (oldest at top), done todos in Done likewise ordered, In Progress empty
  // with its stated emptiness — done renders purely by column membership.
  const view = await snapshot(page);
  assert.deepStrictEqual(titlesOf(view, 'To Do'), notDoneWant,
    'To Do must hold the not-done todos top-to-bottom in creation order');
  assert.deepStrictEqual(titlesOf(view, 'Done'), doneWant,
    'Done must hold the done todos top-to-bottom in creation order');
  assert.deepStrictEqual(titlesOf(view, 'In Progress'), [],
    'In Progress must hold nothing — the old store never had it');
  assert.ok(view.find((c) => c.column === 'Done').cards.every((c) => c.done),
    'imported done todos must render with the done treatment');
  assert.ok(!view.find((c) => c.column === 'To Do').cards.some((c) => c.done),
    'imported not-done todos must not render with the done treatment');
  const emptyIP = page.locator(EMPTY_STATEMENT('in-progress'));
  assert.strictEqual(await emptyIP.count(), 1, 'the empty imported-into In Progress column must state its emptiness');
  assert.ok(await emptyIP.isVisible(), 'the In Progress emptiness statement must be visible');

  // The page IS the contract, and the contract is contiguous.
  const json = await getJSON(`http://127.0.0.1:${port}/board`);
  assertContiguousPositions(json, 'after the first-start import');
  assert.deepStrictEqual(view, boardView(json),
    'the first page after the import does not mirror GET /board');

  // Fresh identifiers: the board's first five autoincrements, contiguous —
  // the source's GAPPED ids (1,2,4,5,7) were not carried over.
  assert.deepStrictEqual(allCardIds(view), [1, 2, 3, 4, 5],
    'imported card ids must be the contiguous fresh set 1..5 — the gapped source ids must not survive');

  // The board data file now holds the cards.
  assert.strictEqual(boardCardCount(dbs.board), 5,
    'the board data file must hold the imported cards after the first start');

  // The superseded todo data file is byte-identical: the reader is mode=ro.
  assert.strictEqual(fileSHA256(fixture), hashBefore,
    'the first-start import modified the superseded todo data file');

  // Restart with the SAME source still present: the guard holds, the board
  // is exactly as it was — card-for-card, identifiers included. A
  // re-import would show duplicates or fresh ids; equality pins "the import
  // does not run twice".
  const exit = await stopServer(srv);
  assert.strictEqual(exit.code, 0, `SIGTERM before the migration restart must exit 0, saw ${JSON.stringify(exit)}`);
  srv = startServer(port, dbs.board, fixture);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  const page2 = await browser.newPage();
  await page2.goto(`http://127.0.0.1:${port}/`);
  const again = await snapshot(page2);
  assert.deepStrictEqual(again, view,
    'the restarted board differs from the first-start board — the import ran twice');
  assert.strictEqual(again.reduce((n, col) => n + col.cards.length, 0), 5,
    'the restart duplicated cards');
  assert.strictEqual(boardCardCount(dbs.board), 5,
    'the restart grew the card rows in the board data file — a re-import ran');
  assert.strictEqual(fileSHA256(fixture), hashBefore,
    'the restart modified the superseded todo data file');

  await stopServer(srv);
  await page.close();
  await page2.close();
}

// ————— scenario 3: "Repeat activation while an operation is in flight" —————
// Four legs, four seeded boards. Each leg stages its in-flight window with
// delayRoute (the held-URL list proves the request was really in flight),
// fires the repeat activation inside that window, and counts the requests
// the page issued: the ui/12 server-side seams (markup hx-disabled-elt and
// the drag fetch's dragPending guard) are proven HERE at the gesture level
// they deferred to.
async function scenarioRepeatActivation(browser) {
  const todoAbsentSuffix = '.todos-absent';

  // ——— leg a: double-clicked Delete → EXACTLY ONE DELETE request. ———
  // The first click's request is held in flight; the second click of the
  // burst lands while htmx still holds the control disabled
  // (hx-disabled-elt="this"), so it starts nothing: the card is deleted
  // once, by one 200-answered DELETE — no second DELETE, no 404 follow-up.
  {
    const dbs = tmpPaths('inflight-delete');
    run(seedBin, [
      '--db', dbs.board,
      '--titles', 'Write the weekly report,Call the plumber,File the expense claim,Draft the launch note,Review the security scan,Ship v1.2',
      '--in-progress', '3,4',
      '--done', '5',
    ]);
    const port = await freePort();
    const srv = startServer(port, dbs.board, dbs.board + todoAbsentSuffix);
    await waitForHTTP(`http://127.0.0.1:${port}/board`);
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${port}/`);
    const log = instrumentMutations(page);
    const held = await delayRoute(page, '**/ui/cards/*', 'DELETE');

    const before = await snapshot(page);
    const victim = before.find((c) => c.column === 'To Do').cards[1];
    const deleteButton = cardById(page, victim.id).locator('.card__delete');
    log.reset();
    await deleteButton.dblclick();

    await waitUntil(() => held.length === 1, 'the delete request to be held in flight');
    assert.notStrictEqual(await deleteButton.getAttribute('disabled'), null,
      'the double-clicked delete control must carry its in-flight disabled block while the request is unanswered');
    assert.strictEqual(log.for('DELETE', victim.id).length, 1,
      `the second click of the double-click must leave no second DELETE — saw ${JSON.stringify(log.for('DELETE', victim.id))}`);

    await waitForCardGone(page, victim.id);
    assert.strictEqual(log.for('DELETE', victim.id).length, 1,
      'the whole double-click must produce EXACTLY ONE DELETE request');
    assert.ok(log.statuses('DELETE', victim.id).every((s) => s >= 200 && s < 300),
      `the single DELETE must succeed — a leaked second activation would surface a 404, saw ${log.statuses('DELETE', victim.id)}`);
    const after = await snapshot(page);
    assert.deepStrictEqual(after, before.map((col) => ({
      ...col,
      cards: col.cards.filter((c) => c.id !== victim.id),
    })), 'the double-click deleted something beyond the one card, or twice over');
    assert.deepStrictEqual(after, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
      'after the double-clicked delete the page does not mirror GET /board');

    await unrout(page, '**/ui/cards/*');
    await stopServer(srv);
    await page.close();
  }

  // ——— leg b: create submitted by Enter, Enter again mid-flight → SILENT. ———
  // While the POST is held, both triggered controls (input and button)
  // carry their disabled block, so Enter cannot re-submit: exactly ONE
  // POST for the double-Enter gesture, nothing appended twice, no error
  // statement — silent by construction. After the response the input is
  // ready again.
  {
    const dbs = tmpPaths('inflight-create');
    run(seedBin, ['--db', dbs.board, '--titles', 'Write the weekly report,Call the plumber']);
    const port = await freePort();
    const srv = startServer(port, dbs.board, dbs.board + todoAbsentSuffix);
    await waitForHTTP(`http://127.0.0.1:${port}/board`);
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${port}/`);
    const log = instrumentMutations(page);
    const held = await delayRoute(page, '**/ui/cards', 'POST');

    const before = await snapshot(page);
    log.reset();
    await page.locator(CREATE_INPUT).fill('Plan the offsite');
    await page.keyboard.press('Enter');
    await waitUntil(() => held.length === 1, 'the create POST to be held in flight');

    assert.notStrictEqual(await page.locator(CREATE_INPUT).getAttribute('disabled'), null,
      'the create input must be disabled while its POST is in flight (a live input could re-submit on Enter)');
    assert.notStrictEqual(await page.locator(CREATE_SUBMIT).getAttribute('disabled'), null,
      'the create button must be disabled while its POST is in flight');
    await page.keyboard.press('Enter'); // the repeat activation: in flight, must be silent
    await page.waitForTimeout(300);
    assert.strictEqual(log.for('POST').length, 1,
      `an Enter pressed while the create POST is in flight must not submit a second POST, saw ${JSON.stringify(log.for('POST'))}`);
    assert.strictEqual(await page.locator('#create-error').count(), 0,
      'the blocked Enter re-submit must be silent — no error statement appears');

    await waitForTitleInTodo(page, 'Plan the offsite');
    assert.strictEqual(log.for('POST').length, 1,
      'the whole double-Enter gesture must produce EXACTLY ONE POST');
    assert.ok(log.statuses('POST').every((s) => s >= 200 && s < 300),
      `the single POST must succeed, saw ${log.statuses('POST')}`);
    assert.strictEqual(await page.locator(CREATE_INPUT).getAttribute('disabled'), null,
      'the create input must be ready again once the response arrives');
    const after = await snapshot(page);
    assert.deepStrictEqual(titlesOf(after, 'To Do'), [...titlesOf(before, 'To Do'), 'Plan the offsite'],
      'the double-Enter gesture must append the card exactly once');
    assert.deepStrictEqual(after, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
      'after the in-flight create the page does not mirror GET /board');

    await unrout(page, '**/ui/cards');
    await stopServer(srv);
    await page.close();
  }

  // ——— leg c: double-drag → ONE PATCH, controls ready for a third ———
  // The first drop's PATCH is held in flight; the card — the triggered
  // control of the move — goes inactive the moment the request leaves
  // (draggable=false, dragstart guard). A second drag attempted inside that
  // window starts nothing: ONE PATCH for the attempted pair. After the
  // response the card is draggable again and a THIRD, legitimate drag
  // succeeds — the controls-usable-again half of the ledger contract.
  {
    const dbs = tmpPaths('inflight-drag');
    run(seedBin, [
      '--db', dbs.board,
      '--titles', 'Write the weekly report,File the expense claim,Draft the launch note,Review the security scan',
      '--in-progress', '2,3',
    ]);
    const port = await freePort();
    const srv = startServer(port, dbs.board, dbs.board + todoAbsentSuffix);
    await waitForHTTP(`http://127.0.0.1:${port}/board`);
    // TALL viewport — same auto-scroll defense as scenario 1's drag page.
    const page = await browser.newPage({ viewport: { width: 1280, height: 1400 } });
    await page.goto(`http://127.0.0.1:${port}/`);
    const log = instrumentMutations(page);
    const held = await delayRoute(page, '**/ui/cards/*/move', 'PATCH');

    const before = await snapshot(page);
    const dragged = before.find((c) => c.column === 'To Do').cards[0];
    const ipCards = before.find((c) => c.column === 'In Progress').cards;
    const gutter = await page.locator('#column-to-do').evaluate((sec) => {
      const box = sec.getBoundingClientRect();
      return { x: box.right + 8, y: box.top + box.height / 2 };
    });
    log.reset();
    await dragLive(page, cardById(page, dragged.id), {
      via: [gutter],
      landing: (p) => liveUpperQuarter(p, ipCards[1].id),
    });
    await page.mouse.up();

    await waitUntil(() => held.length === 1, 'the move PATCH to be held in flight');
    assert.strictEqual(await cardById(page, dragged.id).getAttribute('draggable'), 'false',
      'the dragged card must lose draggable the moment its drop request leaves (card ui/12)');
    await refusedDrag(page, cardById(page, dragged.id)); // the second drag, attempted mid-flight
    await page.waitForTimeout(200);
    assert.strictEqual(log.for('PATCH', dragged.id).length, 1,
      `a second drag attempted while the drop's PATCH is in flight must start no second PATCH — saw ${JSON.stringify(log.for('PATCH', dragged.id))}`);

    await waitForCardIn(page, dragged.id, 'In Progress');
    assert.strictEqual(log.for('PATCH', dragged.id).length, 1,
      'the double-drag attempt must produce EXACTLY ONE PATCH');
    assert.deepStrictEqual(log.statuses('PATCH', dragged.id), [200],
      'the one PATCH of the double-drag pair must succeed');

    // Response arrived, truth swapped in: a THIRD, legitimate drag succeeds
    // — exactly one PATCH for it, and the swapped card renders ready.
    // Landing: just past the live last To Do card's bottom, clamped inside
    // the section — past every midpoint in any indicator layout, so the
    // line parks after the whole list and the column stays the drop target.
    log.reset();
    await dragLive(page, cardById(page, dragged.id), {
      landing: (p) => liveColumnBottom(p, '#column-to-do'),
    });
    await page.mouse.up();
    await waitForCardIn(page, dragged.id, 'To Do');
    assert.strictEqual(log.for('PATCH', dragged.id).length, 1,
      'after the response the drag control must work again — the third gesture is exactly one PATCH');
    assert.deepStrictEqual(log.statuses('PATCH', dragged.id), [200], 'the third drag must succeed');
    assert.strictEqual(await cardById(page, dragged.id).getAttribute('draggable'), 'true',
      'the swapped card renders draggable — the ready state');
    const after = await snapshot(page);
    assert.deepStrictEqual(after, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
      'after the third drag the page does not mirror GET /board');

    await unrout(page, '**/ui/cards/*/move');
    await stopServer(srv);
    await page.close();
  }

  // ——— leg d: 4xx rejection restores the controls. ———
  // A stale page deletes a card removed out-of-band: one DELETE, answered
  // 404, stated via the shared missing-card surface. htmx restores every
  // disabled control on the settled 4xx path too (its onload leg runs for
  // non-2xx as well), so after the swap NOTHING on the page sits disabled,
  // and the next legitimate operation succeeds: the third-op-usable proof
  // on the rejection leg.
  {
    const dbs = tmpPaths('inflight-4xx');
    run(seedBin, [
      '--db', dbs.board,
      '--titles', 'Write the weekly report,File the expense claim,Draft the launch note,Ship v1.2',
      '--in-progress', '2',
      '--done', '3',
    ]);
    const port = await freePort();
    const srv = startServer(port, dbs.board, dbs.board + todoAbsentSuffix);
    await waitForHTTP(`http://127.0.0.1:${port}/board`);
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${port}/`);
    const log = instrumentMutations(page);

    const before = await snapshot(page);
    const stale = before.find((c) => c.column === 'Done').cards[0];
    deleteCardOutOfBand(dbs.board, stale.id);
    assert.strictEqual(await cardById(page, stale.id).count(), 1,
      'the page must still show the card removed behind its back — the stale Given');

    log.reset();
    await deleteCard(page, cardById(page, stale.id));
    const banner = page.locator(MISSING_BANNER);
    await banner.waitFor({ state: 'visible' });
    assert.match((await banner.textContent()).trim(), /no such card/,
      'the stale delete must state the contract reason');
    assert.deepStrictEqual(log.statuses('DELETE', stale.id), [404],
      `the stale delete is the 4xx rejection leg — one DELETE answered 404, saw ${log.statuses('DELETE', stale.id)}`);

    // Settled (response arrived, failure surface swapped): no control on
    // the page may still sit in its in-flight disabled state.
    const blocked = await page.evaluate(() =>
      document.querySelectorAll('li.card .card__delete[disabled], #create-form [disabled]').length
    );
    assert.strictEqual(blocked, 0,
      'a 4xx rejection must restore the triggered control — htmx settles 4xx like any response');

    // Controls usable again: the next legitimate delete succeeds outright.
    const other = (await snapshot(page)).find((c) => c.column === 'To Do').cards[0];
    log.reset();
    await deleteCard(page, cardById(page, other.id));
    await waitForCardGone(page, other.id);
    assert.strictEqual(log.for('DELETE', other.id).length, 1, 'the post-4xx delete must be exactly one DELETE');
    assert.deepStrictEqual(log.statuses('DELETE', other.id), [200], 'the post-4xx delete must succeed');
    const after = await snapshot(page);
    assert.ok(!after.some((col) => col.cards.some((c) => c.id === stale.id || c.id === other.id)),
      'both deleted cards must be absent from the restored page');
    assert.deepStrictEqual(after, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
      'after the post-4xx delete the page does not mirror GET /board');

    await stopServer(srv);
    await page.close();
  }
}

// ————— scenario 4: poisoned migration fails loud (process-level) —————
// The same loud-fail boundary cmd/todo/interrupted_import_test.go proves at
// the Go level, witnessed once inside the lane idiom: a fixture row the
// board's own text rule refuses makes the composed process FAIL startup —
// non-zero exit, stated stderr — and what it leaves is at most an empty
// board file, never a half board. No browser, no page: pure process check.
async function scenarioPoisonedMigration() {
  const dbs = tmpPaths('poisoned');
  const fixture = path.join(dbs.dir, 'todos.poison.db');
  // Good material first, then the poison: a whitespace-only title, which
  // the board's text rule trims to blank — the old app itself would not
  // have stored it (raw insert mirrors migration_test.go's poisoned rows).
  seedTodoFixture(fixture, [
    { title: 'Pay the electricity bill' },
    { title: 'Renew the passport', done: true },
    { title: '   ' },
  ]);
  const hashBefore = fileSHA256(fixture);

  const port = await freePort();
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--board-db', dbs.board, '--todo-db', fixture], {
    stdio: ['ignore', 'ignore', 'pipe'],
  });
  let stderr = '';
  proc.stderr.on('data', (d) => (stderr += d));
  const exited = await Promise.race([
    new Promise((resolve) => proc.on('exit', (code, signal) => resolve({ code, signal }))),
    new Promise((resolve) => setTimeout(() => resolve('TIMEOUT'), 15000)),
  ]);
  assert.notStrictEqual(exited, 'TIMEOUT',
    'a poisoned import must fail startup, not serve — the process is still running');
  assert.notStrictEqual(exited.code, 0,
    `the poisoned migration must exit non-zero, saw ${JSON.stringify(exited)}`);
  assert.match(stderr, /import todos/,
    `startup must state the todo import as the failure reason on stderr, saw: ${stderr}`);

  // No half board: the refused import executed no statement, so the board
  // file is absent or holds zero cards — a clean board the next start can
  // complete against. And the poisoned source itself stays untouched.
  if (fs.existsSync(dbs.board)) {
    assert.strictEqual(boardCardCount(dbs.board), 0,
      'a failed migration must leave zero cards in the board file — never a half board');
  }
  assert.strictEqual(fileSHA256(fixture), hashBefore,
    'even a failed migration must not touch the superseded todo data file');
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const failures = [];

  try {
    await scenarioRestart(browser);
    console.log('scenario 1 (board survives server restart — create/edit/drag, SIGTERM exit 0, respawn truth + one more op): OK');

    await scenarioMigration(browser);
    console.log('scenario 2 (migrate existing todos on first start — creation-order mapping, fresh ids, source untouched, restart never re-imports): OK');

    await scenarioRepeatActivation(browser);
    console.log('scenario 3 (repeat activation in flight — one DELETE per double-click, one silent POST per double-Enter, one PATCH per double-drag + third op works, controls restored after 404): OK');

    await scenarioPoisonedMigration();
    console.log('scenario 4 (poisoned migration fails loud — exit non-zero, stated stderr, zero-card board — mirrors cmd/todo/interrupted_import_test.go): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW6 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
