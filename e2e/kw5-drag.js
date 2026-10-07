// KW5 e2e — drag end-to-end in the browser, re-executing the parent scenarios
// "Drag card between columns", "Drag reorder within a column" and "Done is
// column membership", plus the MOVE leg of "Operation on missing card" — the
// leg that completes that scenario fully (its edit and delete legs went green
// in the KW3 and KW4 lanes; workplan_kanban_application.md, wave-end contract
// workplans/dependencies_kanban.md §KW5).
//
// The surfaces are cards ui/08–10 (render.go's shell script): every card is
// draggable="true", an accepted drop issues EXACTLY ONE PATCH from the page's
// single fetch site to /ui/cards/{id}/move carrying the target column and the
// drop position, the answer swaps the fresh board into #board-area (never a
// reload, never a local guess), and the drop indicator (li.drop-indicator)
// marks the landing gap BEFORE release. An abandoned drag — released outside
// any column — never reaches that fetch site (dragover outside a .column does
// not preventDefault, so drop never fires): zero requests by construction.
// The stale drop rides the shared stale-failure surface (#missing-card,
// stale.go) over the re-rendered truth.
//
// Instrumentation: a page.on('request') listener records every PATCH the
// browser issues, with its body. Per accepted gesture the lane asserts the
// count is EXACTLY ONE (one-request-per-drop) and per abandoned gesture that
// it is ZERO (card ui/10) — the counters are the proof, not the DOM aftermath
// alone. The server-side contract call count (ui → api PATCH) is owned by
// ui/move_test.go's recorder; from the browser the visible one-request rule
// is the client's fetch, which is what this lane counts.
//
// DnD mechanics: chromium needs real event sequences, so the gestures here
// are mouse choreography — hover, mouse.down, mouse.moves (Playwright's
// chromium drag interception converts these into dragstart/dragenter/dragover,
// and the release into drop/dragend). Mid-gesture the lane hovers the landing
// point, then inspects the indicator BEFORE mouse.up (best-effort witness,
// reported honestly in the run output).
//
// Stale-drag staging (scenario 4): two browser contexts on the SAME server.
// Context A deletes the card through its own Delete control — an in-band
// deletion by the other session, the workplan risk "a stale page drags a card
// that another action deleted" verbatim. Context B's page never refreshed, so
// it stays genuinely stale and its stale card is still draggable — the honest
// staging for a drag whose reference died behind the page's back.
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

// The composition root opens only the board data file. Point the flag at a
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

// The shared stale-failure banner the move verb rides for a 404 (stale.go).
const MISSING_BANNER = '#missing-card';
const anchor = (title) => title.toLowerCase().replace(/ /g, '-');

// The whole board as the DOM shows it: per fixed column, its cards
// top-to-bottom as {title, id, done}. DOM order IS the assertion surface for
// every drag outcome — a drop must move exactly one entry of this structure
// to exactly one index and leave every other entry identical.
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
// GET /board can be compared directly.
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

// The contract's position invariant: within every column the cards carry
// contiguous positions 0..n-1 in array order. Returns {title, id, position}
// rows per column so DOM order can be tied to stored order card-for-card.
function assertContiguousPositions(json, where) {
  const rows = {};
  for (const col of json.columns) {
    col.cards.forEach((card, i) => {
      assert.strictEqual(card.position, i,
        `${where}: ${col.title} card ${card.id} carries position ${card.position}, not the contiguous index ${i}`);
    });
    rows[col.title] = col.cards.map((card) => ({ title: card.title, id: card.id, position: card.position }));
  }
  return rows;
}

// A card in the DOM by its identifier, anywhere on the board.
function cardById(page, id) {
  return page.locator(`li.card[data-card="${id}"]`);
}

// ————— instrumentation: the one-request / zero-request proof —————
//
// One listener per page records every PATCH the browser issues, with its
// body. The lane resets the log before each gesture and asserts the count
// after it: exactly one PATCH per accepted drop (body: column + position),
// zero PATCHes per abandonment. GETs (reload, GET /board) never count —
// only the mutation verb the gesture is supposed to produce.
function instrumentPatches(page) {
  const log = {
    patches: [],
    reset() {
      this.patches.length = 0;
    },
  };
  page.on('request', (req) => {
    if (req.method() === 'PATCH') log.patches.push({ url: req.url(), body: req.postData() });
  });
  return log;
}

// The gesture's request accounting for an ACCEPTED drop: exactly one PATCH,
// addressed at the dragged card's move endpoint, body parses — returned for
// the scenario's own column/position assertion.
function assertOneMoveRequest(log, id, where) {
  assert.strictEqual(log.patches.length, 1,
    `${where}: expected EXACTLY ONE PATCH per accepted drop, saw ${log.patches.length}: ${JSON.stringify(log.patches)}`);
  assert.ok(log.patches[0].url.endsWith(`/ui/cards/${id}/move`),
    `${where}: the drop's PATCH addressed ${log.patches[0].url}, want the move endpoint for card ${id}`);
  let body;
  try {
    body = JSON.parse(log.patches[0].body);
  } catch (err) {
    assert.fail(`${where}: the drop's PATCH body ${log.patches[0].body} does not parse: ${err}`);
  }
  return body;
}

