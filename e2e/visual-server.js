// KW7 visual globalSetup — the servers the visual spec points at, built and
// seeded by the lane itself (same self-contained pattern as the kw1..kw6
// scripts): build the binaries, seed a FRESH deterministic board file through
// the e2e seed tool (fixed titles, fixed column placements — plain, unicode
// and long-title cards spread over all three columns), start TWO servers on
// FREE ports: one over the seeded board (populated surfaces, gallery) and
// one over a never-created board file (the stated-empty page, the same
// "Start without todo data" shape the kw1 lane asserts), hand both base URLs
// to the workers via env. Returns a teardown that stops both servers and
// removes the temp dir.
//
// The --todo-db flags point at never-created paths — the kw1..kw6 convention
// that the superseded todos.db beside the repo must never leak into a lane.
const assert = require('assert');
const { spawn, spawnSync } = require('child_process');
const net = require('net');
const fs = require('fs');
const os = require('os');
const path = require('path');
const http = require('http');

const repoRoot = path.join(__dirname, '..');
const binDir = path.join(__dirname, 'bin');
const serverBin = process.env.BIN || path.join(binDir, 'todo');
const seedBin = process.env.SEED_BIN || path.join(binDir, 'seed');

// Deterministic seed: fixed titles (plain, unicode, and one long title near
// the length limit), fixed column placements (the seed tool's index flags).
// Commas are the seed tool's list separator, so no title contains one.
// Every column ends up populated: To Do gets the plain + long-title cards,
// In Progress two cards, Done the unicode card — so the done treatment is
// exercised with the widest font coverage.
const SEED_TITLES = [
  'Buy milk',
  'Pay electricity bill',
  'Walk the dog',
  'Read 20 pages',
  'Unicode tytuł: żółw ✓ café — 日本語 🎉 ₹500',
  'Extended wrap-test title: ' + 'walrus paragraph '.repeat(27), // 485 chars
];
const SEED_IN_PROGRESS = '2,3';
const SEED_DONE = '4';

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

// startServer launches one composed server over one board file (never-
// created path = empty board; the store creates it at open, kw1's shape)
// and resolves its base URL once /board answers.
function startServer(boardDb) {
  return freePort().then((port) => {
    const proc = spawn(serverBin, [
      '--addr', `127.0.0.1:${port}`,
      '--board-db', boardDb,
      '--todo-db', boardDb + '.todos-absent',
    ], { stdio: ['ignore', 'ignore', 'pipe'] });
    let stderr = '';
    proc.stderr.on('data', (d) => (stderr += d));
    return waitForHTTP(`http://127.0.0.1:${port}/board`)
      .then(() => ({ proc, base: `http://127.0.0.1:${port}`, stderr }))
      .catch((err) => {
        proc.kill('SIGKILL');
        return assert.fail(`${err.message}; server stderr:\n${stderr}`);
      });
  });
}

module.exports = async function visualGlobalSetup() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'kanban-visual-'));
  const seededDb = path.join(dir, 'board-seeded.db');
  const emptyDb = path.join(dir, 'board-empty.db'); // never seeded, never pre-opened
  run(seedBin, [
    '--db', seededDb,
    '--titles', SEED_TITLES.join(','),
    '--in-progress', SEED_IN_PROGRESS,
    '--done', SEED_DONE,
  ]);

  const seeded = await startServer(seededDb);
  let empty;
  try {
    empty = await startServer(emptyDb);
  } catch (err) {
    seeded.proc.kill('SIGKILL');
    throw err;
  }

  process.env.VISUAL_BASE_URL = seeded.base;
  process.env.VISUAL_EMPTY_URL = empty.base;

  return async function teardown() {
    for (const srv of [seeded, empty]) {
      srv.proc.kill('SIGTERM');
      await new Promise((resolve) => srv.proc.on('exit', resolve));
    }
    fs.rmSync(dir, { recursive: true, force: true });
  };
};
