// KW9 e2e — assignments end-to-end in the browser, re-executing the parent
// scenarios "Assign a user to a card", "Unassign a card", "Unknown user is
// refused", "Done card assignment is frozen" and "Assignments survive
// restart" (workplans/workplan_kanban_application.md; cards server/09,
// api/12–13, ui/14–15 landed through lanes 1–3).
//
// Five scenarios, every surface an assignment touches:
//   assign — a card showing no assignee: the edit band's select offers
//     exactly Unassigned + the fixed roster (the page's roster agrees with
//     GET /users verbatim, in order); picking a name is ONE PATCH to the
//     card endpoint, the chip shows the name with no reload, a reload keeps
//     it, and GET /board carries the name for that card.
//   unassign — a card assigned to a user (staged through the contract
//     seam): picking "Unassigned" is one PATCH carrying the empty field,
//     the chip is gone with no reload, and the contract reports the card
//     as null.
//   unknown user — the scenario's submission forced through the card seam
//     (the select cannot offer an off-cast name, so page.request is the
//     honest "name outside the roster is submitted"): 422 with the
//     contract's "unknown user" stated AT the card, the rendered board
//     untouched, GET /board repeating the pre-probe truth.
//   done freeze — an assigned Done card (staged assign-then-move, the
//     honest shape over HTTP) renders chip only — no band, no select —
//     while an unassigned Done card renders no chip; forcing set OR clear
//     through the seam answers 422 "cannot edit a done card" with the
//     board unchanged; dragging the assigned card out of Done (one move
//     PATCH) returns the band with the select preselected to the carried
//     name, and picking another name there succeeds — one PATCH, chip
//     shows it.
//   restart — assignments staged over the contract seam across all
//     columns (assigned To Do / In Progress / Done, one set-then-cleared,
//     one never assigned); SIGTERM must EXIT the process with code 0
//     (kw6's restart pattern), a respawn on the same data file shows every
//     chip exactly as before — DOM card-for-card and in agreement with a
//     fresh GET /board's name-or-null.
//
// Forced seam probes are form-encoded PATCHes to /ui/cards/{id}: handleEdit
// reads r.FormValue, so a JSON body would state no fields at all — form is
// the probe shape that actually reaches the contract (kw8's reasoning,
// unchanged). Given-state staging (assignments that must pre-exist a
// scenario's When) rides the api seam PATCH /cards/{id} out-of-band, the
// same staging role kw6 gave its sqlite3 writes: the browser legs below
// only ever perform the scenario's own action.
//
// Flow: build binaries -> seed fresh temp boards -> start server -> legs ->
// SIGTERM teardown; fresh server per scenario, mirroring the other lanes.
// The drag gesture is kw5's mouse choreography (kw8's geometry).
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

// The fixed cast — the scenario Given's roster, pinned against GET /users
// so the page can never quietly diverge from the contract's cast.
const ROSTER = ['Ada', 'Grace', 'Alan', 'Barbara', 'Linus'];

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

// stageJSON PATCHes the api seam (/cards/{id}) out-of-band — the Given-state
// staging channel: assignments that must EXIST before a scenario's When ride
// the contract there, so no browser leg ever stages its own precondition.
// Requires 2xx and returns the card the contract answered.
function stageJSON(port, id, body) {
  return new Promise((resolve, reject) => {
    const req = http.request(
      {
        host: '127.0.0.1',
        port,
        path: `/cards/${id}`,
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
      },
      (res) => {
        let data = '';
        res.on('data', (d) => (data += d));
        res.on('end', () => {
          if (res.statusCode < 200 || res.statusCode >= 300) {
            return reject(new Error(`staging PATCH /cards/${id} ${body} -> ${res.statusCode} ${data}`));
          }
          try {
            resolve(JSON.parse(data));
          } catch (err) {
            reject(err);
          }
        });
      }
    );
    req.on('error', reject);
    req.end(body);
  });
}

// The composition root opens only the board data file; --todo-db points at a
// never-created path so no repo-side todos.db can leak in (kw6's discipline).
function startServer(port, boardDb) {
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--board-db', boardDb, '--todo-db', boardDb + '.todos-absent'], {
    stdio: ['ignore', 'ignore', 'pipe'],
  });
  let stderr = '';
  proc.stderr.on('data', (d) => (stderr += d));
  proc._stderr = () => stderr;
  return proc;
}