// The gesture's request accounting for an ABANDONED drag: nothing left the
// page at all.
function assertNoRequests(log, where) {
  assert.deepStrictEqual(log.patches, [],
    `${where}: an abandoned drag must issue ZERO PATCH requests, saw ${JSON.stringify(log.patches)}`);
}

// ————— gestures: real HTML5 DnD through mouse choreography —————
//
// hover + mouse.down on the card, then mouse.moves (chromium's drag
// interception, driven by Playwright's CDP, turns these into
// dragstart/dragenter/dragover at each traversed point), then release at
// `target`. `via` points are traversed first — through the gutter to exercise
// dragover rejection mid-drag, or over a column to accept there. The final
// move parks the last dragover at `target`, which is where the insertion line
// sits when mouse.up turns into drop (inside a column) or nothing (outside).
// `beforeRelease` runs AFTER the final hover and BEFORE mouse.up: the only
// window in which the mid-gesture indicator exists.
async function dragTo(page, source, target, { via = [], beforeRelease } = {}) {
  await source.hover();
  await page.mouse.down();
  const src = await source.boundingBox();
  // A short lead-in past the browser's drag-start threshold so the drag is
  // committed before the traversed points are interpreted as dragover.
  await page.mouse.move(src.x + 16, src.y + 4, { steps: 4 });
  for (const p of via) await page.mouse.move(p.x, p.y, { steps: 6 });
  await page.mouse.move(target.x, target.y, { steps: 10 });
  if (beforeRelease) await beforeRelease(page);
  await page.mouse.up();
}

// Wait until the swap landed the card in the named column — the fragment
// arriving is what "appears in <column> without a reload" observably is.
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

// Mid-gesture, best-effort indicator witness: where the insertion line sits
// (its neighbors' card ids) and whether it is actually painted. Witnessed,
// not asserted — chromium's drag interception timing can miss the window;
// the lane reports the witness honestly, and the request body's position is
// the hard proof that the SAME gap the indicator marks rides the request
// (both are the one dropPosition count in render.go).
async function readDropIndicator(page, columnTitle) {
  try {
    const line = page.locator(`#column-${anchor(columnTitle)} li.drop-indicator`);
    await line.waitFor({ state: 'attached', timeout: 1500 });
    return await line.evaluate((el) => ({
      visible: el.getBoundingClientRect().height > 0,
      between: [el.previousElementSibling, el.nextElementSibling].map(
        (n) => (n && n.classList.contains('card') ? n.dataset.card : null)
      ),
    }));
  } catch (err) {
    return { seen: false, why: String(err && err.message) };
  }
}

// Zero separate done controls, anywhere: no checkbox/radio/switch input, no
// button/submit/link whose text mentions done or toggling (a column header is
// a heading, not a control — J6: column membership IS the done state, so any
// done control would be a second source of truth).
async function assertNoDoneControl(page, where) {
  const hits = await page.evaluate(() => {
    const found = [];
    document
      .querySelectorAll('input[type="checkbox"], input[type="radio"], [role="checkbox"], [role="switch"]')
      .forEach((el) => found.push(el.outerHTML));
    document.querySelectorAll('button, input[type="submit"], a').forEach((el) => {
      if (/\bdone\b|\btoggle\b/i.test(el.textContent || el.value || '')) found.push(el.outerHTML);
    });
    return found;
  });
  assert.deepStrictEqual(hits, [], `${where}: no separate done control may exist anywhere on the page`);
}

// Drag chrome must not survive a finished gesture (accepted or abandoned):
// the slot, line, chip and target ring are all drag-only.
async function assertNoDragChrome(page, where) {
  const residue = await page.evaluate(() =>
    document.querySelectorAll('.drop-indicator, .card--source, .drag-chip, .column--drop-target').length
  );
  assert.strictEqual(residue, 0, `${where}: drag chrome survived the finished gesture`);
}

// A point at fraction `fy` of an element's height, horizontally centered.
async function pointOn(locator, fy) {
  const box = await locator.boundingBox();
  return { x: box.x + box.width / 2, y: box.y + box.height * fy };
}

