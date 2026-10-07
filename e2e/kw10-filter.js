// KW10 e2e — the filter over the board, end-to-end in the browser, re-executing
// the parent scenarios "Filter board by user", "All and Unassigned filters"
// and "Move under filter keeps whole-board truth"
// (workplans/workplan_kanban_application.md; cards ui/16–18 landed through
// lanes 1–2).
//
// Three scenarios, every filter surface the page carries:
//   filter by user — the dropdown offers exactly All users, the roster in
//     contract order, Unassigned; picking a name is a plain GET navigation to
//     ?assignee=<name>, so only that user's cards render, each in its own
//     column in its own order, an empty column under the filter states its
//     filter-naming empty, a reload replays the URL's view and the back
//     button returns the full board — no client-side filter state exists to
//     disagree with the address.
//   All and Unassigned — from an active user filter, "All users" drops the
//     parameter and shows the full board; "Unassigned" shows exactly the
//     cards nobody is assigned to and carries ?assignee=unassigned; the
//     filtered empty states its wording ("Nothing for <name> here", and the
//     sentinel's "Nothing unassigned here").
//   move under filter — hidden cards are interleaved among the visible ones;
//     a drop between two visible cards, and a drop into a column, each issue
//     EXACTLY ONE PATCH whose body carries the slot+within PAIR (never an
//     absolute position); the filtered view updates; clearing the filter
//     shows the true interleaved whole-board truth — hidden cards unmoved in
//     their relative order, DOM ≡ GET /board.
//
// The kw9 chip scenarios stay kw9's: no parent scenario describes assigning
// WHILE filtered, so this lane invents no such leg — kw9's own legs ride the
// unfiltered page exactly as kw9 shipped them (the kw10 chrome sits beside
// them untouched).
//
// Given-state (assignments, a card in Done) rides the contract seam
// (PATCH /cards/{id} out-of-band, kw9's staging role): no browser leg stages
// its own precondition. Flow: build binaries -> seed fresh temp boards ->
// start server -> legs -> SIGTERM teardown; fresh server per scenario,
// mirroring the other lanes. The drag gesture is kw5's mouse choreography
// (kw8/kw9's geometry). CommonJS so `require('playwright')` resolves via
// NODE_PATH=$(npm root -g).
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

// The fixed cast — the roster the dropdown's middle options mirror, pinned
// against GET /users so the page can never quietly diverge from the contract.
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
// staging channel (kw9's role): assignments and Done placements that must
// EXIST before a scenario's When ride the contract, so no browser leg stages
// its own precondition. Requires 2xx.
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

function stopServer(proc) {
  proc.kill('SIGTERM');
  return new Promise((resolve) => proc.on('exit', (code, signal) => resolve({ code, signal })));
}

function tmpPaths(name) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kanban-e2e-'));
  return { board: path.join(dir, name + '.kanban.db'), dir };
}

// The whole board as the DOM shows it (kw9's snapshot shape: a filtered page
// simply renders fewer cards — the same comparison works under either view).
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

// Each column's stated filtered-empty text (null where the column renders
// cards instead of the emptiness) — card ui/17's treatment as the DOM carries
// it, read from the one column__empty site.
async function columnEmpties(page) {
  return page.locator('#board > section.column').evaluateAll((sections) =>
    sections.map((section) => {
      const p = section.querySelector('.column__empty');
      return { column: section.dataset.column, empty: p ? p.textContent : null };
    })
  );
}

// The board as the contract answers it, shaped like snapshot(). The filtered
// read (GET /board?assignee=…, card api/14's presence branch) shapes through
// the same function — filtered DOM ≡ filtered contract is one deepStrictEqual.
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

const idsOf = (view) => view.map((col) => ({ column: col.column, ids: col.cards.map((c) => c.id) }));

function cardById(page, id) {
  return page.locator(`li.card[data-card="${id}"]`);
}

// PATCH instrumentation (kw5/kw8/kw9's pattern): the drag's one fetch site is
// the page's only per-drop request, so counting PATCHes counts operations.
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