// stopServer SIGTERMs and resolves with the exit {code, signal} — the
// process-exit observation kw6's restart pattern asserts on.
function stopServer(proc) {
  proc.kill('SIGTERM');
  return new Promise((resolve) => proc.on('exit', (code, signal) => resolve({ code, signal })));
}

function tmpPaths(name) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kanban-e2e-'));
  return { board: path.join(dir, name + '.kanban.db'), dir };
}

// The whole board as the DOM shows it: per fixed column, its cards
// top-to-bottom as {title, id, done, assignee} — the other lanes' shape
// extended with the chip's text (null where no chip element exists). DOM
// and GET /board compare directly: chip = contract is one deepStrictEqual.
async function snapshot(page) {
  return page.locator('#board > section.column').evaluateAll((sections) =>
    sections.map((section) => ({
      column: section.dataset.column,
      cards: [...section.querySelectorAll('ul.column__cards > li.card')].map((li) => {
        const chip = li.querySelector('.card__assignee');
        return {
          title: li.querySelector('.card__title').textContent,
          id: Number(li.dataset.card),
          done: li.classList.contains('card--done'),
          assignee: chip ? chip.textContent : null,
        };
      }),
    }))
  );
}

// The board as the contract answers it, shaped like snapshot(). The wire's
// name-or-null decodes straight through: the contract carries assignee on
// EVERY card (api/13), so no defaulting is needed.
function boardView(json) {
  return json.columns.map((c) => ({
    column: c.title,
    cards: c.cards.map((card) => ({
      title: card.title,
      id: card.id,
      done: c.title === 'Done',
      assignee: card.assignee,
    })),
  }));
}

function cardById(page, id) {
  return page.locator(`li.card[data-card="${id}"]`);
}

// PATCH instrumentation (kw5/kw8's pattern): one PATCH per accepted drop,
// one PATCH per band save, nothing else.
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

// A band save is exactly one change request (the one-update-operation
// decision): one PATCH at the card's own seam, its form carrying the wanted
// assignee spelling (a name, or the empty value the Unassigned option holds).
function assertOneAssignRequest(log, id, assignee, where) {
  assert.strictEqual(log.patches.length, 1,
    `${where}: expected EXACTLY ONE PATCH per band save, saw ${log.patches.length}: ${JSON.stringify(log.patches.map((p) => p.url))}`);
  assert.ok(log.patches[0].url.endsWith(`/ui/cards/${id}`),
    `${where}: the save's PATCH addressed ${log.patches[0].url}, want the card endpoint for card ${id}`);
  const form = new URLSearchParams(log.patches[0].body);
  assert.ok(form.has('assignee'),
    `${where}: the save form carries no assignee field at all: ${log.patches[0].body}`);
  assert.strictEqual(form.get('assignee'), assignee,
    `${where}: the save form carried assignee=${JSON.stringify(form.get('assignee'))}, want ${JSON.stringify(assignee)}: ${log.patches[0].body}`);
}

// kw5's mouse choreography, kw8's geometry.
async function dragTo(page, source, target) {
  await source.hover();
  await page.mouse.down();
  const src = await source.boundingBox();
  await page.mouse.move(src.x + 16, src.y + 4, { steps: 4 });
  await page.mouse.move(target.x, target.y, { steps: 10 });
  await page.mouse.up();
}

