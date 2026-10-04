// W4 e2e — edit title through Change + frozen-done rule.
//
// Browser-driven, through the UI endpoints only (PATCH /ui/todos/{id} with a
// title form field). Asserts, against a real chromium page:
//   - done rows expose no edit affordance at all (the ui/10 pin),
//   - Edit opens the band in place: visible and prefilled,
//   - Save applies the new title with no navigation: the list fragment swaps
//     in, row position and done state stay stable, the band collapses with the
//     new text rendered, and the value survives a reload,
//   - Cancel collapses the band with the text untouched,
//   - an edit saved after the todo was done behind the page's back is refused
//     visibly — the todo stays original, the row states done, and "cannot edit
//     a done todo" is stated — and no further edit control exists for it,
//   - an empty edit is refused with the stated reason, the todo itself intact
//     (a reload shows the original title back).
// CommonJS so `require('playwright')` resolves via NODE_PATH=$(npm root -g).

const assert = require("assert");
const { chromium } = require("playwright");
const { spawn } = require("child_process");
const fs = require("fs");
const http = require("http");
const os = require("os");
const path = require("path");

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function poll(predicate, { timeout = 4000, interval = 50, label = "condition" } = {}) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try {
      const ok = await predicate();
      if (ok) return;
    } catch (_) {
      /* locator not resolvable yet */
    }
    await sleep(interval);
  }
  throw new Error(`timed out after ${timeout}ms waiting for ${label}`);
}

// set the input value and dispatch a plain input event (no debounce), so a
// following click submits exactly the typed value.
const typeValue = (locator, text) =>
  locator.evaluate((el, value) => {
    el.value = value;
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }, text);

function freePort() {
  return new Promise((resolve, reject) => {
    const server = http.createServer();
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      server.close(() => resolve(port));
    });
    server.on("error", reject);
  });
}

function waitForHTTP(url, timeoutMs) {
  const start = Date.now();
  return new Promise((resolve, reject) => {
    const tick = () => {
      const req = http.get(url, (res) => {
        res.resume();
        resolve();
      });
      req.on("error", () => {
        if (Date.now() - start > timeoutMs) {
          reject(new Error(`server never answered at ${url}`));
          return;
        }
        setTimeout(tick, 50);
      });
    };
    tick();
  });
}

async function startServer(db, port) {
  const bin = path.resolve("e2e", "bin", "todo");
  const logPath = path.join(os.tmpdir(), `w4-edit-${port}.log`);
  const log = fs.openSync(logPath, "a");
  const proc = spawn(bin, ["--db", db, "--addr", `127.0.0.1:${port}`], {
    stdio: ["ignore", log, log],
    env: { ...process.env, TODO_DB: db },
  });
  let stopping = false;
  proc.on("exit", (code) => {
    if (stopping) return;
    console.error(`server exited early (code ${code}), log: ${logPath}`);
    process.exit(1);
  });
  await waitForHTTP(`http://127.0.0.1:${port}/`, 10_000);
  proc.stop = () => {
    stopping = true;
    proc.kill("SIGTERM");
    return new Promise((resolve) => proc.on("exit", resolve));
  };
  return proc;
}

async function step(name, fn) {
  try {
    await fn();
    console.log(`ok   ${name}`);
  } catch (err) {
    console.log(`FAIL ${name}\n     ${String(err).split("\n")[0]}`);
    throw err;
  }
}