// A drop under an active filter is exactly ONE PATCH at the move endpoint
// whose WHOLE BODY is the slot+within pair (card ui/18): the payload asserts
// as a whole — slot alone, within alone, or an absolute position alongside
// either cannot pass.
function assertOneFilteredMoveRequest(log, id, want, where) {
  assert.strictEqual(log.patches.length, 1,
    `${where}: expected EXACTLY ONE PATCH per accepted drop, saw ${log.patches.length}: ${JSON.stringify(log.patches)}`);
  assert.ok(log.patches[0].url.endsWith(`/ui/cards/${id}/move`),
    `${where}: the drop's PATCH addressed ${log.patches[0].url}, want the move endpoint for card ${id}`);
  assert.deepStrictEqual(JSON.parse(log.patches[0].body), want,
    `${where}: the drop under the filter must carry the slot+within pair as its whole body, saw ${log.patches[0].body}`);
}

// kw5's mouse choreography, kw8/kw9's geometry.
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

// Poll until a column renders EXACTLY these cards, in this order — the
// filtered view after a swap (the drop's answer is a filtered re-read).
async function waitForVisibleOrder(page, columnTitle, ids, timeout = 10000) {
  const start = Date.now();
  for (;;) {
    const got = await page.evaluate((title) => {
      const section = [...document.querySelectorAll('#board > section.column')]
        .find((s) => s.dataset.column === title);
      if (!section) return null;
      return [...section.querySelectorAll('ul.column__cards > li.card')].map((li) => Number(li.dataset.card));
    }, columnTitle);
    if (got && got.length === ids.length && got.every((v, i) => v === ids[i])) return;
    if (Date.now() - start > timeout) {
      throw new Error(`timeout waiting for column ${JSON.stringify(columnTitle)} to show ${JSON.stringify(ids)}, saw ${JSON.stringify(got)}`);
    }
    await page.waitForTimeout(25);
  }
}

// The filter dropdown's offer: option texts and values top-to-bottom plus the
// selected value. The roster middle-leg must equal GET /users exactly, with
// "All users" first and the "unassigned" sentinel last.
async function filterOffer(page) {
  const select = page.locator('#board-filter');
  const options = await select.locator('option').evaluateAll((os) =>
    os.map((o) => ({ value: o.value, text: o.textContent }))
  );
  const selected = await select.evaluate((el) => [...el.selectedOptions].map((o) => o.value));
  assert.strictEqual(selected.length, 1, 'the filter select must carry exactly one selected option');
  return { options, selected: selected[0] };
}

function assertFilterOffer(offer, where) {
  assert.deepStrictEqual(offer.options.map((o) => o.text), ['All users', ...ROSTER, 'Unassigned'],
    `${where}: the dropdown must offer exactly All users, each roster name, Unassigned`);
  assert.deepStrictEqual(offer.options.map((o) => o.value), ['', ...ROSTER, 'unassigned'],
    `${where}: the dropdown values must be the empty value, the roster names, the unassigned sentinel`);
}

// Picking a filter value IS navigation (card ui/16): the page's own mechanism
// is a plain GET to '?assignee=' + encodeURIComponent(value) — "All users"
// to the plain '/'. Wait for that address, then assert the URL carries (or
// no longer carries) the parameter, verbatim.
async function pickFilter(page, port, value, where) {
  const expected = value
    ? `http://127.0.0.1:${port}/?assignee=${encodeURIComponent(value)}`
    : `http://127.0.0.1:${port}/`;
  await Promise.all([
    page.waitForURL(expected),
    page.locator('#board-filter').selectOption({ value }),
  ]);
  assert.strictEqual(page.url(), expected,
    `${where}: after picking the filter, the URL must be ${expected}, saw ${page.url()}`);
}

const boardURL = (port, filter) =>
  filter ? `http://127.0.0.1:${port}/board?assignee=${encodeURIComponent(filter)}`
         : `http://127.0.0.1:${port}/board`;