async function pointOn(locator, fy) {
  const box = await locator.boundingBox();
  return { x: box.x + box.width / 2, y: box.y + box.height * fy };
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

// The chip's text for one card, right now ('waitForChip' polls it until a
// name or until the chip is gone at null).
async function chipText(page, id) {
  return page.evaluate((cardId) => {
    const chip = document.querySelector(`li.card[data-card="${cardId}"] .card__assignee`);
    return chip ? chip.textContent : null;
  }, id);
}

async function waitForChip(page, id, name, timeout = 10000) {
  const start = Date.now();
  for (;;) {
    if ((await chipText(page, id)) === name) return;
    if (Date.now() - start > timeout) throw new Error(`timeout waiting for card ${id} chip ${JSON.stringify(name)}`);
    await page.waitForTimeout(25);
  }
}

// The band's assignee select: open it through the card's Edit control and
// hand back the locator (kw8's band choreography, select edition).
async function openAssigneeSelect(page, cardLi) {
  await cardLi.locator('.card__edit').click();
  const select = cardLi.locator('.edit-form select[name="assignee"]');
  await select.waitFor({ state: 'visible' });
  return select;
}

// The select's offer: option texts top-to-bottom, plus the one selected
// value (the current state preselected — the ui/15 contract the page reads).
async function selectOffer(select) {
  const options = await select.locator('option').allTextContents();
  const selected = await select.evaluate((el) => [...el.selectedOptions].map((o) => o.value));
  assert.strictEqual(selected.length, 1, 'the assignee select must carry exactly one selected option');
  return { options, selected: selected[0] };
}

// ————— scenario: "Assign a user to a card" —————
// Given the fixed roster and a card showing no assignee. When the user picks
// a roster name in the edit band's select: the card shows the name (one
// PATCH — the one update operation), after reload it still shows it, and
// GET /board carries the assignee for that card. The select's offer is the
// roster the contract serves — Unassigned first, then GET /users' names in
// order — so the page can never invent a cast of its own.
async function scenarioAssign(browser) {
  const dbs = tmpPaths('assign');
  run(seedBin, [
    '--db', dbs.board,
    '--titles', 'Write the weekly report,Call the plumber,Ship v1.2',
    '--done', '2',
  ]);
  const port = await freePort();
  const srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);
  const log = instrumentPatches(page);

  // Given (roster half): the contract's own roster endpoint serves exactly
  // the cast the scenario names, in its order.
  const roster = await getJSON(`http://127.0.0.1:${port}/users`);
  assert.deepStrictEqual(roster.users, ROSTER,
    'GET /users must serve the fixed cast in contract order — the select mirrors this');

  const before = await snapshot(page);
  const target = before.find((c) => c.column === 'To Do').cards[0];
  const li = cardById(page, target.id);
  assert.strictEqual(await chipText(page, target.id), null,
    "the scenario's Given: the card shows no assignee");

  // When: open the edit band and pick a roster name. The select offers
  // exactly Unassigned + the roster and preselects the card's current
  // (unassigned) state.
  await page.evaluate(() => { window.__kw9NoReloadMarker = 'alive'; });
  const select = await openAssigneeSelect(page, li);
  const offer = await selectOffer(select);
  assert.deepStrictEqual(offer.options, ['Unassigned', ...ROSTER],
    'the select must offer exactly Unassigned then the roster in contract order');
  assert.strictEqual(offer.selected, '',
    'the select of an unassigned card must preselect Unassigned');
  log.reset();
  await select.selectOption({ value: 'Grace' });
  await li.locator('.edit-form button.save').click();
  await waitForChip(page, target.id, 'Grace');

  // Then: exactly one PATCH for the pick, at the card's own seam, the form
  // carrying the name — and no page reload: the chip lands on the swap.
  assertOneAssignRequest(log, target.id, 'Grace', 'the assign save');
  assert.strictEqual(await page.evaluate(() => window.__kw9NoReloadMarker), 'alive',
    'the assign must update the page without a reload');

  // Then: the board contract carries the assignee for that card, and the
  // chip mirrors it card-for-card.
  const truthJSON = await getJSON(`http://127.0.0.1:${port}/board`);
  assert.deepStrictEqual(await snapshot(page), boardView(truthJSON),
    'after the assign the page does not agree with GET /board');
  const carried = boardView(truthJSON).flatMap((c) => c.cards).find((c) => c.id === target.id);
  assert.strictEqual(carried.assignee, 'Grace', 'the contract must carry the chosen name for the card');

  // Then: after reload the card still shows that name, in agreement with
  // a fresh read.
  await page.reload();
  const after = await snapshot(page);
  assert.strictEqual(after.flatMap((c) => c.cards).find((c) => c.id === target.id).assignee, 'Grace',
    'after reload the card must still show the chosen name');
  assert.deepStrictEqual(after, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'after reload the page does not agree with a fresh GET /board');

  await stopServer(srv);
  await page.close();
}

