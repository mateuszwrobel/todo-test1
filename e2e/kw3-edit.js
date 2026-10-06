// KW3 e2e — inline edit end-to-end in the browser, re-executing the parent
// scenarios "Edit card text keeps place" + "Operation on missing card" (the
// edit leg: KW3's PATCH surface; the move/delete legs join at KW4/KW5) plus
// the edit's stated rejection legs, which card ui/06 renders on the editing
// card (workplan_kanban_application.md; wave-end contract:
// workplans/dependencies_kanban.md §KW3). CommonJS so
// `require('playwright')` resolves via NODE_PATH=$(npm root -g).
//
// Flow: build binaries -> seed a board across all three columns -> start
// server -> edit a middle To Do card and a Done card, asserting the in-place
// swap (no reload, same id, same position, done treatment kept), then the
// stale-page edit against a card removed out-of-band, then the two
// rejections stated at the card -> SIGTERM teardown. Fresh server per
// scenario, mirroring the KW1/KW2 lanes.
//
// Missing-card mutation (scenario 2): the lane deletes the target card's row
// straight from the board data file with the sqlite3 CLI, from its own
// process, between page load and Save. No card DELETE endpoint exists until
// KW4, so an out-of-band file mutation is the only honest way to make "no
// card exists with identifier X" true against a live composed server while
// the page stays stale. The store reads in fresh transactions, so the next
// contract read sees the committed delete.
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

// Out-of-band card removal (scenario 2's Given): direct SQL DELETE against
// the board data file from a separate process — the sqlite3 CLI, with a busy
// timeout so a concurrent store write never races the lane. This simulates
// the card vanishing behind a stale page's back.
function deleteCardOutOfBand(dbPath, id) {
  run('sqlite3', [dbPath, `PRAGMA busy_timeout=5000; DELETE FROM cards WHERE id = ${id};`]);
}

// Contract truths this lane asserts against — the surface card ui/06
// renders: the per-card Edit control, the inline edit band prefilled with
// the card's title, the card-attached refusal (p#edit-error-{id}), and the
// missing-card banner (#missing-card) stating the contract's reason.
const EDIT_BUTTON = '.card__edit';
const EDIT_INPUT = '.edit-form input.input[name=title]';
const EDIT_SAVE = '.edit-form button.save';
const EDIT_ERROR = (id) => `p#edit-error-${id}`;
const MISSING_BANNER = '#missing-card';

// The whole board as the DOM shows it: per fixed column, its cards
// top-to-bottom as {title, id, done}. DOM order IS the assertion surface
// for "same column at the same position" — an edit must change exactly one
// title field of this structure and nothing else.
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

// A card in the DOM by its identifier, anywhere on the board.
function cardById(page, id) {
  return page.locator(`li.card[data-card="${id}"]`);
}