// ————— scenario: "Filter board by user" —————
// Given cards assigned to different users (Ada, Grace, Barbara, Linus across
// the columns) and some to nobody. When the user picks a name in the filter
// control: only that user's cards show — each in its own column, in its own
// order — the control offers exactly All users, each roster name, Unassigned,
// and the filter rides the URL, so a reload replays the filtered view and the
// back button returns the full board. The pick itself is the page's plain GET
// navigation, which is what puts these views into browser history at all.
async function scenarioFilterByUser(browser) {
  const dbs = tmpPaths('filter-user');
  run(seedBin, [
    '--db', dbs.board,
    '--titles', [
      'Draft the query plan',      // To Do — Grace
      'Rotate the audit keys',     // To Do — Ada
      'Update the runbook',        // To Do — Grace
      'Water the office plants',   // To Do — nobody (the Given's "some to nobody")
      'Ship v1.2',                 // In Progress — Grace
      'Review the security scan',  // In Progress — Barbara
      'Archive the old tickets',   // Done — Linus (staged assign-then-move)
    ].join(','),
    '--in-progress', '4,5',
  ]);
  const port = await freePort();
  const srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  await stageJSON(port, 1, '{"assignee": "Grace"}');
  await stageJSON(port, 2, '{"assignee": "Ada"}');
  await stageJSON(port, 3, '{"assignee": "Grace"}');
  await stageJSON(port, 5, '{"assignee": "Grace"}');
  await stageJSON(port, 6, '{"assignee": "Barbara"}');
  await stageJSON(port, 7, '{"assignee": "Linus"}'); // while still in To Do...
  await stageJSON(port, 7, '{"column": "done"}');    // ...then moved into Done

  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);

  // Given (roster half): the contract's roster endpoint serves the cast the
  // dropdown's middle options must mirror, in order.
  const roster = await getJSON(`http://127.0.0.1:${port}/users`);
  assert.deepStrictEqual(roster.users, ROSTER,
    'GET /users must serve the fixed cast in contract order — the dropdown mirrors this');

  // Given (board half): the unfiltered baseline — DOM ≡ contract, and the
  // staging really did produce mixed assignees with unassigned cards.
  const full = await snapshot(page);
  assert.deepStrictEqual(full, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'the unfiltered baseline page does not agree with GET /board');
  const assignees = new Set(full.flatMap((c) => c.cards).map((c) => c.assignee));
  assert.ok(assignees.has(null) && ['Ada', 'Grace', 'Barbara', 'Linus'].every((n) => assignees.has(n)),
    `the Given needs cards assigned to different users AND some to nobody, saw ${JSON.stringify([...assignees])}`);

  // The control's offer is exactly: All users, each roster name, Unassigned —
  // and it preselects the no-filter state.
  assertFilterOffer(await filterOffer(page), 'the unfiltered dropdown');

  // When: pick Grace in the filter control.
  await pickFilter(page, port, 'Grace', 'picking Grace');

  // Then: the URL carries the chosen filter.
  assert.ok(page.url().includes('?assignee=Grace'),
    `the chosen filter must be part of the page URL, saw ${page.url()}`);

  // Then: only her cards, each in its own column in its own order — DOM ≡
  // the contract's filtered read, and every visible chip names Grace.
  const filtered = await snapshot(page);
  assert.deepStrictEqual(filtered, boardView(await getJSON(boardURL(port, 'Grace'))),
    'under the Grace filter the page does not agree with GET /board?assignee=Grace');
  const herCards = filtered.flatMap((c) => c.cards);
  assert.ok(herCards.length > 0 && herCards.every((c) => c.assignee === 'Grace'),
    `only Grace's cards may be visible, saw ${JSON.stringify(herCards.map((c) => [c.id, c.assignee]))}`);
  // Columns preserved, relative order preserved: all three render, and her
  // To Do cards keep their stored order (card 1 before card 3).
  assert.deepStrictEqual(filtered.map((c) => c.column), ['To Do', 'In Progress', 'Done'],
    'the filtered view keeps the three fixed columns');
  assert.deepStrictEqual(filtered.find((c) => c.column === 'To Do').cards.map((c) => c.id), [1, 3],
    "her column's cards keep their own relative order under the filter");
  assert.deepStrictEqual((await columnEmpties(page)).find((c) => c.column === 'Done').empty,
    'Nothing for Grace here',
    'an empty column under the filter states the filter-naming empty (the stated treatment)');
  assert.deepStrictEqual((await filterOffer(page)).selected, 'Grace',
    'the dropdown reads its selected state from the URL like everything else');

  // Then: reload keeps the filtered view — the URL owns the filter.
  await page.reload();
  assert.ok(page.url().includes('?assignee=Grace'), 'reload must keep the filter in the URL');
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(boardURL(port, 'Grace'))),
    'after reload the filtered page does not agree with the filtered contract read');

  // Then: the back button returns the full board — history navigation, and
  // the page mechanism making that work is the plain GET (kw10 carries no
  // client-side filter state: each history entry re-renders from its URL).
  await page.goBack();
  assert.strictEqual(page.url(), `http://127.0.0.1:${port}/`,
    'the back button must return to the plain address — the filter left with the history entry');
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'after back the page must show the full board, in agreement with GET /board');
  // The dropdown's rendered markup reads the URL again: the '/' entry's
  // markup marks All users as selected. (The live selected PROPERTY after a
  // history traversal is the browser's form-state restore, not page state —
  // no scenario owns it; the page owns markup, and markup comes from the
  // address, which is what makes reload/back replay the view.)
  const marked = await page.locator('#board-filter').evaluate((el) =>
    [...el.querySelectorAll('option[selected]')].map((o) => o.value)
  );
  assert.deepStrictEqual(marked, [''],
    'back at the full board the dropdown markup rendered from the plain URL marks All users');

  await stopServer(srv);
  await page.close();
}