// The center of an element as a plain point (for via / release points).
async function centerOf(locator) {
  const box = await locator.boundingBox();
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const failures = [];
  const witnesses = [];

  try {
    // --- Scenario 1: "Drag card between columns" — a seeded board puts
    // three cards in To Do and two in In Progress. Dragging the MIDDLE To Do
    // card onto In Progress between its two cards moves EXACTLY ONE PATCH
    // (body: the target column + the drop index), the card lands at the drop
    // index (DOM order AND a fresh GET /board agree), the source column
    // closes its gap, the text and identifier ride along unchanged, and the
    // page never reloads. Mid-gesture the insertion line marks the landing
    // gap before release (best-effort witness).
    const dbs1 = tmpPaths('drag-between');
    run(seedBin, [
      '--db', dbs1.board,
      '--titles', [
        'Write the weekly report',  // To Do
        'Call the plumber',         // To Do (the middle card — this scenario drags it)
        'File the expense claim',   // To Do
        'Draft the launch note',    // In Progress (the drop lands BELOW this one...)
        'Review the security scan', // In Progress (...and ABOVE this one)
        'Archive last quarter',     // Done
      ].join(','),
      '--in-progress', '3,4',
      '--done', '5',
    ]);
    const port1 = await freePort();
    const srv1 = startServer(port1, dbs1.board);
    await waitForHTTP(`http://127.0.0.1:${port1}/board`);
    const page1 = await browser.newPage();
    await page1.goto(`http://127.0.0.1:${port1}/`);
    const log1 = instrumentPatches(page1);

    await page1.evaluate(() => { window.__kw5NoReloadMarker = 'alive'; });
    const before1 = await snapshot(page1);
    const dragged1 = before1.find((c) => c.column === 'To Do').cards[1];
    const ipCards = before1.find((c) => c.column === 'In Progress').cards;
    assert.ok(before1.find((c) => c.column === 'To Do').cards.length >= 3,
      'the seeded To Do column must hold a card with neighbors above and below');
    assert.strictEqual(ipCards.length, 2,
      'the seeded In Progress column must hold the two cards the drop lands between');
    assert.strictEqual(await cardById(page1, dragged1.id).getAttribute('draggable'), 'true',
      'the card itself is the drag source — it must be draggable');

    // Land between the two In Progress cards: the release point sits below
    // the first card's midpoint and above the second's, so the last dragover
    // parks the indicator in exactly that gap and dropPosition counts one
    // card ahead of the line — the contract's index for "between the two".
    const landing = await pointOn(page1.locator(`li.card[data-card="${ipCards[0].id}"]`), 0.75);
    const gutter = await page1.locator('#column-to-do').evaluate((sec) => {
      const box = sec.getBoundingClientRect();
      return { x: box.right + 8, y: box.top + box.height / 2 }; // gutter left of In Progress
    });

    log1.reset();
    let indicator1;
    await dragTo(page1, cardById(page1, dragged1.id), landing, {
      via: [gutter],
      beforeRelease: async (page) => { indicator1 = await readDropIndicator(page, 'In Progress'); },
    });
    await waitForCardIn(page1, dragged1.id, 'In Progress');
    witnesses.push(`scenario 1 mid-gesture drop indicator: ${JSON.stringify(indicator1)}`);

    // The one-request-per-drop proof, counted at the browser boundary.
    const body1 = assertOneMoveRequest(log1, dragged1.id, 'the between-columns drop');
    assert.deepStrictEqual(body1, { column: 'In Progress', position: 1 },
      'the between-columns drop must carry the target column and the index between the two cards');

    assert.strictEqual(await page1.evaluate(() => window.__kw5NoReloadMarker), 'alive',
      'the drop must not reload the page — the landing is a fragment swap onto the existing render');
    const after1 = await snapshot(page1);
    const expected1 = before1.map((col) => {
      if (col.column === 'To Do') return { ...col, cards: col.cards.filter((c) => c.id !== dragged1.id) };
      if (col.column === 'In Progress') {
        const rest = col.cards.filter((c) => c.id !== dragged1.id);
        return { ...col, cards: [rest[0], dragged1, ...rest.slice(1)] };
      }
      return col;
    });
    assert.deepStrictEqual(after1, expected1,
      'the drag moved something beyond placing the one card at the drop index (source gap not closed, or another card moved)');
    const json1 = await getJSON(`http://127.0.0.1:${port1}/board`);
    const rows1 = assertContiguousPositions(json1, 'after the between-columns drop');
    assert.deepStrictEqual(rows1['In Progress'],
      expected1.find((c) => c.column === 'In Progress').cards.map((card, i) => ({ title: card.title, id: card.id, position: i })),
      'the landed In Progress order in the DOM does not agree with GET /board order and positions');
    assert.deepStrictEqual(after1, boardView(json1), 'after the between-columns drop the page does not mirror GET /board');
    await assertNoDragChrome(page1, 'after the between-columns drop');

    await stopServer(srv1);
    await page1.close();
    console.log('scenario 1 (drag card between columns — one PATCH, lands at drop index): OK');

    // --- Scenario 2: "Drag reorder within a column" — a column of four
    // cards; dragging the BOTTOM card to the top of the SAME column is one
    // PATCH (position 0 — the contract's index-after-removal, which the
    // client's dropPosition already computes that way), the column lists the
    // new order immediately, and the order survives an ACTUAL page.reload()
    // with contiguous stored positions agreeing card-for-card.
    const dbs2 = tmpPaths('drag-reorder');
    run(seedBin, [
      '--db', dbs2.board,
      '--titles', 'Write the weekly report,Call the plumber,File the expense claim,Tidy the print stack',
    ]);
    const port2 = await freePort();
    const srv2 = startServer(port2, dbs2.board);
    await waitForHTTP(`http://127.0.0.1:${port2}/board`);
    const page2 = await browser.newPage();
    await page2.goto(`http://127.0.0.1:${port2}/`);
    const log2 = instrumentPatches(page2);

    const before2 = await snapshot(page2);
    const todo2 = before2.find((c) => c.column === 'To Do');
    assert.ok(todo2.cards.length >= 3, 'the scenario needs a column of at least three cards');
    const bottom = todo2.cards[todo2.cards.length - 1];
    // Landing point for the TOP of the column, retargeted at the KW7
    // side-by-side layout (harness-side only; the behavior tested is
    // unchanged). The pre-KW7 pointOn(firstCard, 0.25) sat high enough on
    // the card that the insertion line's ~14px reflow pushed the point out
    // of the shifted card into the list band. The narrower columns wrap
    // titles, making cards tall enough that 0.25 now stays INSIDE the card
    // the line insertion moves — chromium then closes the gesture with a
    // dragleave as its last event and fires no drop. Mirror of 2b's clamp
    // toward the other edge: park just ABOVE the live first card, inside
    // the section. clientY is below the midpoint of nothing — the line
    // parks before the first card (position 0) — and no reflow ever moves
    // an element under the release point.
    const topPoint = await page2.locator('#column-to-do').evaluate((sec) => {
      const ul = sec.querySelector('.column__cards');
      const firstBox = ul.firstElementChild.getBoundingClientRect();
      const secBox = sec.getBoundingClientRect();
      return { x: firstBox.x + firstBox.width / 2, y: Math.max(firstBox.top - 8, secBox.top + 4) };
    });

    log2.reset();
    await dragTo(page2, cardById(page2, bottom.id), topPoint);
    await page2.waitForFunction(
      (cardId) => {
        const ul = document.querySelector('#column-to-do .column__cards');
        return ul && ul.firstElementChild && ul.firstElementChild.dataset.card === String(cardId);
      },
      bottom.id,
      { timeout: 10000 }
    );

    const body2 = assertOneMoveRequest(log2, bottom.id, 'the same-column drop');
    assert.deepStrictEqual(body2, { column: 'To Do', position: 0 },
      'dragging the bottom card to the top must carry position 0 of the same column');
    const after2 = await snapshot(page2);
    const expected2 = before2.map((col) =>
      col.column === 'To Do'
        ? { ...col, cards: [bottom, ...col.cards.filter((c) => c.id !== bottom.id)] }
        : col
    );
    assert.deepStrictEqual(after2, expected2,
      'the reorder changed something beyond placing the bottom card at the top (the others lost their relative order)');

    // The order survives an ACTUAL reload — position is stored state, not a
    // DOM arrangement — and the contract carries it contiguously.
    await page2.reload();
    const reloaded2 = await snapshot(page2);
    assert.deepStrictEqual(reloaded2, expected2, 'after reload the reordered column shows a different order');
    const rows2 = assertContiguousPositions(await getJSON(`http://127.0.0.1:${port2}/board`), 'after the reorder reload');
    assert.deepStrictEqual(rows2['To Do'],
      expected2.find((c) => c.column === 'To Do').cards.map((card, i) => ({ title: card.title, id: card.id, position: i })),
      'the reloaded DOM order does not agree with the stored contiguous positions');

    await stopServer(srv2);
    await page2.close();
    console.log('scenario 2 (drag reorder within a column — one PATCH, survives reload): OK');

    // --- Scenario 2b: the DOWNWARD leg of "Drag reorder within a column" —
    // the branch scenario 2's upward drag cannot reach. When the dragged
    // card's slot sits ABOVE the landing gap, dropPosition must count PAST
    // it (skipping the dragged card among the nodes ahead of the line) to
    // carry the contract's index-after-removal (board.Move removes before
    // inserting). A three-card column: dragging the TOP card to the BOTTOM
    // (the last dragover parks the line below the last card) is EXACTLY ONE
    // PATCH with position 2 — the raw count ahead of the line is 3, so 2 is
    // only reachable by skipping the dragged card's own slot — and the
    // column lands [B, C, A]. The reload proof stays scenario 2's; this leg
    // adds the downward geometry and the counting-past assertion.
    const dbs2b = tmpPaths('drag-reorder-down');
    run(seedBin, [
      '--db', dbs2b.board,
      '--titles', 'Call the plumber,File the expense claim,Write the weekly report',
    ]);
    const port2b = await freePort();
    const srv2b = startServer(port2b, dbs2b.board);
    await waitForHTTP(`http://127.0.0.1:${port2b}/board`);
    const page2b = await browser.newPage();
    await page2b.goto(`http://127.0.0.1:${port2b}/`);
    const log2b = instrumentPatches(page2b);

    const before2b = await snapshot(page2b);
    const todo2b = before2b.find((c) => c.column === 'To Do');
    assert.strictEqual(todo2b.cards.length, 3,
      'the downward leg needs a three-card column so index-after-removal (2) separates from the raw drop count (3)');
    const head = todo2b.cards[0];
    const middle = todo2b.cards[1];
    const tail = todo2b.cards[2];

    // Release BELOW the last card, inside the column — the mid-viewport card
    // geometry (never a strip near the page's bottom edge, where the kw5 lane
    // saw chromium auto-scroll mid-drag) extended past the list: once the
    // indicator sits anywhere above the last card it reflows that card ~14px
    // down (2px line + 6px margins), so a fixed fy on the card can hold a
    // stable above-center equilibrium. A point past the card's bottom edge
    // but clamped inside the section puts clientY past its midpoint in EVERY
    // indicator layout, parking the line after the whole list.
    const belowPoint = await page2b.locator('#column-to-do').evaluate((sec) => {
      const ul = sec.querySelector('.column__cards');
      const lastBox = ul.lastElementChild.getBoundingClientRect();
      const secBox = sec.getBoundingClientRect();
      return {
        x: lastBox.x + lastBox.width / 2,
        y: Math.min(lastBox.bottom + 8, secBox.bottom - 4),
      };
    });
    log2b.reset();
    let indicator2b;
    await dragTo(page2b, cardById(page2b, head.id), belowPoint, {
      beforeRelease: async (page) => { indicator2b = await readDropIndicator(page, 'To Do'); },
    });
    witnesses.push(`scenario 2b mid-gesture drop indicator: ${JSON.stringify(indicator2b)}`);
    await page2b.waitForFunction(
      (cardId) => {
        const ul = document.querySelector('#column-to-do .column__cards');
        return ul && ul.firstElementChild && ul.firstElementChild.dataset.card === String(cardId);
      },
      middle.id,
      { timeout: 10000 }
    );

    // The counting-past-own-slot proof lives in this body: three rendered
    // cards sit ahead of the landing line, one of them is the dragged card's
    // own slot, and the payload must say 2 — position 3 would land the card
    // past the bottom of the list after the removal.
    const body2b = assertOneMoveRequest(log2b, head.id, 'the same-column downward drop');
    assert.deepStrictEqual(body2b, { column: 'To Do', position: 2 },
      'dragging the top card to the bottom must count past its own slot — position 2 (index-after-removal), not 3');

    const after2b = await snapshot(page2b);
    const expected2b = before2b.map((col) =>
      col.column === 'To Do'
        ? { ...col, cards: [...col.cards.filter((c) => c.id !== head.id), head] }
        : col
    );
    assert.deepStrictEqual(after2b, expected2b,
      'the downward reorder changed something beyond moving the top card to the bottom (the others lost their relative order)');
    const json2b = await getJSON(`http://127.0.0.1:${port2b}/board`);
    const rows2b = assertContiguousPositions(json2b, 'after the same-column downward drop');
    assert.deepStrictEqual(rows2b['To Do'],
      expected2b.find((c) => c.column === 'To Do').cards.map((card, i) => ({ title: card.title, id: card.id, position: i })),
      'the downward-reordered DOM order does not agree with GET /board order and positions');
    assert.deepStrictEqual(after2b, boardView(json2b),
      'after the same-column downward drop the page does not mirror GET /board');
    await assertNoDragChrome(page2b, 'after the same-column downward drop');

    await stopServer(srv2b);
    await page2b.close();
    console.log('scenario 2b (drag reorder downward — one PATCH, position 2 past own slot, lands [B,C,A]): OK');

    // --- Scenario 3: "Done is column membership" — dragging the In Progress
    // card into Done renders it with the done treatment (a class off column
    // membership — the contract carries no done field), and dragging it back
    // out clears the treatment. Nowhere on the page — before, during, or
    // after any of this — does a separate done control exist: zero
    // checkbox/radio/switch markup, no done/toggle button or link. The
    // edit-a-Done-card cross-check (kw3's coverage) follows as its own
    // block, on a card the drag placed in Done.
    const dbs3 = tmpPaths('done-membership');
    run(seedBin, [
      '--db', dbs3.board,
      '--titles', 'Tune the database indexes,Prepare the demo,Rewrite the onboarding doc',
      '--in-progress', '1', // 'Prepare the demo' sits in In Progress
    ]);
    const port3 = await freePort();
    const srv3 = startServer(port3, dbs3.board);
    await waitForHTTP(`http://127.0.0.1:${port3}/board`);
    const page3 = await browser.newPage();
    await page3.goto(`http://127.0.0.1:${port3}/`);
    const log3 = instrumentPatches(page3);

    await page3.evaluate(() => { window.__kw5NoReloadMarker = 'alive'; });
    const before3 = await snapshot(page3);
    const demo = before3.find((c) => c.column === 'In Progress').cards[0];
    await assertNoDoneControl(page3, 'on the freshly rendered board');

    // Drag INTO Done — Done is empty, so the release point is its stated
    // emptiness and the contract index is 0.
    log3.reset();
    await dragTo(page3, cardById(page3, demo.id), await pointOn(page3.locator('#column-done p.column__empty'), 0.5));
    await waitForCardIn(page3, demo.id, 'Done');
    const body3 = assertOneMoveRequest(log3, demo.id, 'the drop into Done');
    assert.deepStrictEqual(body3, { column: 'Done', position: 0 },
      'the drag into Done must carry Done as the target column');
    const inDone = await snapshot(page3);
    const doneCard = inDone.find((c) => c.column === 'Done').cards[0];
    assert.strictEqual(doneCard.id, demo.id, 'the dragged card must be the Done column card');
    assert.strictEqual(doneCard.done, true, 'a card in the Done column must render with the done treatment');
    assert.strictEqual(await page3.evaluate(() => window.__kw5NoReloadMarker), 'alive',
      'the drag into Done must stay a fragment swap, not a page reload');
    await assertNoDoneControl(page3, 'while the card sits in Done');

    // Drag OUT of Done back into To Do — the treatment IS membership, so it
    // clears with the column, and no control was ever involved.
    log3.reset();
    await dragTo(page3, cardById(page3, demo.id), await pointOn(page3.locator('#column-to-do li.card').first(), 0.75), {
      via: [await centerOf(page3.locator('#column-in-progress'))],
    });
    await waitForCardIn(page3, demo.id, 'To Do');
    const body3b = assertOneMoveRequest(log3, demo.id, 'the drag out of Done');
    assert.strictEqual(body3b.column, 'To Do', 'the drag out of Done must carry To Do as the target column');
    const after3 = await snapshot(page3);
    const backCard = after3.find((c) => c.column === 'To Do').cards.find((c) => c.id === demo.id);
    assert.strictEqual(backCard.done, false, 'a card dragged out of Done must lose the done treatment');
    await assertNoDoneControl(page3, 'after the round trip out of Done');
    assert.strictEqual(await page3.evaluate(() => window.__kw5NoReloadMarker), 'alive',
      'the Done round trip must never reload the page');
    assert.deepStrictEqual(after3, boardView(await getJSON(`http://127.0.0.1:${port3}/board`)),
      'after the Done round trip the page does not mirror GET /board');

    await stopServer(srv3);
    await page3.close();
    console.log('scenario 3 (done is column membership — treatment with membership, no done control): OK');

    // --- Scenario 3 cross-check — the done-edit leg, FLIPPED for the
    // contract-level done freeze (amendment 2026-10-07, parent scenario 15
    // "Edit of a done card is rejected"): on a card the DRAG places in Done
    // the edit affordance must DISAPPEAR (kw3's retired "editing a Done
    // card keeps the treatment" coverage — delete + drag hooks stay), and
    // after the drag back OUT it must be BACK, with the edit there landing
    // exactly one PATCH to the card endpoint ("becomes editable after it is
    // moved out of Done"). The post-drop fragment's INTERACTION WIRING
    // stays PINNED HARD on that markup: the edit Save is exactly one PATCH
    // with no page navigation, and a Delete (after a further in-place drop
    // re-injects the fetch markup) exactly one DELETE with the card gone.
    // (This block previously WITNESSED a product bug — the drag's fetch
    // swap assigned raw innerHTML and htmx 2.0.6 auto-processes nothing it
    // did not swap, leaving post-drop controls unwired; render.go's swap
    // site now calls htmx.process on the swapped region, so the behavior
    // is asserted on the post-drop markup too, not reported.)
    const dbs3x = tmpPaths('done-edit-crosscheck');
    run(seedBin, ['--db', dbs3x.board, '--titles', 'Prepare the demo,File the expense claim', '--in-progress', '0']);
    const port3x = await freePort();
    const srv3x = startServer(port3x, dbs3x.board);
    await waitForHTTP(`http://127.0.0.1:${port3x}/board`);
    const page3x = await browser.newPage();
    await page3x.goto(`http://127.0.0.1:${port3x}/`);
    const log3x = instrumentPatches(page3x);
    const before3x = await snapshot(page3x);
    const demoX = before3x.find((c) => c.column === 'In Progress').cards[0];
    log3x.reset();
    await dragTo(page3x, cardById(page3x, demoX.id), await pointOn(page3x.locator('#column-done p.column__empty'), 0.5));
    await waitForCardIn(page3x, demoX.id, 'Done');
    assert.strictEqual(assertOneMoveRequest(log3x, demoX.id, 'the cross-check drop into Done').column, 'Done',
      'the cross-check card must reach Done through the one-request drop');

    // The flip (kw8 done freeze, parent scenario 15): dragging INTO Done
    // makes the edit affordance DISAPPEAR — asserted on the fetch-swapped
    // post-drop markup, so the freeze holds on the drag's own render path,
    // not just a fresh page. This block used to reload and EDIT the Done
    // card (kw3's cross-check); that leg retires with the affordance the
    // freeze removed. What stays at this seam is the freeze's UI half: no
    // Edit control, no edit band — delete and the drag hook intact.
    const doneLiX = cardById(page3x, demoX.id);
    assert.strictEqual(await doneLiX.locator('.card__edit').count(), 0,
      'the card dragged into Done must render no edit control (done freeze — parent scenario 15)');
    assert.strictEqual(await doneLiX.locator('.edit-form').count(), 0,
      'the card dragged into Done must render no edit band (done freeze — parent scenario 15)');
    assert.strictEqual(await doneLiX.locator('.card__delete').count(), 1,
      'a card dragged into Done keeps its delete control — deleting is not editing');
    assert.strictEqual(await doneLiX.getAttribute('draggable'), 'true',
      'the card dragged into Done stays draggable — dragging out is the unlock');

    // Fresh-truth render agrees: the freeze is the render's rule, not a
    // fragment artifact of the drop swap.
    await page3x.reload();
    assert.strictEqual(await cardById(page3x, demoX.id).locator('.card__edit').count(), 0,
      'the fresh render of a Done card must carry no edit control either');

    // Post-drop wiring PINNED (the block that previously witnessed the
    // product bug): drag the card so the fetch swap injects fresh markup,
    // then EDIT that markup — Save must be exactly one PATCH to the card
    // endpoint, must not navigate the page (an unwired form submits
    // natively: GET /?title=..., a full navigation, edit discarded), and
    // must update the title. This drag is also scenario 15's UNLOCK drag —
    // out of Done, back into To Do — so the edit doubles as the freeze's
    // "becomes editable after it is moved out of Done" leg, landing one
    // PATCH. The edit's own swap is htmx-mediated, so the DELETE pin needs
    // a drop of its own to re-inject the fetch markup: drag the card again,
    // then Delete it — exactly one DELETE to the card endpoint, no
    // navigation, the card gone from the page.
    const deleteLog = [];
    page3x.on('request', (req) => {
      if (req.method() === 'DELETE') deleteLog.push({ url: req.url() });
    });
    log3x.reset();
    await dragTo(page3x, cardById(page3x, demoX.id), await pointOn(page3x.locator('#column-to-do li.card').first(), 0.75));
    await waitForCardIn(page3x, demoX.id, 'To Do');
    // The unlock half of scenario 15: moved out of Done, the card regains
    // its edit control — on the very fetch-injected markup the wiring pin
    // below now exercises.
    assert.strictEqual(await cardById(page3x, demoX.id).locator('.card__edit').count(), 1,
      'the card dragged out of Done must regain its edit control (drag-out is the unlock — parent scenario 15)');
    const urlBefore = await page3x.evaluate(() => location.href);
    const renamedPostDrop = 'Renamed on fetch-swapped markup';
    await page3x.locator(`li.card[data-card="${demoX.id}"] .card__edit`).click();
    await page3x.locator(`li.card[data-card="${demoX.id}"] .edit-form input.input[name=title]`).fill(renamedPostDrop);
    log3x.reset();
    await page3x.locator(`li.card[data-card="${demoX.id}"] .edit-form .save`).click();
    // A buggy native submit navigates, which Playwright survives by
    // re-attaching to the fresh page — the title then never changes and
    // this wait times out; the assertions below state the failure.
    await page3x
      .locator(`li.card[data-card="${demoX.id}"] .card__title`)
      .waitForFunction((el, t) => el.textContent === t, renamedPostDrop, { timeout: 10000 })
      .catch(() => {});
    assert.strictEqual(log3x.patches.length, 1,
      `the post-drop edit must be exactly one PATCH to the card endpoint, saw ${JSON.stringify(log3x.patches.map((p) => p.url))}`);
    assert.ok(log3x.patches[0].url.endsWith(`/ui/cards/${demoX.id}`),
      `the post-drop edit must address the card endpoint, not a move: ${log3x.patches[0].url}`);
    assert.strictEqual(page3x.url(), urlBefore,
      'the post-drop edit must not navigate the page — an unwired form submits natively (GET /?title=...)');
    assert.strictEqual(
      await page3x.locator(`li.card[data-card="${demoX.id}"] .card__title`).textContent(),
      renamedPostDrop,
      'the post-drop edit must update the card title'
    );

    // Re-run the fetch swap over the card so the Delete control under test
    // is one the fetch swap injected: an in-place re-order drop (the card
    // lands where it sits — accepted, one PATCH, the truth swaps back).
    // The gesture targets the mid-viewport To Do card like every other drop
    // in this lane — the empty Done placeholder sits too near the bottom
    // edge, where the taller post-edit page auto-scrolls mid-drag and the
    // release lands outside every column. The detached-node wait proves the
    // swap landed before the Delete click — no racing the fetch response.
    await cardById(page3x, demoX.id).evaluate((el) => { window.__preDropCard = el; });
    log3x.reset();
    await dragTo(page3x, cardById(page3x, demoX.id), await pointOn(page3x.locator('#column-to-do li.card').first(), 0.75));
    await page3x.waitForFunction(() => window.__preDropCard && !window.__preDropCard.isConnected, null, { timeout: 10000 });
    deleteLog.length = 0;
    await page3x.locator(`li.card[data-card="${demoX.id}"] .card__delete`).click();
    await page3x.waitForFunction(
      (cardId) => !document.querySelector(`li.card[data-card="${cardId}"]`),
      demoX.id,
      { timeout: 10000 }
    );
    assert.strictEqual(deleteLog.length, 1,
      `the post-drop delete must be exactly one DELETE request, saw ${deleteLog.length}: ${JSON.stringify(deleteLog.map((d) => d.url))}`);
    assert.ok(deleteLog[0].url.endsWith(`/ui/cards/${demoX.id}`),
      `the post-drop delete must address the card endpoint: ${deleteLog[0].url}`);
    assert.strictEqual(page3x.url(), urlBefore,
      'the post-drop delete must not navigate the page — an unwired hx-delete control issues nothing or navigates');

    await stopServer(srv3x);
    await page3x.close();
    console.log('scenario 3 cross-check (done freeze — drag in removes the edit affordance, drag out restores it, edit one PATCH): OK');

    // --- Scenario 4 (the MOVE leg of "Operation on missing card" — the leg
    // that completes the scenario: the edit leg went green at kw3, the
    // delete leg at kw4). Stale-drag staging: two browser contexts on the
    // SAME server. Context A deletes the card through its own Delete control
    // — an honest in-band deletion by the other session; from B's seat the
    // card ceases to exist while B's render keeps showing it, exactly the
    // workplan risk "a stale page drags a card that another action deleted".
    // Context B never refreshed, so its stale card is still there and still
    // draggable. Dropping it attempts EXACTLY ONE PATCH; the contract
    // answers 404; the shared stale-failure surface states "no such card"
    // over the re-rendered truth. Nothing moved; the page under the banner
    // IS GET /board.
    const dbs4 = tmpPaths('drag-stale');
    run(seedBin, [
      '--db', dbs4.board,
      '--titles', 'Review the migration plan,Fix the flaky test,Update the runbook,Rotate the service keys',
      '--in-progress', '3',
    ]);
    const port4 = await freePort();
    const srv4 = startServer(port4, dbs4.board);
    await waitForHTTP(`http://127.0.0.1:${port4}/board`);
    const ctxA = await browser.newContext();
    const ctxB = await browser.newContext();
    const pageA = await ctxA.newPage();
    const pageB = await ctxB.newPage();
    await pageA.goto(`http://127.0.0.1:${port4}/`);
    await pageB.goto(`http://127.0.0.1:${port4}/`);
    const logB = instrumentPatches(pageB);

    const before4 = await snapshot(pageB);
    const stale = before4.find((c) => c.column === 'To Do').cards[1];

    await pageA.locator(`li.card[data-card="${stale.id}"] .card__delete`).click();
    await pageA.waitForFunction(
      (cardId) => !document.querySelector(`li.card[data-card="${cardId}"]`),
      stale.id,
      { timeout: 10000 }
    );
    const truth4 = await getJSON(`http://127.0.0.1:${port4}/board`);
    assert.ok(!truth4.columns.some((col) => col.cards.some((card) => card.id === stale.id)),
      'GET /board must not contain the card context A deleted');
    assert.strictEqual(await cardById(pageB, stale.id).count(), 1,
      "context B's page must still show the card — genuinely stale, and still draggable");

    logB.reset();
    await dragTo(pageB, cardById(pageB, stale.id),
      await pointOn(pageB.locator('#column-in-progress li.card').first(), 0.75));

    const banner = pageB.locator(MISSING_BANNER);
    await banner.waitFor({ state: 'visible' });
    assert.match((await banner.textContent()).trim(), /no such card/,
      "the stale move must state the contract's reason verbatim");
    assert.strictEqual(logB.patches.length, 1,
      `the stale drop must attempt EXACTLY ONE PATCH, saw ${logB.patches.length}`);
    assert.ok(logB.patches[0].url.endsWith(`/ui/cards/${stale.id}/move`),
      `the stale drop's PATCH addressed ${logB.patches[0].url}, want the move endpoint for the stale card`);
    // Nothing faked, nothing moved: under the banner the truth re-rendered.
    assert.strictEqual(await cardById(pageB, stale.id).count(), 0,
      'the missing card must be gone from context B after the stated failure');
    const after4 = await snapshot(pageB);
    assert.deepStrictEqual(after4, boardView(truth4),
      "after the stale move context B does not mirror the contract's truth");
    assert.deepStrictEqual(after4, before4.map((col) => ({ ...col, cards: col.cards.filter((c) => c.id !== stale.id) })),
      'the failed drag changed something beyond dropping the stale card from the stale page');
    // The statement never outlives a reload.
    await pageB.reload();
    assert.strictEqual(await pageB.locator(MISSING_BANNER).count(), 0,
      'the missing-card statement must not survive a reload');

    await stopServer(srv4);
    await ctxA.close();
    await ctxB.close();
    console.log('scenario 4 (missing-card move leg — stale drag, one PATCH attempted, stated failure): OK');

    // --- Scenario 5: "abandoned drag changes nothing" (card ui/10) — the
    // gesture the parent scenarios presuppose. Drag a card, hover it over
    // the gutter between columns AND over a column, then release OUTSIDE
    // every column (over the page heading). Outside a .column the client
    // never preventDefaults dragover, so drop never fires and the page's
    // single fetch site is unreachable from the gesture: ZERO PATCH
    // requests, the board byte-for-byte unchanged in DOM and in
    // GET /board, no reload, and no drag chrome left behind.
    const dbs5 = tmpPaths('drag-abandoned');
    run(seedBin, [
      '--db', dbs5.board,
      '--titles', 'Write the weekly report,Call the plumber,Prepare the demo,Ship v1.2',
      '--in-progress', '2',
      '--done', '3',
    ]);
    const port5 = await freePort();
    const srv5 = startServer(port5, dbs5.board);
    await waitForHTTP(`http://127.0.0.1:${port5}/board`);
    const page5 = await browser.newPage();
    await page5.goto(`http://127.0.0.1:${port5}/`);
    const log5 = instrumentPatches(page5);

    await page5.evaluate(() => { window.__kw5NoReloadMarker = 'alive'; });
    const before5 = await snapshot(page5);
    const wander = before5.find((c) => c.column === 'To Do').cards[0];
    const outsidePoint = await centerOf(page5.locator('h1.heading')); // the page heading: outside every column
    const between = await page5.locator('#column-to-do').evaluate((sec) => {
      const box = sec.getBoundingClientRect();
      return { x: box.right + 8, y: box.top + box.height / 2 };
    });

    log5.reset();
    await dragTo(page5, cardById(page5, wander.id), outsidePoint, {
      via: [between, await centerOf(page5.locator('#column-in-progress'))],
    });
    // Let any (buggy) request the gesture could have triggered arrive and
    // be counted before asserting the zero.
    await page5.waitForTimeout(300);

    assertNoRequests(log5, 'the abandoned drag');
    assert.strictEqual(await page5.evaluate(() => window.__kw5NoReloadMarker), 'alive',
      'an abandoned drag must not reload the page');
    const after5 = await snapshot(page5);
    assert.deepStrictEqual(after5, before5, 'the abandoned drag changed the board in the DOM');
    assert.deepStrictEqual(after5, boardView(await getJSON(`http://127.0.0.1:${port5}/board`)),
      'the abandoned drag changed the stored board');
    await assertNoDragChrome(page5, 'after the abandoned drag');

    await stopServer(srv5);
    await page5.close();
    console.log('scenario 5 (abandoned drag — zero requests, board unchanged): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  for (const w of witnesses) console.log('witness:', w);
  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW5 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
