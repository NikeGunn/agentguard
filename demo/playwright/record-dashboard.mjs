// AgentGuard — interactive dashboard recorder (Playwright).
//
// Drives the real web dashboard (served by the Go binary) through a scripted,
// human-paced tour and records it to video. Used for the README hero GIF and
// the LinkedIn launch clip.
//
// Run (from repo root):
//   demo/agentguard.exe seed-demo --db demo/out/dash.db --count 140
//   demo/agentguard.exe dashboard --db demo/out/dash.db --no-browser &   # :7878
//   demo/agentguard.exe seed-demo --db demo/out/dash.db --live &         # flashing rows
//   node demo/playwright/record-dashboard.mjs
//
// The wrapper run-dashboard.ps1 wires all of that up for you.

import { chromium } from "playwright";
import { existsSync, mkdirSync, readdirSync, renameSync, statSync } from "node:fs";
import { join, resolve } from "node:path";
import os from "node:os";

const URL = process.env.AG_URL || "http://127.0.0.1:7878";
const OUT_DIR = resolve(process.env.AG_OUT || "demo/out");
const VIDEO_DIR = join(OUT_DIR, "video");
const DESKTOP = join(os.homedir(), "Desktop");
const W = 1600, H = 1000;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// Move the mouse along a short eased path so the cursor reads as human, then
// (optionally) click. Playwright renders the real cursor into the video.
async function glide(page, x, y, { click = false, steps = 22 } = {}) {
  await page.mouse.move(x, y, { steps });
  await sleep(220);
  if (click) {
    await page.mouse.down();
    await sleep(70);
    await page.mouse.up();
  }
}

async function center(page, selector) {
  const box = await page.locator(selector).first().boundingBox();
  if (!box) throw new Error(`no box for ${selector}`);
  return [box.x + box.width / 2, box.y + box.height / 2];
}

async function gotoNav(page, label) {
  const [x, y] = await center(page, `nav a:has-text("${label}")`);
  await glide(page, x, y, { click: true });
  await sleep(1400);
}

(async () => {
  mkdirSync(VIDEO_DIR, { recursive: true });

  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    viewport: { width: W, height: H },
    deviceScaleFactor: 2,
    recordVideo: { dir: VIDEO_DIR, size: { width: W, height: H } },
  });
  const page = await ctx.newPage();

  // ---- Tour ----
  // The dashboard holds an open SSE connection (/events), so "networkidle"
  // never fires — wait on the DOM + the first KPI render instead.
  await page.goto(URL, { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#kpiCalls", { timeout: 15000 });
  await sleep(1600); // let KPIs + donut + sparkline settle and the live feed warm up

  // Linger on the hero / KPI bar — the headline numbers.
  await glide(page, W / 2, 150);
  await sleep(1400);

  // Hover the donut (verdict breakdown) and the sparkline.
  if (await page.locator("#donut").count()) {
    const [dx, dy] = await center(page, "#donut");
    await glide(page, dx, dy);
    await sleep(1500);
  }

  // Watch live rows flash into the Activity river (driven by seed-demo --live).
  if (await page.locator("#recentTable").count()) {
    const [rx, ry] = await center(page, "#recentTable");
    await glide(page, rx, Math.max(ry, 360));
    await sleep(3200);
  }

  // Open a call-detail modal by clicking the first row in the recent table.
  const firstRow = page.locator("#recentTable tr").first();
  if (await firstRow.count()) {
    const box = await firstRow.boundingBox();
    if (box) {
      await glide(page, box.x + box.width / 2, box.y + box.height / 2, { click: true });
      await sleep(2600); // read the verdict / pipeline detail
      await page.keyboard.press("Escape");
      await sleep(900);
    }
  }

  // Tour the other pages.
  await gotoNav(page, "Tool calls");
  await sleep(2400);
  await gotoNav(page, "Top tools");
  await sleep(2200);
  await gotoNav(page, "Servers");
  await sleep(2400);
  await gotoNav(page, "Overview");
  await sleep(1200);

  // Command palette (Ctrl+K) — the power-user touch.
  await page.keyboard.press("Control+k");
  await sleep(900);
  if (await page.locator("#paletteInput").count()) {
    await page.locator("#paletteInput").type("theme", { delay: 90 });
    await sleep(1100);
    await page.keyboard.press("Enter"); // toggle theme via palette
    await sleep(1800); // light theme
  } else {
    // fallback: direct theme toggle
    const [tx, ty] = await center(page, "#themeToggle");
    await glide(page, tx, ty, { click: true });
    await sleep(1800);
  }

  // Flip back to dark and end on the overview.
  const [tx, ty] = await center(page, "#themeToggle");
  await glide(page, tx, ty, { click: true });
  await sleep(1600);

  // ---- Finalize video ----
  const video = page.video();
  await ctx.close(); // flushes the .webm
  await browser.close();

  const tmp = await video.path();
  // Wait for the file to be fully written.
  for (let i = 0; i < 20 && (!existsSync(tmp) || statSync(tmp).size < 1024); i++) await sleep(150);

  const stamp = new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19);
  const finalName = `agentguard-dashboard-${stamp}.webm`;

  // Keep a copy in demo/out/video for the GIF conversion step.
  const keep = join(VIDEO_DIR, finalName);
  renameSync(tmp, keep);

  // Save the raw clip to the Desktop for the LinkedIn launch post.
  let desktopCopy = "(Desktop not found)";
  if (existsSync(DESKTOP)) {
    desktopCopy = join(DESKTOP, finalName);
    // copy (not move) so demo/out keeps its copy too
    const { copyFileSync } = await import("node:fs");
    copyFileSync(keep, desktopCopy);
  }

  console.log("VIDEO_OUT=" + keep);
  console.log("DESKTOP_OUT=" + desktopCopy);
  // surface any stray videos so nothing is silently lost
  console.log("VIDEO_DIR contents: " + readdirSync(VIDEO_DIR).join(", "));
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