// ————— scenario: "All and Unassigned filters" —————
// Given a board under an active user filter (reached by picking Grace — the
// scenario names no way in but the control, and the control navigates). When
// the user picks "All users": the full board is back and the filter has
// LEFT the URL — the plain '/'. When the user instead picks "Unassigned":
// exactly the cards with no assignee show and the URL carries the sentinel.
// The filtered empties state their wording — "Nothing for <name> here" under
// a name filter, "Nothing unassigned here" under the sentinel.
async function scenarioAllAndUnassigned(browser) {
  const dbs = tmpPaths('all-unassigned');
  run(seedBin, [
    '--db', dbs.board,
    '--titles', [
      'Draft the regression suite', // To Do — Grace
      'File the expense report',    // To Do — nobody
      'Sign the vendor contract',   // To Do — nobody
      'Tune the alert rules',       // In Progress — Alan
      'Review the security scan',   // In Progress — Barbara
      'Archive the old tickets',    // Done — nobody
    ].join(','),
    '--in-progress', '3,4',
    '--done', '5',
  ]);
  const port = await freePort();
  const srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  await stageJSON(port, 1, '{"assignee": "Grace"}');
  await stageJSON(port, 4, '{"assignee": "Alan"}');
  await stageJSON(port, 5, '{"assignee": "Barbara"}');

  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);

  // The Given: a board under an active user filter. Grace has To Do cards
  // and none elsewhere — so the name filter also stages the stated empty.
  await pickFilter(page, port, 'Grace', 'entering the scenario filter');
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(boardURL(port, 'Grace'))),
    'the scenario Given must start under a live Grace filter agreeing with the contract');
  const emptyUnderGrace = await columnEmpties(page);
  assert.deepStrictEqual(emptyUnderGrace.find((c) => c.column === 'In Progress').empty,
    'Nothing for Grace here', 'a column with none of her cards states "Nothing for <name> here"');
  assert.deepStrictEqual(emptyUnderGrace.find((c) => c.column === 'Done').empty,
    'Nothing for Grace here', 'every filtered-empty column carries the filter-naming treatment');

  // When the user picks "All users": the full board is shown again and the
  // filter leaves the URL — the plain '/', no query at all.
  await pickFilter(page, port, '', "picking 'All users'");
  assert.strictEqual(new URL(page.url()).search, '',
    `'All users' must drop the parameter from the URL entirely, saw ${page.url()}`);
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    "'All users' must show the full board again, in agreement with GET /board");
  assert.deepStrictEqual((await filterOffer(page)).selected, '',
    "the dropdown must read the All-users state back off the URL");

  // When the user instead picks "Unassigned" — again from an active user
  // filter, the scenario's Given standing under the When:
  await pickFilter(page, port, 'Grace', 'restoring the active user filter');
  await pickFilter(page, port, 'unassigned', "picking 'Unassigned'");
  assert.ok(page.url().includes('?assignee=unassigned'),
    `the sentinel filter must ride the URL, saw ${page.url()}`);

  // Then: exactly the cards with no assignee — DOM ≡ the contract's
  // sentinel-filtered read, and no card on the page carries a chip.
  const unassignedView = await snapshot(page);
  assert.deepStrictEqual(unassignedView, boardView(await getJSON(boardURL(port, 'unassigned'))),
    'under the Unassigned filter the page does not agree with GET /board?assignee=unassigned');
  const visible = unassignedView.flatMap((c) => c.cards);
  assert.ok(visible.length > 0 && visible.every((c) => c.assignee === null),
    `only unassigned cards may be visible, saw ${JSON.stringify(visible.map((c) => [c.id, c.assignee]))}`);
  assert.deepStrictEqual(idsOf(unassignedView), [
    { column: 'To Do', ids: [2, 3] },        // the two nobody-cards, in order
    { column: 'In Progress', ids: [] },      // assigned to Alan + Barbara: empty under the sentinel
    { column: 'Done', ids: [6] },            // the Done card nobody holds
  ], 'the Unassigned filter must show exactly the unassigned cards, columns preserved');
  assert.deepStrictEqual((await columnEmpties(page)).find((c) => c.column === 'In Progress').empty,
    'Nothing unassigned here', "the sentinel's empty column states the sentinel's stated wording");

  await stopServer(srv);
  await page.close();
}

