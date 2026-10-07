// KW8 e2e — the done freeze end-to-end in the browser, re-executing the
// parent scenario "Edit of a done card is rejected" (scenario 15, appended
// by the amendment 2026-10-07 to the one-update-operation decision: the
// contract-level done freeze restoring the todo app's frozen-text rule —
// workplan_kanban_application.md; ledger: dependencies_kanban.md's KW8
// supersession note; cards board/08, api/05, ui/06 with their amendments).
//
// One scenario, two legs, every surface the freeze touches:
//   refusal — Given a card in Done: the page shows NO edit affordance on it
//     (delete + drag hooks stay — neither is an editing surface), while
//     every card outside Done still carries its Edit control; forcing the
//     card's own seam anyway — PATCH /ui/cards/{id} carrying a title, sent
//     through page.request straight to the server so the page never moved —
//     answers 422 with the contract's "cannot edit a done card" verbatim,
//     stated at the card (the refusal fragment carries the card's
//     edit-error paragraph), and the board is unchanged: the untouched DOM
//     and a fresh GET /board still agree with the pre-probe truth.
//   unlock — drag the card out of Done (one move PATCH — Move never sees a
//     title, so the drag is the only unlock), its edit control comes back
//     on the swapped markup, the edit lands EXACTLY ONE PATCH to the card
//     endpoint, the title updates with no page reload, and a later reload
//     agrees with a fresh GET /board.
//
// The forced PATCH carries its title form-encoded: handleEdit reads
// r.FormValue("title"), the native form submit's shape — an honest
// "user submits a text change" forced past the absent control. A JSON body
// at that seam would state no title at all (form parsing ignores JSON), so
// form encoding is the probe that actually reaches the freeze.
//
// Flow: build binaries -> seed a board whose Done column holds one card ->
// start server -> both legs -> SIGTERM teardown. Fresh server per scenario,
// mirroring the other lanes; the drag gesture is kw5's mouse choreography.
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
// temp file so no run touches a file beside the repo; the KW6 migration flag
// (--todo-db) points at a never-created path — these lanes state no todo
// data file, and a superseded todos.db beside the repo must not leak in.
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

// The whole board as the DOM shows it: per fixed column, its cards
// top-to-bottom as {title, id, done}. Same shape the other lanes assert, so
// DOM and GET /board can be compared directly.
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

// The board as the contract answers it, shaped like snapshot().
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

// PATCH instrumentation (kw5's pattern): the unlock leg's request counting —
// one PATCH per accepted drop, one PATCH per edit, nothing else.
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
// addressed at the dragged card's move endpoint, body parses.
function assertOneMoveRequest(log, id, where) {
  assert.strictEqual(log.patches.length, 1,
    `${where}: expected EXACTLY ONE PATCH per accepted drop, saw ${log.patches.length}: ${JSON.stringify(log.patches)}`);
  assert.ok(log.patches[0].url.endsWith(`/ui/cards/${id}/move`),
    `${where}: the drop's PATCH addressed ${log.patches[0].url}, want the move endpoint for card ${id}`);
  return JSON.parse(log.patches[0].body);
}

// kw5's mouse choreography: chromium turns hover + mouse.down + stepped
// moves into dragstart/dragenter/dragover and the release into drop.
async function dragTo(page, source, target) {
  await source.hover();
  await page.mouse.down();
  const src = await source.boundingBox();
  await page.mouse.move(src.x + 16, src.y + 4, { steps: 4 });
  await page.mouse.move(target.x, target.y, { steps: 10 });
  await page.mouse.up();
}

// A point at fraction `fy` of an element's height, horizontally centered.
async function pointOn(locator, fy) {
  const box = await locator.boundingBox();
  return { x: box.x + box.width / 2, y: box.y + box.height * fy };
}

