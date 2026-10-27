import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { extname, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const root = fileURLToPath(new URL("../dist", import.meta.url));
const types = { ".js": "application/javascript", ".css": "text/css", ".html": "text/html", ".json": "application/json", ".svg": "image/svg+xml" };
const server = createServer(async (request, response) => {
  const pathname = decodeURIComponent(new URL(request.url, "http://localhost").pathname);
  const path = resolve(root, "." + (pathname === "/" ? "/index.html" : pathname));
  if (!path.startsWith(root + sep)) { response.writeHead(403).end(); return; }
  try {
    response.setHeader("Content-Type", types[extname(path)] || "application/octet-stream");
    response.end(await readFile(path));
  } catch { response.writeHead(404).end(); }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
let browser;
try {
  browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1100, height: 800 } });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/*", (route) => route.request().url().startsWith(origin) || route.request().url().startsWith("data:")
    ? route.continue() : route.abort());
  await page.addInitScript(() => {
    const photos = Array.from({ length: 1000 }, (_, index) => ({
      id: index + 1, originalName: `photo-${index + 1}.jpg`, backupPath: `/mock/photo-${index + 1}.jpg`,
      backedUpAt: "2026-10-27 10:00:00", thumbnail: "", description: "", analysisStatus: "pending",
      analysisError: "", analyzedAt: "", tags: [],
    }));
    const listeners = new Set();
    const calls = { full: 0, delta: [], status: 0, cloud: 0 };
    const mock = window.mock = { photos, calls, delay: 100, failNext: false };
    const copy = (value) => JSON.parse(JSON.stringify(value));
    mock.emit = (id, status, tags = []) => {
      Object.assign(photos[id - 1], { analysisStatus: status, tags, description: tags.length ? "คำอธิบายภาพทดสอบ" : "" });
      for (const listener of listeners) listener({ photoId: id, status });
    };
    mock.listenerCount = () => listeners.size;
    window.runtime = { EventsOnMultiple: (_name, callback) => { listeners.add(callback); return () => listeners.delete(callback); } };
    window.go = { main: { App: {
      GetTaggedPhotos: async () => { calls.full++; return copy(photos); },
      GetTaggedPhotosByIDs: async (ids) => {
        calls.delta.push([...ids]);
        const result = copy(photos.filter((photo) => ids.includes(photo.id)));
        await new Promise((resolve) => setTimeout(resolve, mock.delay));
        if (mock.failNext) { mock.failNext = false; throw new Error("simulated temporary error"); }
        return result;
      },
      GetTagCloud: async () => {
        calls.cloud++;
        const tags = new Map();
        for (const photo of photos) for (const tag of photo.tags) {
          if (!tags.has(tag.id)) tags.set(tag.id, { ...tag, photoCount: 0 });
          tags.get(tag.id).photoCount++;
        }
        return [...tags.values()];
      },
      GetAnalysisStatus: async () => {
        calls.status++;
        const counts = { enabled: true, pending: 0, processing: 0, retrying: 0, completed: 0, failed: 0 };
        for (const photo of photos) counts[photo.analysisStatus]++;
        return counts;
      },
      RemoveTagFromPhoto: async (id, tagID) => { photos[id - 1].tags = photos[id - 1].tags.filter((tag) => tag.id !== tagID); },
      DeleteTag: async (id) => { for (const photo of photos) photo.tags = photo.tags.filter((tag) => tag.id !== id); },
      AddTagToPhotos: async (ids, name) => { for (const id of ids) photos[id - 1].tags.push({ id: 9000, name }); },
      GetDatabaseStatus: async () => "Connected", GetDatabasePath: async () => "/mock.db",
      GetPhotoCount: async () => 1000, GetRecentPhotos: async () => [], GetTagCount: async () => 0, GetTags: async () => [],
    } } };
  });
  await page.goto(origin + "/#/tags");
  await page.waitForFunction(() => document.querySelectorAll("[data-photo-id]").length === 1000);
  await page.locator('[data-photo-id="501"] input').check();
  const anchor = await page.evaluate(() => {
    const card = document.querySelector('[data-photo-id="500"]');
    window.scrollTo(0, window.scrollY + card.getBoundingClientRect().top - 100);
    window.savedCard = card;
    return { top: card.getBoundingClientRect().top, scroll: window.scrollY };
  });
  await page.evaluate(() => {
    mock.emit(1, "completed", [{ id: 1, name: "แมว" }]);
    mock.emit(2, "processing");
    mock.emit(3, "retrying");
    mock.emit(4, "failed");
  });
  await page.waitForFunction(() => document.querySelector('[data-photo-id="1"] .tag-chip') && document.querySelector('[data-status="retrying"]').textContent.includes("1"));
  const updated = await page.evaluate(() => ({
    top: document.querySelector('[data-photo-id="500"]').getBoundingClientRect().top,
    scroll: window.scrollY, sameNode: savedCard === document.querySelector('[data-photo-id="500"]'),
    selected: document.querySelector('[data-photo-id="501"] input').checked,
    first: document.querySelector('[data-photo-id]').dataset.photoId,
    counts: Object.fromEntries([...document.querySelectorAll("[data-status]")].map((node) => [node.dataset.status, node.textContent.trim()])),
  }));
  assert.ok(Math.abs(updated.top - anchor.top) <= 2, JSON.stringify({ anchor, updated }));
  assert.ok(updated.scroll > 1000);
  assert.equal(updated.sameNode, true);
  assert.equal(updated.selected, true);
  assert.equal(updated.first, "1");
  assert.deepEqual(updated.counts, { completed: "1 รูป", processing: "1 รูป", retrying: "1 รูป", pending: "996 รูป", failed: "1 รูป" });

  // A visible row may move to the tagged section; keep its unchanged neighbour in view.
  const neighbour = await page.evaluate(() => {
    const node = [...document.querySelectorAll('[data-photo-id]')].find((node) => node.dataset.photoId !== "500" && node.getBoundingClientRect().bottom > 0);
    return { id: node.dataset.photoId, top: node.getBoundingClientRect().top };
  });
  await page.evaluate(() => mock.emit(500, "completed", [{ id: 2, name: "ทะเล" }]));
  await page.waitForFunction(() => document.querySelector('[data-photo-id="500"] .tag-chip'));
  const neighbourAfter = await page.locator(`[data-photo-id="${neighbour.id}"]`).evaluate((node) => node.getBoundingClientRect().top);
  assert.ok(Math.abs(neighbourAfter - neighbour.top) <= 2, JSON.stringify({ neighbour, neighbourAfter }));

  const before = await page.evaluate(() => mock.calls.delta.length);
  await page.evaluate(() => { mock.delay = 500; mock.emit(10, "processing"); });
  await page.waitForFunction((before) => mock.calls.delta.length > before, before);
  await page.evaluate(() => mock.emit(10, "completed", [{ id: 3, name: "วิว" }]));
  await page.waitForFunction(() => document.querySelector('[data-photo-id="10"] .tag-chip'), null, { timeout: 10000 });
  await page.evaluate(() => { mock.delay = 50; mock.failNext = true; mock.emit(11, "completed", [{ id: 4, name: "ทดสอบ" }]); });
  await page.waitForFunction(() => document.querySelector('[role="alert"]'));
  assert.ok(await page.evaluate(() => window.scrollY > 1000 && document.querySelectorAll("[data-photo-id]").length === 1000));
  await page.waitForFunction(() => document.querySelector('[data-photo-id="11"] .tag-chip') && !document.querySelector('[role="alert"]'), null, { timeout: 12000 });

  await page.evaluate(() => window.scrollTo(0, 0));
  await page.evaluate(() => mock.emit(100, "completed", [{ id: 5, name: "สัตว์" }]));
  await page.waitForFunction(() => document.querySelector('[data-photo-id="100"] .tag-chip'));
  assert.equal(await page.evaluate(() => window.scrollY), 0);
  if (process.env.PHOTO_BACKUP_TEST_SCREENSHOT) await page.screenshot({ path: process.env.PHOTO_BACKUP_TEST_SCREENSHOT });

  await page.evaluate(() => {
    for (let id = 1; id <= 1000; id++) mock.emit(id, "completed", [{ id: 6, name: "ครบพันรูป" }]);
  });
  await page.waitForFunction(() => document.querySelectorAll(".photo-card .tag-chip").length === 1000, null, { timeout: 30000 });
  assert.equal(await page.locator('[data-status="completed"]').textContent(), "1,000 รูป");
  const calls = await page.evaluate(() => mock.calls);
  assert.equal(calls.full, 1, "background updates must not reload all thumbnails");
  assert.ok(calls.delta.every((ids) => ids.length <= 100));
  assert.ok(await page.locator('[data-photo-id="501"] input').isChecked());
  await page.getByRole("link", { name: "Dashboard", exact: true }).click();
  await page.waitForFunction(() => !document.querySelector(".tags-page"));
  const deltaCount = await page.evaluate(() => mock.calls.delta.length);
  await page.evaluate(() => mock.emit(20, "processing"));
  await page.waitForTimeout(1500);
  assert.equal(await page.evaluate(() => mock.calls.delta.length), deltaCount, "unmounted Tags must not fetch");
  assert.deepEqual(errors, []);
  console.log(JSON.stringify({ result: "PASS", photos: 1000, fullLoads: calls.full, deltaRequests: calls.delta.length, anchorErrorPixels: updated.top - anchor.top, preservedSelection: updated.selected, checks: ["retry counts", "tagged first", "stable DOM and viewport", "in-flight events", "failed refresh retry", "top stays top", "1000 event burst", "unmount cleanup"] }, null, 2));
} finally {
  await browser?.close();
  await new Promise((resolve) => server.close(resolve));
}