// The user's edit gesture on a card: reveal the band via the card's Edit
// control, type the new text, submit. The band is prefilled; fill replaces.
async function editCard(page, card, text) {
  await card.locator(EDIT_BUTTON).click();
  await card.locator(EDIT_INPUT).fill(text);
  await card.locator(EDIT_SAVE).click();
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    // --- Scenario 1: "Edit card text keeps place" — a seeded board spans
    // all three columns (the done leg needs a Done card; three To Do cards
    // give the edited one real neighbors above and below). Editing a middle
    // To Do card's text shows the new title at the same position in the
    // same column, between the same neighbors, under the same identifier,
    // without a page reload and without the done treatment; editing a Done
    // card keeps that card's done treatment — done is column membership,
    // untouched by a title change. A reload then agrees with a fresh
    // GET /board.
    const seededTitles = [
      'Write the weekly report', // -> To Do
      'Call the plumber',        // -> To Do (the middle card — this lane edits it)
      'File the expense claim',  // -> To Do
      'Draft the launch note',   // -> In Progress
      'Ship v1.2',               // -> Done
    ];
    const dbs1 = tmpPaths('edit-place');
    run(seedBin, [
      '--db', dbs1.board,
      '--titles', seededTitles.join(','),
      '--in-progress', '3',
      '--done', '4',
    ]);
    const port1 = await freePort();
    const srv1 = startServer(port1, dbs1.todo, dbs1.board);
    await waitForHTTP(`http://127.0.0.1:${port1}/board`);
    await page.goto(`http://127.0.0.1:${port1}/`);

    // Reload marker: lives on the page's window object only as long as no
    // navigation happens — an in-place edit is a fragment swap.
    await page.evaluate(() => { window.__kw3NoReloadMarker = 'alive'; });
    const before1 = await snapshot(page);
    const todo1 = before1.find((c) => c.column === 'To Do');
    const middleIndex = 1;
    const edited = todo1.cards[middleIndex];
    assert.ok(todo1.cards.length > middleIndex + 1, 'the seeded To Do column must have a card below the edited one');

    // The band reveals prefilled with the card's existing text (the
    // scenario's "card with an existing text" shown in the editor).
    await cardById(page, edited.id).locator(EDIT_BUTTON).click();
    const band = cardById(page, edited.id).locator(EDIT_INPUT);
    await band.waitFor({ state: 'visible' });
    assert.strictEqual(await band.inputValue(), edited.title,
      'the edit band must open prefilled with the card\'s existing text');

    const editedTitle = 'Call the plumber, again';
    await band.fill(editedTitle);
    await cardById(page, edited.id).locator(EDIT_SAVE).click();
    await cardById(page, edited.id).locator('.card__title')
      .waitForFunction((el, text) => el.textContent === text, editedTitle);

    assert.strictEqual(await page.evaluate(() => window.__kw3NoReloadMarker), 'alive',
      'the edit must not reload the page — the update is a fragment swap onto the existing render');
    const after1 = await snapshot(page);
    // Same structure everywhere else: the edit changed exactly this one
    // card's title — identifier, position, neighbors, columns, done flags
    // all identical, which is what "keeps place" observably means.
    const expected1 = before1.map((col) => ({
      ...col,
      cards: col.cards.map((card) =>
        card.id === edited.id && col.column === 'To Do' ? { ...card, title: editedTitle } : card
      ),
    }));
    assert.deepStrictEqual(after1, expected1,
      'the edit changed something beyond the edited card\'s text (position, id, neighbors, or done flags moved)');
    const editedAfter1 = after1.find((c) => c.column === 'To Do').cards[middleIndex];
    assert.strictEqual(editedAfter1.id, edited.id, 'the edited card\'s identifier is unchanged');
    assert.ok(!editedAfter1.done, 'a card edited in To Do does not carry the done treatment');

    // The Done leg: editing a Done card's text keeps its done treatment —
    // the treatment belongs to the column membership the edit never touches.
    const doneCard = before1.find((c) => c.column === 'Done').cards[0];
    await cardById(page, doneCard.id).locator(EDIT_BUTTON).click();
    const doneBand = cardById(page, doneCard.id).locator(EDIT_INPUT);
    await doneBand.waitFor({ state: 'visible' });
    assert.strictEqual(await doneBand.inputValue(), doneCard.title,
      'the edit band on a Done card must also open prefilled (done cards stay editable)');
    const doneTitle = 'Ship v1.2 (final)';
    await doneBand.fill(doneTitle);
    await cardById(page, doneCard.id).locator(EDIT_SAVE).click();
    await cardById(page, doneCard.id).locator('.card__title')
      .waitForFunction((el, text) => el.textContent === text, doneTitle);

    const after1b = await snapshot(page);
    const expected1b = after1.map((col) => ({
      ...col,
      cards: col.cards.map((card) =>
        card.id === doneCard.id && col.column === 'Done' ? { ...card, title: doneTitle } : card
      ),
    }));
    assert.deepStrictEqual(after1b, expected1b,
      'editing a Done card changed something beyond its text');
    const doneAfter = after1b.find((c) => c.column === 'Done').cards[0];
    assert.strictEqual(doneAfter.id, doneCard.id, 'the edited Done card\'s identifier is unchanged');
    assert.ok(doneAfter.done, 'an edited Done card keeps the done treatment (it stays in the Done column)');

    // Reload: the fresh render is the stored truth — DOM agrees with GET
    // /board card-for-card, column-for-column.
    await page.reload();
    const reloaded1 = await snapshot(page);
    const freshJSON1 = await getJSON(`http://127.0.0.1:${port1}/board`);
    assert.deepStrictEqual(boardView(freshJSON1), reloaded1,
      'the reloaded page does not agree with a fresh GET /board');
    assert.deepStrictEqual(reloaded1, after1b,
      'the reloaded page shows different cards than the in-place edits produced');

    await stopServer(srv1);
    console.log('scenario 1 (edit card text keeps place): OK');

    // --- Scenario 2 (edit leg of "Operation on missing card"): a stale
    // page edits a card that no longer exists. The seed board is opened and
    // rendered; then the target card is deleted SERVER-SIDE out-of-band —
    // direct sqlite3 DELETE against kanban.db from this lane's process (no
    // card DELETE endpoint exists before KW4; see the file header) — while
    // the page keeps showing it. Editing it then states the contract's
    // "no such card", the truth re-renders without the card, and nothing
    // of the stale card remains on the page.
    const dbs2 = tmpPaths('edit-missing');
    run(seedBin, ['--db', dbs2.board, '--titles', 'Write the weekly report,Call the plumber,File the expense claim']);
    const port2 = await freePort();
    const srv2 = startServer(port2, dbs2.todo, dbs2.board);
    await waitForHTTP(`http://127.0.0.1:${port2}/board`);
    await page.goto(`http://127.0.0.1:${port2}/`);

    const before2 = await snapshot(page);
    const stale = before2.find((c) => c.column === 'To Do').cards[1];
    deleteCardOutOfBand(dbs2.board, stale.id);
    // The page has not re-rendered: it still shows the now-missing card —
    // this is precisely the stale state the scenario's Given describes.
    assert.strictEqual(await cardById(page, stale.id).count(), 1,
      'the stale page must still show the card between the out-of-band delete and the edit');

    await page.evaluate(() => { window.__kw3NoReloadMarker = 'alive'; });
    await editCard(page, cardById(page, stale.id), 'Text nobody will ever read');

    const banner = page.locator(MISSING_BANNER);
    await banner.waitFor({ state: 'visible' });
    assert.match((await banner.textContent()).trim(), /no such card/,
      'the missing-card failure must state the contract\'s reason verbatim');
    assert.strictEqual(await page.evaluate(() => window.__kw3NoReloadMarker), 'alive',
      'the missing-card failure is a fragment swap, not a page reload');
    // The card is gone from the page: no node with its identifier, and no
    // phantom remnant — an empty edit form, orphan error, or ghost title
    // would all keep the stale card visible in some form.
    assert.strictEqual(await cardById(page, stale.id).count(), 0,
      'the missing card must be gone from the page after the stated failure');
    assert.strictEqual(await page.locator(`.edit-form[hx-patch$="/cards/${stale.id}"]`).count(), 0,
      'no form targeting the missing card may survive the re-render');
    const after2 = await snapshot(page);
    const json2 = boardView(await getJSON(`http://127.0.0.1:${port2}/board`));
    assert.deepStrictEqual(after2, json2,
      'after the missing-card failure the page does not mirror the contract\'s truth');
    assert.ok(!after2.some((col) => col.cards.some((card) => card.id === stale.id)),
      'GET /board still contains the card the page said was missing');

    // The statement never outlives the reload: reloading reads the same
    // truth and shows no banner, still agreeing with a fresh GET /board.
    await page.reload();
    assert.strictEqual(await page.locator(MISSING_BANNER).count(), 0,
      'the missing-card statement must not survive a reload');
    assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(`http://127.0.0.1:${port2}/board`)),
      'the reloaded page does not agree with a fresh GET /board');

    await stopServer(srv2);
    console.log('scenario 2 (operation on missing card — edit leg): OK');

    // --- Rejections on edit (card ui/06's stated failure surface): a blank
    // title is refused with the contract's "title is required" AT the
    // editing card over unchanged truth — the card's original text stays
    // and survives a reload; 501 characters is refused there stating the
    // character limit, the board untouched.
    const dbs3 = tmpPaths('edit-reject');
    run(seedBin, ['--db', dbs3.board, '--titles', 'Write the weekly report,Call the plumber']);
    const port3 = await freePort();
    const srv3 = startServer(port3, dbs3.todo, dbs3.board);
    await waitForHTTP(`http://127.0.0.1:${port3}/board`);
    await page.goto(`http://127.0.0.1:${port3}/`);

    const before3 = await snapshot(page);
    const jsonBefore3 = boardView(await getJSON(`http://127.0.0.1:${port3}/board`));
    const blankTarget = before3.find((c) => c.column === 'To Do').cards[0];

    await editCard(page, cardById(page, blankTarget.id), '   ');
    const blankError = page.locator(EDIT_ERROR(blankTarget.id));
    await blankError.waitFor({ state: 'visible' });
    assert.strictEqual((await blankError.textContent()).trim(), 'title is required',
      'the blank edit must state the contract\'s empty-text refusal verbatim at the card');
    const afterBlank = await snapshot(page);
    assert.deepStrictEqual(afterBlank, before3,
      'a rejected blank edit must leave every card\'s text, id, position, and column untouched');
    assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port3}/board`)), jsonBefore3,
      'a rejected blank edit must leave GET /board unchanged');

    // The second rejection leg targets the other card, so the blank-refusal
    // error element cannot shadow it: over-long text states the limit at
    // that card, board untouched.
    const longTarget = before3.find((c) => c.column === 'To Do').cards[1];
    await editCard(page, cardById(page, longTarget.id), 'a'.repeat(501));
    const longError = page.locator(EDIT_ERROR(longTarget.id));
    await longError.waitFor({ state: 'visible' });
    const limitText = (await longError.textContent()).trim();
    assert.match(limitText, /500/,
      `the over-long edit must state the character limit, got: ${JSON.stringify(limitText)}`);
    assert.match(limitText, /character/i,
      `the refusal must name the character limit, got: ${JSON.stringify(limitText)}`);
    assert.deepStrictEqual(await snapshot(page), before3,
      'a rejected over-long edit must leave every card untouched');
    assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port3}/board`)), jsonBefore3,
      'a rejected over-long edit must leave GET /board unchanged');

    // Reload-ready truth: after the rejections the original titles are the
    // stored state — a reload shows them, unchanged, with no refusal left over.
    await page.reload();
    const reloaded3 = await snapshot(page);
    assert.deepStrictEqual(reloaded3, before3,
      'after reload the rejected edits must show the original titles intact');
    assert.strictEqual(await page.locator('.error-text').count(), 0,
      'a rejected edit\'s stated reason must not survive a reload');

    await stopServer(srv3);
    console.log('scenario 3 (edit rejections stated at the card): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW3 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