(async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "todo-w4-edit-"));
  const db = path.join(dir, "todos.db");
  const seed = spawn(path.resolve("e2e", "bin", "seed"), [
    "--db", db,
    "--titles", "Write tests,Review PR,Ship release",
    "--done", "2",
  ], { stdio: "inherit" });
  const seedCode = await new Promise((resolve) => seed.on("exit", resolve));
  assert.strictEqual(seedCode, 0, `seed failed with code ${seedCode}`);

  const port = await freePort();
  const proc = await startServer(db, port);
  const browser = await chromium.launch();
  const page = await browser.newPage();
  // pageerror = uncaught JS exceptions. Network console lines from the
  // deliberate 422 refusals are expected and not page bugs.
  const pageErrors = [];
  page.on("pageerror", (err) => pageErrors.push(String(err)));

  const rows = page.locator("#todo-list li");
  // [{id, state, title}] — the exact list snapshot the page shows.
  const snapshot = () =>
    rows.evaluateAll((els) =>
      els.map((el) => ({
        id: el.id,
        state: el.dataset.state,
        title: el.querySelector(".title").textContent.trim(),
      }))
    );
  const titles = () => snapshot().then((s) => s.map((r) => r.title));
  const titleAt = (i) => textOf(rows.nth(i).locator(".title"));
  const stateAt = (i) => rows.nth(i).getAttribute("data-state");

  function textOf(locator) {
    return locator.evaluate((el) => el.textContent.trim());
  }

  try {
    await page.goto(`http://127.0.0.1:${port}/`, { waitUntil: "domcontentloaded" });

    await step("three rows render with exact titles", async () => {
      await poll(async () => (await rows.count()) === 3, { label: "three rows" });
      assert.deepStrictEqual(await titles(), ["Write tests", "Review PR", "Ship release"]);
    });

    await step("ui/10 pin: done row exposes no edit control, not-done rows do", async () => {
      assert.strictEqual(await rows.nth(2).locator("button.edit").count(), 0);
      assert.strictEqual(await rows.nth(2).locator("form.edit-form").count(), 0);
      assert.strictEqual(await rows.nth(0).locator("button.edit").count(), 1);
      assert.strictEqual(await rows.nth(1).locator("button.edit").count(), 1);
    });

    await step("ui/09 Edit opens the band in place: visible and prefilled", async () => {
      await rows.nth(0).locator("button.edit").click();
      const band = rows.nth(0).locator("form.edit-form");
      await poll(async () => band.isVisible(), { label: "edit band visible" });
      assert.strictEqual(await band.locator("input").inputValue(), "Write tests");
    });

    await step("ui/09 Save applies the new title with no navigation, position and state stable", async () => {
      const urlBefore = page.url();
      const before = await snapshot();
      await typeValue(rows.nth(0).locator("form.edit-form input"), "Write tests v2");
      await rows.nth(0).locator("form.edit-form button[type=submit]").click();

      await poll(async () => (await titleAt(0)) === "Write tests v2", { label: "saved title rendered" });
      assert.strictEqual(page.url(), urlBefore, "the page navigated on Save");
      const after = await snapshot();
      assert.strictEqual(after[0].id, before[0].id);
      assert.strictEqual(after[0].state, "not-done", "done state changed");
      assert.deepStrictEqual(after.slice(1).map((r) => r.id + ":" + r.state),
        before.slice(1).map((r) => r.id + ":" + r.state), "other rows moved");
      const band = rows.nth(0).locator("form.edit-form");
      assert.ok(!(await band.isVisible()), "the band stayed open after Save");
    });

    await step("ui/09 reload persists the edited title", async () => {
      await page.reload({ waitUntil: "domcontentloaded" });
      assert.strictEqual(await titleAt(0), "Write tests v2");
    });

    await step("ui/09 Cancel collapses the band and changes nothing", async () => {
      await rows.nth(1).locator("button.edit").click();
      const band = rows.nth(1).locator("form.edit-form");
      await poll(async () => band.isVisible(), { label: "edit band visible" });
      await typeValue(band.locator("input"), "changed then cancelled");
      await band.locator("button.cancel").click();
      await poll(async () => !(await band.isVisible()), { label: "band collapsed" });
      assert.strictEqual(await titleAt(1), "Review PR");
      await page.reload({ waitUntil: "domcontentloaded" });
      assert.strictEqual(await titleAt(1), "Review PR", "the cancel attempt changed the store");
    });

    // ui/11: mark row B done behind the stale page's back through the API —
    // the page still shows it editable.
    const idB = await rows.nth(1).evaluate((el) => el.id.replace("todo-", ""));
    await step("ui/11 stale edit refused visibly: reason stated, todo stays original", async () => {
      const res = await page.evaluate(async (id) => {
        const r = await fetch(`/todos/${id}`, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ done: true }),
        });
        return r.status;
      }, idB);
      assert.strictEqual(res, 200, "behind-the-scenes mark-done failed");

      // The stale row still offers Edit; open it and try to save a title.
      await rows.nth(1).locator("button.edit").click();
      const band = rows.nth(1).locator("form.edit-form");
      await poll(async () => band.isVisible(), { label: "edit band visible on stale row" });
      const urlBefore = page.url();
      await typeValue(band.locator("input"), "edited after done");
      await band.locator("button[type=submit]").click();

      // Refused: the row fragment swaps in stating the frozen rule.
      await poll(async () => {
        const err = rows.nth(1).locator(".edit-error");
        return (await err.count()) === 1 && (await textOf(err)).includes("cannot edit a done todo");
      }, { label: "frozen-done refusal stated" });
      assert.strictEqual(page.url(), urlBefore, "the page navigated on the refused edit");
      // The todo itself: original text kept, now rendered done.
      assert.strictEqual(await titleAt(1), "Review PR");
      assert.strictEqual(await stateAt(1), "done");
      // And from here no further edit is possible.
      assert.strictEqual(await rows.nth(1).locator("button.edit").count(), 0);
      // The other rows are untouched.
      assert.strictEqual(await titleAt(0), "Write tests v2");
    });

    await step("ui/11 a page reload shows the todo done with its original title", async () => {
      await page.reload({ waitUntil: "domcontentloaded" });
      await poll(async () => (await rows.nth(1).getAttribute("data-state")) === "done",
        { label: "two done rows" });
      assert.strictEqual(await titleAt(1), "Review PR");
      assert.strictEqual(await rows.nth(1).locator("button.edit").count(), 0);
    });

    await step("ui/12 empty edit refused with stated reason, todo intact", async () => {
      await rows.nth(0).locator("button.edit").click();
      const band = rows.nth(0).locator("form.edit-form");
      await poll(async () => band.isVisible(), { label: "edit band visible" });
      await typeValue(band.locator("input"), "");
      await band.locator("button[type=submit]").click();

      await poll(async () => {
        const err = rows.nth(0).locator(".edit-error");
        return (await err.count()) === 1 && (await textOf(err)).includes("title is required");
      }, { label: "empty-title refusal stated" });
      await page.reload({ waitUntil: "domcontentloaded" });
      assert.strictEqual(await titleAt(0), "Write tests v2", "the empty edit damaged the todo");
    });

    assert.deepStrictEqual(pageErrors, [], "uncaught page errors during the run");
    console.log("\nw4 edit e2e: PASS");
  } finally {
    await browser.close();
    await proc.stop();
    fs.rmSync(dir, { recursive: true, force: true });
  }
})().catch((err) => {
  console.error("\nw4 edit e2e: FAIL");
  console.error(err);
  process.exit(1);
});