// ————— scenario: "Unassign a card" —————
// Given a card assigned to a user (staged through the contract seam, so the
// browser leg is only the scenario's own action). When the user chooses
// "Unassigned": the card shows nobody — chip element gone, one PATCH whose
// form carries the empty field — and the board contract reports the card
// as unassigned (null).
async function scenarioUnassign(browser) {
  const dbs = tmpPaths('unassign');
  run(seedBin, ['--db', dbs.board, '--titles', 'Water the office plants,Read the standards doc']);
  const port = await freePort();
  const srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  await stageJSON(port, 1, '{"assignee": "Alan"}');

  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);
  const log = instrumentPatches(page);

  // Given (card half): the page shows the card assigned to Alan, chip and
  // contract agreeing — the assigned state the scenario starts from.
  assert.strictEqual(await chipText(page, 1), 'Alan', 'the Given card must show its assignee');
  const truthBefore = boardView(await getJSON(`http://127.0.0.1:${port}/board`));
  assert.strictEqual(truthBefore.flatMap((c) => c.cards).find((c) => c.id === 1).assignee, 'Alan',
    'the Given card must be assigned in the contract');

  await page.evaluate(() => { window.__kw9NoReloadMarker = 'alive'; });
  const select = await openAssigneeSelect(page, cardById(page, 1));
  assert.strictEqual((await selectOffer(select)).selected, 'Alan',
    'the select of an assigned card must preselect its current assignee');
  log.reset();
  await select.selectOption({ value: '' }); // the select's "Unassigned"
  await cardById(page, 1).locator('.edit-form button.save').click();
  await waitForChip(page, 1, null);

  // Then: one PATCH carrying the empty field (the contract's null clear),
  // no reload, and the card shows nobody — no chip element at all.
  assertOneAssignRequest(log, 1, '', 'the unassign save');
  assert.strictEqual(await page.evaluate(() => window.__kw9NoReloadMarker), 'alive',
    'the unassign must update the page without a reload');
  assert.strictEqual(await cardById(page, 1).locator('.card__assignee').count(), 0,
    'the unassigned card must render no chip element at all');

  // Then: the board contract reports the card as unassigned.
  const truthJSON = await getJSON(`http://127.0.0.1:${port}/board`);
  assert.deepStrictEqual(await snapshot(page), boardView(truthJSON),
    'after the unassign the page does not agree with GET /board');
  const cleared = boardView(truthJSON).flatMap((c) => c.cards).find((c) => c.id === 1);
  assert.strictEqual(cleared.assignee, null, 'the contract must report the card as unassigned (null)');

  await page.reload();
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'after reload the page does not agree with a fresh GET /board');

  await stopServer(srv);
  await page.close();
}

