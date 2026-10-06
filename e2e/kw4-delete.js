// KW4 e2e — delete end-to-end in the browser, re-executing the parent
// scenario "Delete card" plus the delete leg of "Operation on missing card"
// (workplan_kanban_application.md; wave-end contract:
// workplans/dependencies_kanban.md §KW4). The surfaces are card ui/07 — the
// per-card Delete control (hx-delete → DELETE /ui/cards/{id}, swap of the
// fresh board into #board-area, no confirmation dialog) and card ui/02's
// stated empty treatment on a column that emptied — and edit's stated-404
// banner (#missing-card), one mechanism per failure class across verbs.
// CommonJS so `require('playwright')` resolves via NODE_PATH=$(npm root -g).
//
// Flow: build binaries -> seed a board across all three columns -> start
// server -> delete a middle To Do card and a Done card, asserting the card
// removal without a reload, the column packing up in order with contiguous
// stored positions agreeing with a fresh GET /board, other columns untouched,
// and Done's stated empty treatment once it empties; then the stale-page
// delete of a card removed out-of-band, which states "no such card" over the
// re-rendered truth; then the delete of a column's last card, showing that
// column's empty treatment while the others keep their cards — SIGTERM
// teardown. Fresh server per scenario, mirroring the KW1–KW3 lanes.
//
// Missing-card mutation (scenario 2): same out-of-band sqlite3 DELETE against
// the board data file from this lane's process as the KW3 lane pioneered.
// The card DELETE endpoint exists since KW4, but the scenario's Given is "no
// card exists" while the PAGE still shows the card — deleting through the
// endpoint would re-render the page truth first and destroy the stale state,
// so the file-level delete stays the honest way to stage the Given.
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
// temp file so no run touches a file beside the repo.
function startServer(port, boardDb) {
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--board-db', boardDb], {
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

// Out-of-band card removal (scenario 2's Given): direct SQL DELETE against
// the board data file from a separate process — the sqlite3 CLI, with a busy
// timeout so a concurrent store write never races the lane. This simulates
// the card vanishing behind a stale page's back.
function deleteCardOutOfBand(dbPath, id) {
  run('sqlite3', [dbPath, `PRAGMA busy_timeout=5000; DELETE FROM cards WHERE id = ${id};`]);
}

// Contract truths this lane asserts against — the surfaces card ui/07 and
// card ui/02 render: the per-card Delete control (hx-delete →
// DELETE /ui/cards/{id}), the missing-card banner (#missing-card) stating
// the contract's reason over the truth, and a column's stated emptiness.
const DELETE_BUTTON = '.card__delete';
const MISSING_BANNER = '#missing-card';
const EMPTY_STATEMENT = (anchor) => `#column-${anchor} p.column__empty[data-empty="true"]`;

// The whole board as the DOM shows it: per fixed column, its cards
// top-to-bottom as {title, id, done}. DOM order IS the assertion surface
// for "keep their relative order with no gap" — a delete must drop exactly
// one entry from one column's array of this structure and nothing else.
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

// The contract's position invariant after a mutation: within every column
// the cards carry contiguous positions 0..n-1 in array order — "no gap
// between positions" is the store renormalizing, observable through the
// contract. Returns the same {title, id, position} rows so DOM order can be
// tied to stored order card-for-card.
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

// The user's delete gesture: activate the card's Delete control. No
// confirmation exists — the click is the whole operation.
async function deleteCard(page, card) {
  await card.locator(DELETE_BUTTON).click();
}

// Wait for the swap that removes one card from the page — the fragment
// landing is what "disappears without a reload" observably is.
async function waitForCardGone(page, id) {
  await page.waitForFunction(
    (cardId) => !document.querySelector(`li.card[data-card="${cardId}"]`),
    id,
    { timeout: 10000 }
  );
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    // --- Scenario 1: "Delete card" — a seeded board spans all three
    // columns (three To Do cards give the deleted one real neighbors above
    // and below; the Done leg needs a Done card). Deleting a middle To Do
    // card drops it with no page reload, the column packs up in order with
    // contiguous positions agreeing with a fresh GET /board, and every other
    // card on the board is untouched; deleting the Done card — a card is as
    // deletable as any other, done is just a column — empties Done into its
    // stated treatment. A reload still shows both cards gone, in agreement
    // with the contract.
    const seededTitles = [
      'Write the weekly report', // -> To Do
      'Call the plumber',        // -> To Do (the middle card — this lane deletes it)
      'File the expense claim',  // -> To Do
      'Draft the launch note',   // -> In Progress
      'Ship v1.2',               // -> Done (the single Done card)
    ];
    const dbs1 = tmpPaths('delete-card');
    run(seedBin, [
      '--db', dbs1.board,
      '--titles', seededTitles.join(','),
      '--in-progress', '3',
      '--done', '4',
    ]);
    const port1 = await freePort();
    const srv1 = startServer(port1, dbs1.board);
    await waitForHTTP(`http://127.0.0.1:${port1}/board`);
    await page.goto(`http://127.0.0.1:${port1}/`);

    // Reload marker: lives on the page's window object only as long as no
    // navigation happens — a delete is a fragment swap.
    await page.evaluate(() => { window.__kw4NoReloadMarker = 'alive'; });
    const before1 = await snapshot(page);
    const todo1 = before1.find((c) => c.column === 'To Do');
    const middleIndex = 1;
    const deleted = todo1.cards[middleIndex];
    assert.ok(todo1.cards.length > middleIndex + 1, 'the seeded To Do column must have a card below the deleted one');

    await deleteCard(page, cardById(page, deleted.id));
    await waitForCardGone(page, deleted.id);

    assert.strictEqual(await page.evaluate(() => window.__kw4NoReloadMarker), 'alive',
      'the delete must not reload the page — the removal is a fragment swap onto the existing render');
    const after1 = await snapshot(page);
    // The whole-board view: exactly this one card dropped from its column,
    // every other card — order, id, done treatment — identical. That is
    // "no longer shows that card" + "keeps their relative order" + "every
    // other card untouched" pinned at once.
    const expected1 = before1.map((col) => ({
      ...col,
      cards: col.cards.filter((card) => card.id !== deleted.id),
    }));
    assert.deepStrictEqual(after1, expected1,
      'the delete changed something beyond dropping the one card (order packed wrong, or another card moved)');

    // The contract side of "no gap between positions": fresh GET /board
    // carries contiguous 0..n-1 positions per column, and its card order is
    // the DOM's order card-for-card — screen order IS stored order.
    const json1 = await getJSON(`http://127.0.0.1:${port1}/board`);
    const rows1 = assertContiguousPositions(json1, 'after the To Do delete');
    assert.deepStrictEqual(rows1['To Do'],
      expected1.find((c) => c.column === 'To Do').cards.map((card, i) => ({ title: card.title, id: card.id, position: i })),
      'the packed To Do column in the DOM does not agree with GET /board order and positions');
    assert.deepStrictEqual(boardView(json1), after1,
      'after the To Do delete the page does not mirror GET /board');

    // The Done leg: a Done card is as deletable as any other (J6: done is
    // just a column). Deleting this board's only Done card empties Done —
    // which then shows its stated treatment — while the other columns keep
    // their cards untouched.
    const doneCard = before1.find((c) => c.column === 'Done').cards[0];
    await deleteCard(page, cardById(page, doneCard.id));
    await waitForCardGone(page, doneCard.id);

    assert.strictEqual(await page.evaluate(() => window.__kw4NoReloadMarker), 'alive',
      'the Done delete must also stay a fragment swap, not a page reload');
    const emptyDone = page.locator(EMPTY_STATEMENT('done'));
    assert.strictEqual(await emptyDone.count(), 1, 'an emptied Done column must state its emptiness');
    assert.ok(await emptyDone.isVisible(), 'the emptied Done column\'s emptiness statement must be visible');
    const after1b = await snapshot(page);
    const expected1b = after1.map((col) => ({
      ...col,
      cards: col.cards.filter((card) => card.id !== doneCard.id),
    }));
    assert.deepStrictEqual(after1b, expected1b,
      'the Done delete changed something beyond dropping the one Done card');
    assertContiguousPositions(await getJSON(`http://127.0.0.1:${port1}/board`), 'after the Done delete');

    // Reload: both cards stay gone, and the fresh render agrees with the
    // contract.
    await page.reload();
    const reloaded1 = await snapshot(page);
    assert.deepStrictEqual(reloaded1, expected1b,
      'after reload the deleted cards are back, or the survivors moved');
    assert.deepStrictEqual(reloaded1, boardView(await getJSON(`http://127.0.0.1:${port1}/board`)),
      'the reloaded page does not agree with a fresh GET /board');

    await stopServer(srv1);
    console.log('scenario 1 (delete card): OK');

    // --- Scenario 2 (delete leg of "Operation on missing card"): a stale
    // page deletes a card that no longer exists. The seed board is opened
    // and rendered; then the target card is removed SERVER-SIDE out-of-band
    // — direct sqlite3 DELETE against the board data file from this lane's
    // process, so the page stays stale while the truth already lacks the
    // card. Clicking its Delete then states the contract's "no such card",
    // the truth re-renders without the card, and nothing is faked: the
    // board shown after the failure IS GET /board.
    const dbs2 = tmpPaths('delete-missing');
    run(seedBin, ['--db', dbs2.board, '--titles', 'Write the weekly report,Call the plumber,File the expense claim']);
    const port2 = await freePort();
    const srv2 = startServer(port2, dbs2.board);
    await waitForHTTP(`http://127.0.0.1:${port2}/board`);
    await page.goto(`http://127.0.0.1:${port2}/`);

    const before2 = await snapshot(page);
    const stale = before2.find((c) => c.column === 'To Do').cards[1];
    deleteCardOutOfBand(dbs2.board, stale.id);
    // The page has not re-rendered: it still shows the now-missing card —
    // this is precisely the stale state the scenario's Given describes, and
    // the wiring is still live on it.
    assert.strictEqual(await cardById(page, stale.id).count(), 1,
      'the stale page must still show the card between the out-of-band delete and the click');
    assert.strictEqual(await cardById(page, stale.id).locator(DELETE_BUTTON).count(), 1,
      'the stale card must still carry its Delete control before the click');

    await page.evaluate(() => { window.__kw4NoReloadMarker = 'alive'; });
    await deleteCard(page, cardById(page, stale.id));

    const banner = page.locator(MISSING_BANNER);
    await banner.waitFor({ state: 'visible' });
    assert.match((await banner.textContent()).trim(), /no such card/,
      'the missing-card failure must state the contract\'s reason verbatim');
    assert.strictEqual(await page.evaluate(() => window.__kw4NoReloadMarker), 'alive',
      'the missing-card failure is a fragment swap, not a page reload');
    // The card is gone from the page, and the board under the banner is the
    // server's truth — nothing remains of the card the click could not act
    // on, and nothing else moved.
    assert.strictEqual(await cardById(page, stale.id).count(), 0,
      'the missing card must be gone from the page after the stated failure');
    const after2 = await snapshot(page);
    const json2 = boardView(await getJSON(`http://127.0.0.1:${port2}/board`));
    assert.deepStrictEqual(after2, json2,
      'after the missing-card failure the page does not mirror the contract\'s truth');
    assert.ok(!after2.some((col) => col.cards.some((card) => card.id === stale.id)),
      'GET /board still contains the card the page said was missing');
    assert.deepStrictEqual(after2, expected2Truth(before2, stale),
      'the failed delete changed something beyond removing the stale card from the page');

    // The statement never outlives the reload: reloading reads the same
    // truth and shows no banner, still agreeing with a fresh GET /board.
    await page.reload();
    assert.strictEqual(await page.locator(MISSING_BANNER).count(), 0,
      'the missing-card statement must not survive a reload');
    assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(`http://127.0.0.1:${port2}/board`)),
      'the reloaded page does not agree with a fresh GET /board');

    await stopServer(srv2);
    console.log('scenario 2 (operation on missing card — delete leg): OK');

    // --- Scenario 3: the delete of a column's LAST card shows that
    // column's stated empty treatment (card ui/02's render rule reached
    // through deletion) while every other column keeps its cards. No
    // reload: the empty statement arrives in the same swap.
    const dbs3 = tmpPaths('delete-last');
    run(seedBin, [
      '--db', dbs3.board,
      '--titles', 'Write the weekly report,Plan the sprint review,Book the team offsite,Archive last quarter',
      '--in-progress', '2',
      '--done', '3',
    ]);
    const port3 = await freePort();
    const srv3 = startServer(port3, dbs3.board);
    await waitForHTTP(`http://127.0.0.1:${port3}/board`);
    await page.goto(`http://127.0.0.1:${port3}/`);

    await page.evaluate(() => { window.__kw4NoReloadMarker = 'alive'; });
    const before3 = await snapshot(page);
    const last = before3.find((c) => c.column === 'In Progress').cards[0];
    assert.strictEqual(before3.find((c) => c.column === 'In Progress').cards.length, 1,
      'the seeded In Progress column must hold exactly the one card this scenario deletes');

    await deleteCard(page, cardById(page, last.id));
    await waitForCardGone(page, last.id);

    assert.strictEqual(await page.evaluate(() => window.__kw4NoReloadMarker), 'alive',
      'the last-card delete must stay a fragment swap, not a page reload');
    const emptyInProgress = page.locator(EMPTY_STATEMENT('in-progress'));
    assert.strictEqual(await emptyInProgress.count(), 1, 'a column emptied by deleting its last card must state its emptiness');
    assert.ok(await emptyInProgress.isVisible(), 'the empty statement must be visible in the emptied column');
    assert.strictEqual(await page.locator(`#column-in-progress li.card`).count(), 0,
      'the emptied column must show no card nodes');
    // Every other column keeps its cards — the deletion reaches only its
    // own column.
    const after3 = await snapshot(page);
    const expected3 = before3.map((col) => ({
      ...col,
      cards: col.column === 'In Progress' ? [] : col.cards,
    }));
    assert.deepStrictEqual(after3, expected3,
      'deleting a column\'s last card disturbed another column');
    assertContiguousPositions(await getJSON(`http://127.0.0.1:${port3}/board`), 'after the last-card delete');
    assert.deepStrictEqual(after3, boardView(await getJSON(`http://127.0.0.1:${port3}/board`)),
      'after the last-card delete the page does not mirror GET /board');
    await page.reload();
    assert.strictEqual(await page.locator(EMPTY_STATEMENT('in-progress')).count(), 1,
      'after reload the emptied column must still state its emptiness');

    await stopServer(srv3);
    console.log('scenario 3 (delete a column\'s last card — stated empty treatment): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW4 e2e: all scenarios passed');
}

// The board truth of scenario 2: the out-of-band delete removed the stale
// card from its column, every other card kept its place — the page after
// the failed delete must equal exactly this, which is what "nothing faked"
// pins: the failure re-renders the truth, it does not invent a state.
function expected2Truth(before, stale) {
  return before.map((col) => ({
    ...col,
    cards: col.cards.filter((card) => card.id !== stale.id),
  }));
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
