// W11 visual globalSetup — the server the visual spec points at, built and
// seeded by the lane itself (same self-contained pattern as the w1..w10
// scripts): build the binaries, seed a FRESH temp data file through the
// existing seed tool with a fixed deterministic title set (unicode +
// long-title + done mix), start the app on a FREE port, wait for HTTP, and
// hand the base URL to the workers via env. Returns a teardown that stops
// the server and removes the temp dir.
//
// Note: the screenshots are all taken on /__components, which renders from
// compile-time fixtures (no store, no clock, no randomness — see
// ui/stories.go), so the seeded data file cannot leak into any pixel. It is
// seeded anyway so the lane starts the app exactly like the app is started
// in production: with a real, deterministic db behind it.
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
// the 500-char limit), fixed done indexes. Commas are the seed tool's list
// separator, so no title contains one.
const SEED_TITLES = [
  'Buy milk',
  'Pay electricity bill',
  'Walk the dog',
  'Read 20 pages',
  'Unicode tytuł: żółw ✓ café — 日本語 🎉 ₹500',
  'Extended wrap-test title: ' + 'walrus paragraph '.repeat(27), // 485 chars
];
const SEED_DONE = '0,2,4';

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

module.exports = async function visualGlobalSetup() {
  fs.mkdirSync(binDir, { recursive: true });
  run('go', ['build', '-o', serverBin, './cmd/todo']);
  run('go', ['build', '-o', seedBin, './e2e/testdata']);

  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'todo-visual-'));
  const db = path.join(dir, 'visual.db');
  run(seedBin, ['-db', db, '-titles', SEED_TITLES.join(','), '-done', SEED_DONE]);

  const port = await freePort();
  const proc = spawn(serverBin, ['--addr', `127.0.0.1:${port}`, '--db', db], {
    stdio: ['ignore', 'ignore', 'pipe'],
  });
  let stderr = '';
  proc.stderr.on('data', (d) => (stderr += d));
  try {
    await waitForHTTP(`http://127.0.0.1:${port}/todos`);
  } catch (err) {
    proc.kill('SIGKILL');
    assert.fail(`${err.message}; server stderr:\n${stderr}`);
  }

  process.env.VISUAL_BASE_URL = `http://127.0.0.1:${port}`;

  return async function teardown() {
    proc.kill('SIGTERM');
    await new Promise((resolve) => proc.on('exit', resolve));
    fs.rmSync(dir, { recursive: true, force: true });
  };
};