// ————— scenario: "Unknown user is refused" —————
// The select can only offer roster names, so "a name outside the roster is
// submitted" is the scenario's submission forced through the card seam —
// page.request straight to the server, the page never moved. The seam
// mirrors the contract: 422 with "unknown user", stated AT the card, the
// card and board unchanged — DOM untouched and a fresh GET /board repeating
// the pre-probe truth.
async function scenarioUnknown(browser) {
  const dbs = tmpPaths('unknown-user');
  run(seedBin, ['--db', dbs.board, '--titles', 'Tune the dashboard,File the expense claim']);
  const port = await freePort();
  const srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);

  const before = await snapshot(page);
  const truthBefore = boardView(await getJSON(`http://127.0.0.1:${port}/board`));
  assert.deepStrictEqual(before, truthBefore, "the lane's DOM and contract baselines must agree before the probe");
  const victim = before.find((c) => c.column === 'To Do').cards[0];
  assert.strictEqual(await chipText(page, victim.id), null, 'the Given card starts unassigned');

  // An off-cast name submitted as its assignee — forced past the select at
  // the seam, form-encoded so the probe really states the fields (kw8's
  // reasoning: handleEdit reads r.FormValue).
  const resp = await page.request.patch(`http://127.0.0.1:${port}/ui/cards/${victim.id}`, {
    form: { title: victim.title, assignee: 'Nobody' },
  });
  assert.strictEqual(resp.status(), 422,
    `a forced off-cast assignee must answer 422, saw ${resp.status()}`);
  const refused = await resp.text();
  assert.ok(refused.includes('unknown user'),
    `the seam must state the contract's reason verbatim, got: ${JSON.stringify(refused.slice(0, 400))}`);
  assert.ok(refused.includes(`id="edit-error-${victim.id}"`),
    'the refusal must be stated AT the card — the answer carries the card\'s edit-error paragraph');

  // Then: the card is unchanged and the board is unchanged — the probe
  // never touched the page, and the contract repeats the pre-probe truth;
  // the refused card still shows no chip.
  assert.deepStrictEqual(await snapshot(page), before,
    'the forced PATCH must leave the rendered board untouched — nothing about the probe should move the page');
  assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port}/board`)), truthBefore,
    'a refused unknown-user submission must leave GET /board unchanged');
  assert.strictEqual(await chipText(page, victim.id), null,
    'the refused card must still show nobody — the submission changed nothing');

  await stopServer(srv);
  await page.close();
}

// ————— scenario: "Done card assignment is frozen" —————
// An assigned Done card (staged assign-then-move: the freeze refuses
// assigning a card already in Done, so assign-then-move is the honest shape
// of that Given over HTTP) renders the chip ONLY — no band, therefore no
// select, no Edit — while an unassigned Done card renders no chip at all.
// A change trying to SET the assignee of one Done card and one trying to
// CLEAR another's are both forced through the seam: 422 with the contract's
// "cannot edit a done card" stated at the card, the board unchanged. Then
// the unlock: one drag out of Done returns the band with the select
// preselected to the carried name, and picking another name there succeeds
// — one PATCH, the chip shows it.
async function scenarioDoneFreeze(browser) {
  const dbs = tmpPaths('done-assign');
  run(seedBin, [
    '--db', dbs.board,
    '--titles', 'Refactor the parser,Migrate the registry,Archive the old tickets',
    '--done', '2',
  ]);
  const port = await freePort();
  const srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  await stageJSON(port, 2, '{"assignee": "Linus"}'); // assigned while in To Do...
  await stageJSON(port, 2, '{"column": "done"}'); // ...then moved into Done

  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);
  const log = instrumentPatches(page);

  const view = await snapshot(page);
  const truthBefore = boardView(await getJSON(`http://127.0.0.1:${port}/board`));
  assert.deepStrictEqual(view, truthBefore, "the lane's DOM and contract baselines must agree before the probes");
  const frozenAssigned = view.find((c) => c.column === 'Done').cards.find((c) => c.id === 2);
  const frozenPlain = view.find((c) => c.column === 'Done').cards.find((c) => c.id === 3);
  assert.strictEqual(frozenAssigned.assignee, 'Linus', 'the assigned Done card must show its chip');
  assert.strictEqual(frozenPlain.assignee, null, 'the unassigned Done card must show no chip');

  // The chip is the ONLY assignee surface Done carries: the assigned card
  // renders the chip but no band, no select, no Edit — while delete and
  // drag, neither an editing surface, stay (kw8's freeze affordances).
  const assignedLi = cardById(page, 2);
  assert.strictEqual(await assignedLi.locator('.card__assignee').count(), 1,
    'the assigned Done card renders its chip — display is not an edit affordance');
  for (const doneLi of [cardById(page, 2), cardById(page, 3)]) {
    assert.strictEqual(await doneLi.locator('.edit-form').count(), 0,
      'a Done card renders no edit band, so it renders no assignee select either');
    assert.strictEqual(await doneLi.locator('select[name="assignee"]').count(), 0,
      'no assignee control may exist on a Done card');
    assert.strictEqual(await doneLi.locator('.card__edit').count(), 0,
      'a Done card shows no edit control (the KW8 affordance rule, unchanged)');
    assert.strictEqual(await doneLi.locator('.card__delete').count(), 1,
      'a Done card keeps its delete control — deleting is not editing');
    assert.strictEqual(await doneLi.getAttribute('draggable'), 'true',
      'a Done card stays draggable — dragging out of Done is the unlock');
  }

  // When a change tries to SET the assignee (forced through the seam, an
  // off-band submission the frozen page itself cannot make): refused, board
  // unchanged.
  const setResp = await page.request.patch(`http://127.0.0.1:${port}/ui/cards/3`, {
    form: { title: frozenPlain.title, assignee: 'Grace' },
  });
  assert.strictEqual(setResp.status(), 422,
    `a forced assignee SET on a Done card must answer 422, saw ${setResp.status()}`);
  const setRefused = await setResp.text();
  assert.ok(setRefused.includes('cannot edit a done card'),
    `the seam must state the freeze verbatim, got: ${JSON.stringify(setRefused.slice(0, 400))}`);
  assert.ok(setRefused.includes('id="edit-error-3"'),
    'the refusal must be stated AT the card');
  assert.deepStrictEqual(await snapshot(page), view,
    'the forced SET must leave the rendered board untouched');
  assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port}/board`)), truthBefore,
    'a refused assignee SET must leave GET /board unchanged');

  // When a change tries to CLEAR the assignee of the assigned Done card:
  // refused the same way — one freeze rule, both directions.
  const clearResp = await page.request.patch(`http://127.0.0.1:${port}/ui/cards/2`, {
    form: { title: frozenAssigned.title, assignee: '' },
  });
  assert.strictEqual(clearResp.status(), 422,
    `a forced assignee CLEAR on a Done card must answer 422, saw ${clearResp.status()}`);
  const clearRefused = await clearResp.text();
  assert.ok(clearRefused.includes('cannot edit a done card'),
    `the seam must state the freeze verbatim, got: ${JSON.stringify(clearRefused.slice(0, 400))}`);
  assert.ok(clearRefused.includes('id="edit-error-2"'),
    'the refusal must be stated AT the card');
  assert.deepStrictEqual(await snapshot(page), view,
    'the forced CLEAR must leave the rendered board untouched');
  assert.deepStrictEqual(boardView(await getJSON(`http://127.0.0.1:${port}/board`)), truthBefore,
    'a refused assignee CLEAR must leave GET /board unchanged');
  assert.strictEqual(await chipText(page, 2), 'Linus',
    'the refused clear must leave the chip exactly as it was');

  // Then (unlock): the assignee becomes changeable again once the card is
  // out of Done. One drag out (a move — the freeze never touches moves),
  // the band and its select back on the swapped markup, preselected to the
  // name the card carried.
  await page.evaluate(() => { window.__kw9NoReloadMarker = 'alive'; });
  log.reset();
  await dragTo(page, assignedLi, await pointOn(page.locator('#column-to-do li.card').first(), 0.75));
  await waitForCardIn(page, 2, 'To Do');
  const moveBody = assertOneMoveRequest(log, 2, 'the unlock drag out of Done');
  assert.strictEqual(moveBody.column, 'To Do',
    'the unlock drag must carry the target column — it is a move, not an edit');

  const movedLi = cardById(page, 2);
  assert.strictEqual(await movedLi.locator('select[name="assignee"]').count(), 1,
    'the select must reappear with the band once the card is out of Done');
  const select = await openAssigneeSelect(page, movedLi);
  assert.strictEqual((await selectOffer(select)).selected, 'Linus',
    'the returned select must preselect the assignee the card carried through the moves');

  // Assigning there succeeds: one PATCH, the chip shows the new name, no
  // reload; a reload agrees with a fresh GET /board.
  log.reset();
  await select.selectOption({ value: 'Alan' });
  await movedLi.locator('.edit-form button.save').click();
  await waitForChip(page, 2, 'Alan');
  assertOneAssignRequest(log, 2, 'Alan', 'the post-unlock assign');
  assert.strictEqual(await page.evaluate(() => window.__kw9NoReloadMarker), 'alive',
    'the unlock drag and the post-unlock assign must never reload the page');
  const after = await snapshot(page);
  assert.deepStrictEqual(after, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'after the post-unlock assign the page does not agree with GET /board');
  await page.reload();
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'after reload the page does not agree with a fresh GET /board');

  await stopServer(srv);
  await page.close();
}