// ————— scenario: "Move under filter keeps whole-board truth" —————
// Given a filter active that hides some cards, with the hidden cards
// INTERLEAVED among the visible ones (To Do alternates Ada/Grace cards, so
// under the Grace filter the hidden Ada cards sit between her cards). When
// the user drags a visible card to a slot between two visible cards, or into
// a column: each drop lands at that slot RELATIVE TO THE VISIBLE CARDS — one
// PATCH whose body is the slot+within pair, never an absolute position — the
// filtered view updates, and clearing the filter shows the true interleaved
// whole-board order: the hidden cards unmoved in their relative order, DOM ≡
// GET /board.
async function scenarioMoveUnderFilter(browser) {
  const dbs = tmpPaths('move-filtered');
  run(seedBin, [
    '--db', dbs.board,
    '--titles', [
      'Rotate the audit keys',    // To Do pos 0 — Ada   (hidden, interleaved)
      'Draft the launch note',    // To Do pos 1 — Grace (visible)
      'Tune the dashboard',       // To Do pos 2 — Ada   (hidden, interleaved)
      'Migrate the registry',     // To Do pos 3 — Grace (visible — the dragged card)
      'Ship v1.2',                // In Progress — Grace (visible)
      'File the expense claim',   // In Progress — Ada   (hidden)
      'Archive the old tickets',  // Done — Linus (staged assign-then-move; hidden)
    ].join(','),
    '--in-progress', '4,5',
  ]);
  const port = await freePort();
  const srv = startServer(port, dbs.board);
  await waitForHTTP(`http://127.0.0.1:${port}/board`);
  await stageJSON(port, 1, '{"assignee": "Ada"}');
  await stageJSON(port, 2, '{"assignee": "Grace"}');
  await stageJSON(port, 3, '{"assignee": "Ada"}');
  await stageJSON(port, 4, '{"assignee": "Grace"}');
  await stageJSON(port, 5, '{"assignee": "Grace"}');
  await stageJSON(port, 6, '{"assignee": "Ada"}');
  await stageJSON(port, 7, '{"assignee": "Linus"}');
  await stageJSON(port, 7, '{"column": "done"}');

  const page = await browser.newPage();
  await page.goto(`http://127.0.0.1:${port}/`);
  const log = instrumentPatches(page);

  // The Given: filter active, hidden cards interleaved among the visible —
  // the DOM carries only [2,4] of a stored To Do order [1,2,3,4]: hidden 1
  // before visible 2, hidden 3 between the visible pair.
  await pickFilter(page, port, 'Grace', 'activating the hiding filter');
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(boardURL(port, 'Grace'))),
    'the filtered view must agree with the contract read before any drop');
  assert.deepStrictEqual(idsOf(await snapshot(page)), [
    { column: 'To Do', ids: [2, 4] },
    { column: 'In Progress', ids: [5] },
    { column: 'Done', ids: [] },
  ], 'the filter must hide the interleaved cards the Given states');
  assert.deepStrictEqual(idsOf(boardView(await getJSON(`http://127.0.0.1:${port}/board`))), [
    { column: 'To Do', ids: [1, 2, 3, 4] },
    { column: 'In Progress', ids: [5, 6] },
    { column: 'Done', ids: [7] },
  ], 'the hidden cards must keep their interleaved stored places under the hidden view');

  await page.evaluate(() => { window.__kw10NoReloadMarker = 'alive'; });

  // Leg 1 — drag a visible card to a slot between visible cards (here: card
  // 4 to the slot before card 2, the column front among the visible pair).
  // The contract's slot counts only matching cards after the card's removal,
  // so slot 0 = Grace cards before it: none.
  log.reset();
  await dragTo(page, cardById(page, 4), await pointOn(cardById(page, 2), 0.25));
  await waitForVisibleOrder(page, 'To Do', [4, 2]);
  assertOneFilteredMoveRequest(log, 4, { column: 'To Do', slot: 0, within: 'Grace' },
    'the filtered drop to a slot between visible cards');
  assert.strictEqual(await page.evaluate(() => window.__kw10NoReloadMarker), 'alive',
    'the filtered drop must update the view without a reload');
  assert.deepStrictEqual(await snapshot(page), boardView(await getJSON(boardURL(port, 'Grace'))),
    'after the slot drop the filtered view must agree with the filtered contract read');
  // Whole-board truth while still hidden: the contract resolves slot 0 to
  // the column's front (store semantics), so card 4 sits at index 0 ahead of
  // the hidden Ada 1 — hidden cards keep their relative order (1 before 3),
  // pushed only by the insertion ahead of them, never reordered.
  assert.deepStrictEqual(idsOf(boardView(await getJSON(`http://127.0.0.1:${port}/board`)))
    .find((c) => c.column === 'To Do').ids, [4, 1, 2, 3],
    'the slot drop must land at the visible slot with hidden cards keeping their relative places');

  // Leg 2 — drag a visible card INTO a column: card 4 into In Progress at
  // the slot after the one visible Grace card there (slot 1 among matching
  // cards; the hidden Ada card there keeps its place behind the matching
  // pair, the slot count never seeing it).
  log.reset();
  await dragTo(page, cardById(page, 4), await pointOn(cardById(page, 5), 0.75));
  await waitForVisibleOrder(page, 'In Progress', [5, 4]);
  await waitForVisibleOrder(page, 'To Do', [2]);
  assertOneFilteredMoveRequest(log, 4, { column: 'In Progress', slot: 1, within: 'Grace' },
    'the filtered drop into a column');

  // Then: clearing the filter shows the TRUE interleaved order. The
  // expectation is derived from the staging plus the contract's slot
  // semantics (remove-then-slot among matching rows), independent of any DOM
  // or contract read — so agreement below cannot pass vacuously: To Do keeps
  // its interleaving [Ada 1, Grace 2, Ada 3] around the departed visible
  // card, and In Progress holds [Grace 5, Grace 4, Ada 6] with the hidden
  // Ada card unmoved in its relative place behind the visible pair.
  await pickFilter(page, port, '', 'clearing the filter');
  const truth = [
    { column: 'To Do', ids: [1, 2, 3] },
    { column: 'In Progress', ids: [5, 4, 6] },
    { column: 'Done', ids: [7] },
  ];
  const cleared = await snapshot(page);
  assert.deepStrictEqual(idsOf(cleared), truth,
    'with the filter cleared the board must show the true interleaved order with hidden cards unmoved');
  assert.deepStrictEqual(cleared, boardView(await getJSON(`http://127.0.0.1:${port}/board`)),
    'the cleared DOM must agree card-for-card with GET /board — the whole board is the truth the filter hid');
  const todoHidden = cleared.find((c) => c.column === 'To Do').cards.filter((c) => c.assignee === 'Ada').map((c) => c.id);
  assert.deepStrictEqual(todoHidden, [1, 3], 'the hidden cards keep their relative order through the filtered moves');

  await stopServer(srv);
  await page.close();
}

