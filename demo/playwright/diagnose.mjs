// Diagnose dashboard interactivity: capture console errors, failed requests,
// and whether app.js wired up. Run while the dashboard is serving on :7878.
import { chromium } from "playwright";

const URL = process.env.AG_URL || "http://127.0.0.1:7878";

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();

const logs = [];
page.on("console", (m) => logs.push(`[console.${m.type()}] ${m.text()}`));
page.on("pageerror", (e) => logs.push(`[pageerror] ${e.message}\n${e.stack || ""}`));
page.on("requestfailed", (r) => logs.push(`[requestfailed] ${r.url()} — ${r.failure()?.errorText}`));
page.on("response", (r) => { if (r.status() >= 400) logs.push(`[http ${r.status()}] ${r.url()}`); });

await page.goto(URL, { waitUntil: "domcontentloaded" });
await page.waitForTimeout(2500);

// Did app.js evaluate? Check a few wired behaviors.
const probe = await page.evaluate(() => {
  const out = {};
  out.appJsTag = !!document.querySelector('script[src="/app.js"]');
  out.navLinks = document.querySelectorAll("nav a.nav-link").length;
  out.kpiCallsText = document.getElementById("kpiCalls")?.textContent;
  out.recentRows = document.querySelectorAll("#recentTable tbody tr").length;
  out.liveState = document.getElementById("livestate")?.textContent;
  out.paletteHidden = document.getElementById("palette")?.hidden;
  out.themeAttr = document.documentElement.getAttribute("data-theme");
  return out;
});

// Try a real nav click and see if the route changes.
const before = await page.evaluate(() => location.hash);
await page.click('nav a:has-text("Tool calls")').catch((e) => logs.push("[click err] " + e.message));
await page.waitForTimeout(600);
const after = await page.evaluate(() => ({ hash: location.hash, activePage: document.querySelector(".page.active")?.id }));

// Try theme toggle.
await page.click("#themeToggle").catch((e) => logs.push("[theme click err] " + e.message));
await page.waitForTimeout(300);
const themeAfter = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));

console.log("=== PROBE ===");
console.log(JSON.stringify(probe, null, 2));
console.log("nav hash before:", before, "-> after:", JSON.stringify(after));
console.log("theme after toggle:", themeAfter);
console.log("=== CONSOLE / ERRORS ===");
console.log(logs.join("\n") || "(none)");

await browser.close();