// ————— scenario: "Assignments survive restart" —————
// kw6's restart pattern with assignees as the asserted state: assignments
// staged over the contract seam across every column — assigned in To Do,
// assigned in In Progress, an assigned card in Done (staged assign-then-
// move, the honest shape), one card assigned then cleared, one never
// assigned. SIGTERM must EXIT the process with code 0, not merely stop
// answering (kw6's stop step verbatim in duty); a respawn on the same board
// data file shows every card's assignee EXACTLY as before — chip card-for-
// card, DOM ≡ fresh GET /board, and the contract's name-or-null per
// identifier against an independent expectation derived from the staging,
// so the comparison cannot pass vacuously.
async function scenarioRestart(browser) {
  const dbs = tmpPaths('assign-restart');
  run(seedBin, [
    '--db', dbs.board,
    '--titles', [
      'Rotate the audit keys',    // To Do — gets Ada
      'Renew the SSL certificate',// To Do — assigned then cleared: null
      'Draft the launch note',    // In Progress — gets Grace
      'Review the security scan', // In Progress — gets Linus, then moves to Done
      'Ship v1.2',                // Done — never assigned: null
    ].join(','),
    '--in-progress', '2,3',
    '--done', '4',
  ]);
  const port = await freePort();
  let srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);

  // The Given: assignments over HTTP across all columns, Done included,
  // unassigned cards included (semantics of the staging operations, not
  // observed output — the same independence kw6's restart leg keeps).
  await stageJSON(port, 1, '{"assignee": "Ada"}');
  await stageJSON(port, 2, '{"assignee": "Barbara"}');
  await stageJSON(port, 2, '{"assignee": null}');
  await stageJSON(port, 3, '{"assignee": "Grace"}');
  await stageJSON(port, 4, '{"assignee": "Linus"}');
  await stageJSON(port, 4, '{"column": "done"}');

  const expected = [
    { column: 'To Do', cards: [
      { id: 1, assignee: 'Ada', done: false },
      { id: 2, assignee: null, done: false },
    ] },
    { column: 'In Progress', cards: [
      { id: 3, assignee: 'Grace', done: false },
    ] },
    { column: 'Done', cards: [
      { id: 5, assignee: null, done: true },
      { id: 4, assignee: 'Linus', done: true },
    ] },
  ];
  const stripTitles = (view) =>
    view.map((col) => ({ column: col.column, cards: col.cards.map((c) => ({ id: c.id, assignee: c.assignee, done: c.done })) }));

  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);
  const before = await snapshot(page);
  assert.deepStrictEqual(stripTitles(before), expected,
    'the staging did not produce the assignment layout the operations dictate (before the restart, so the restart cannot be blamed)');
  assert.deepStrictEqual(before, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'before the restart the page does not mirror GET /board');

  // When: the process restarts with the same data files — SIGTERM exits,
  // respawn at the same address and the same board file.
  const exit = await stopServer(srv);
  assert.deepStrictEqual(exit, { code: 0, signal: null },
    `SIGTERM shutdown must exit the process with code 0, saw ${JSON.stringify(exit)}; stderr: ${srv._stderr()}`);
  srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);

  // Then: every card reports exactly the same assignee state as before.
  const page2 = await browser.newPage();
  await page2.goto(`http://127.0.0.1:${port}/`);
  const revived = await snapshot(page2);
  assert.deepStrictEqual(stripTitles(revived), expected,
    'after the restart a chip differs from the assignee state the board was left in');
  const revivedJSON = await getJSON(`http://127.0.0.1:${port}/board`);
  assert.deepStrictEqual(revived, boardView(revivedJSON),
    'after the restart the page does not mirror a fresh GET /board');

  // The contract side, exact and independent of the DOM: the name-or-null
  // the fresh GET /board carries for every identifier, against the staging
  // table above.
  const perId = {};
  for (const col of boardView(revivedJSON)) {
    for (const card of col.cards) perId[card.id] = card.assignee;
  }
  assert.deepStrictEqual(perId, { 1: 'Ada', 2: null, 3: 'Grace', 4: 'Linus', 5: null },
    'the restarted contract does not report every card\'s assignee exactly as before');

  await stopServer(srv);
  await page.close();
  await page2.close();
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const failures = [];

  try {
    await scenarioAssign(browser);
    console.log('scenario 1 (assign a user to a card — select offers Unassigned + GET /users roster, one PATCH carries the name, chip shows it with no reload, reload and GET /board persist it): OK');

    await scenarioUnassign(browser);
    console.log('scenario 2 (unassign a card — pick Unassigned, one PATCH with the empty field, chip element gone with no reload, contract reports null): OK');

    await scenarioUnknown(browser);
    console.log('scenario 3 (unknown user is refused — off-cast name forced at the seam: 422 "unknown user" stated at the card, DOM and GET /board unchanged): OK');

    await scenarioDoneFreeze(browser);
    console.log('scenario 4 (done card assignment is frozen — Done renders chip only, forced set AND clear refused 422 with the board unchanged, drag-out returns the select preselected and assigning succeeds): OK');

    await scenarioRestart(browser);
    console.log('scenario 5 (assignments survive restart — assignments across all columns incl. Done-assigned and cleared, SIGTERM exit 0, respawn chips exactly as before, contract name-or-null exact): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW9 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