async function main() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const browser = await chromium.launch();
  const failures = [];

  try {
    await scenarioFilterByUser(browser);
    console.log('scenario 1 (filter board by user — dropdown offers exactly All users + roster + Unassigned, pick Grace: only her cards in their columns and order, URL carries ?assignee=Grace, reload keeps the view, back returns the full board): OK');

    await scenarioAllAndUnassigned(browser);
    console.log('scenario 2 (all and Unassigned filters — from a Grace filter All users drops the parameter to the plain board, Unassigned shows exactly the nobody-cards with ?assignee=unassigned, filtered empties state "Nothing for <name> here" / "Nothing unassigned here"): OK');

    await scenarioMoveUnderFilter(browser);
    console.log('scenario 3 (move under filter keeps whole-board truth — hidden cards interleaved, each drop one PATCH with the slot+within pair as its whole body, filtered view updates, clearing shows the true interleaved order, hidden cards unmoved, DOM ≡ GET /board): OK');
  } catch (err) {
    failures.push(err);
  } finally {
    await browser.close();
  }

  if (failures.length) {
    for (const f of failures) console.error('E2E FAILURE:', f.message);
    process.exit(1);
  }
  console.log('KW10 e2e: all scenarios passed');
}

main().catch((err) => {
  console.error('E2E CRASH:', err);
  process.exit(1);
});