// Wait until the swap landed the card in the named column.
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

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const page = await browser.newPage();
  const failures = [];

  try {
    // --- Scenario: "Edit of a done card is rejected" (parent scenario 15) —
    // Given a card in the Done column: no edit affordance on the page, the
    // forced PATCH at the card seam refused with the stated error, the board
    // unchanged; and the card becomes editable after it is moved out of Done.
    const dbs = tmpPaths('done-freeze');
    run(seedBin, [
      '--db', dbs.board,
      '--titles', 'Plan the quarterly review,Update the runbook,Ship the hotfix',
      '--done', '2', // 'Ship the hotfix' — the card the scenario freezes
    ]);
    const port = await freePort();
    const srv = startServer(port, dbs.board);
    await waitForHTTP(`http://127.0.0.1:${port}/board`);
    await page.goto(`http://127.0.0.1:${port}/`);
    const log = instrumentPatches(page);

    const before = await snapshot(page);
    const doneCard = before.find((c) => c.column === 'Done').cards[0];
    const doneLi = cardById(page, doneCard.id);

    // Then (page half): a Done card shows no edit control and no edit band,
    // while delete and drag — not editing surfaces — stay on it.
    assert.strictEqual(await doneLi.locator('.card__edit').count(), 0,
      'a Done card must render no edit control (done freeze — parent scenario 15)');
    assert.strictEqual(await doneLi.locator('.edit-form').count(), 0,
      'a Done card must render no edit band (done freeze — parent scenario 15)');
    assert.strictEqual(await doneLi.locator('.card__delete').count(), 1,
      'a Done card keeps its delete control — deleting is not editing');
    assert.strictEqual(await doneLi.getAttribute('draggable'), 'true',
      'a Done card stays draggable — dragging out of Done is the unlock');
    // The absence is the freeze's, not a broken render: every card OUTSIDE
    // Done still carries its Edit control.
    for (const col of before.filter((c) => c.column !== 'Done')) {
      for (const card of col.cards) {
        assert.strictEqual(await cardById(page, card.id).locator('.card__edit').count(), 1,
          `card ${card.id} in ${col.column} must still carry its edit control`);
      }
    }

    // When/Then (seam half): the user's text change forced through the
    // card's own endpoint anyway — page.request goes straight to the server,
    // so this is the scenario's submission past an absent affordance, not a
    // page interaction. The seam answers 422 mirroring the contract and
    // states its reason verbatim AT the card's edit-error slot.
    const truthBefore = boardView(await getJSON(`http://127.0.0.1:${port}/board`));
    assert.deepStrictEqual(before, truthBefore, 'the lane\'s DOM and contract baselines must agree before the probe');
    const resp = await page.request.patch(`http://127.0.0.1:${port}/ui/cards/${doneCard.id}`, {
      form: { title: 'Retitled behind the freeze' },
    });
    assert.strictEqual(resp.status(), 422,
      `a forced title PATCH on a Done card must answer 422, saw ${resp.status()}`);
    const refused = await resp.text();
    assert.ok(refused.includes('cannot edit a done card'),
      `the seam must state the contract's reason verbatim, got: ${JSON.stringify(refused.slice(0, 400))}`);
    assert.ok(refused.includes(`id="edit-error-${doneCard.id}"`),
      'the refusal must be stated AT the card — the answer carries the card\'s edit-error paragraph');

    // Then: the board is unchanged. The probe never touched the page, so
    // the rendered board must sit exactly where it was, and the contract
    // still answers the pre-probe truth.
    assert.deepStrictEqual(await snapshot(page), before,
      'the forced PATCH must leave the rendered board untouched — nothing about the probe should move the page');
    assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port}/board`)), truthBefore,
      'a refused done edit must leave GET /board unchanged');

    // Then (unlock): moving the card out of Done makes it editable again.
    // One drag, one move PATCH (Move never sees a title — the freeze never
    // touches it), the edit control back on the swapped markup.
    await page.evaluate(() => { window.__kw8NoReloadMarker = 'alive'; });
    log.reset();
    await dragTo(page, doneLi, await pointOn(page.locator('#column-to-do li.card').first(), 0.75));
    await waitForCardIn(page, doneCard.id, 'To Do');
    const moveBody = assertOneMoveRequest(log, doneCard.id, 'the unlock drag out of Done');
    assert.strictEqual(moveBody.column, 'To Do',
      'the unlock drag must carry the target column — it is a move, not an edit');
    const movedLi = cardById(page, doneCard.id);
    assert.strictEqual(await movedLi.locator('.card__edit').count(), 1,
      'the card moved out of Done must regain its edit control');

    // The now-editable card edits normally: band prefilled, Save exactly one
    // PATCH to the card endpoint, title swapped in place with no reload.
    await movedLi.locator('.card__edit').click();
    const band = movedLi.locator('.edit-form input.input[name=title]');
    await band.waitFor({ state: 'visible' });
    assert.strictEqual(await band.inputValue(), doneCard.title,
      'the band on the unfrozen card must open prefilled with its existing text');
    const renamed = 'Ship the hotfix (revised)';
    log.reset();
    await band.fill(renamed);
    await movedLi.locator('.edit-form button.save').click();
    await movedLi.locator('.card__title')
      .waitForFunction((el, text) => el.textContent === text, renamed, { timeout: 10000 });
    assert.strictEqual(log.patches.length, 1,
      `the edit after the unlock must be exactly one PATCH, saw ${log.patches.length}: ${JSON.stringify(log.patches.map((p) => p.url))}`);
    assert.ok(log.patches[0].url.endsWith(`/ui/cards/${doneCard.id}`),
      `the unfrozen edit must address the card endpoint: ${log.patches[0].url}`);
    assert.strictEqual(await page.evaluate(() => window.__kw8NoReloadMarker), 'alive',
      'the drag and the unfrozen edit must never reload the page');
    const after = await snapshot(page);
    const moved = after.find((c) => c.column === 'To Do').cards.find((c) => c.id === doneCard.id);
    assert.strictEqual(moved.title, renamed, 'the unfrozen card must show its new title');
    assert.strictEqual(moved.done, false,
      'the unfrozen card lost the done treatment with the Done column (done is membership)');
    await page.reload();
    assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
      'after reload the page does not agree with a fresh GET /board');

    await stopServer(srv);
    console.log('scenario (edit of a done card is rejected — no affordance, forced PATCH refused at the seam, drag-out unlocks): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW8 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
