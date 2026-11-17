// The Skills design round's "before" pictures (docs/design/skills/README.md): the real app as it is
// on main, the in-memory API with the test bard (api/testdata/seeds), headless, French and English at
// 1440x900, into baseline/<screen>--<lang>.png plus a full-height shot of the Skills activity.
// Needs bin/critical-srv (`just build-api`), web/node_modules and Playwright's Chromium.
// Run: node docs/design/skills/baseline.mjs
import { spawn } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync } from "node:fs";
import { createRequire } from "node:module";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../../..");
const web = path.join(root, "web");
const { chromium } = createRequire(path.join(web, "package.json"))("@playwright/test");

const freePort = () =>
  new Promise((ok) => {
    const s = net.createServer().listen(0, "127.0.0.1", () => {
      const p = s.address().port;
      s.close(() => ok(p));
    });
  });
const waitFor = async (url) => {
  for (let i = 0; i < 200; i++) {
    try {
      await fetch(url);
      return;
    } catch {
      await new Promise((r) => setTimeout(r, 150));
    }
  }
  throw new Error(`no answer from ${url}`);
};

const tmp = mkdtempSync(path.join(os.tmpdir(), "skills-baseline-"));
cpSync(path.join(root, "api/testdata/seeds"), path.join(tmp, "seeds"), { recursive: true });
const apiPort = await freePort();
const webPort = await freePort();
const env = { ...process.env };
delete env.CRITICAL_DSN;
const api = spawn(path.join(root, "bin/critical-srv"), [], {
  env: {
    ...env,
    CRITICAL_STORAGE: "memory",
    CRITICAL_ENV: "dev",
    CRITICAL_DICE_SEED: "1",
    CRITICAL_SEEDS_DIR: path.join(tmp, "seeds"),
    CRITICAL_CATALOGUE: path.join(root, "api/testdata/catalogue.json"),
    CRITICAL_ADDR: `127.0.0.1:${apiPort}`,
  },
  stdio: "ignore",
});
const vite = spawn("npx", ["vite", "--port", String(webPort), "--strictPort"], {
  cwd: web,
  env: { ...process.env, CRITICAL_API_URL: `http://127.0.0.1:${apiPort}` },
  stdio: "ignore",
});
const stop = () => {
  api.kill();
  vite.kill();
};
try {
  await waitFor(`http://127.0.0.1:${apiPort}/api/health`);
  const base = `http://127.0.0.1:${webPort}`;
  await waitFor(base);
  const out = path.join(here, "baseline");
  mkdirSync(out, { recursive: true });
  const browser = await chromium.launch();
  for (const lang of ["fr", "en"]) {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    await ctx.addInitScript((l) => {
      localStorage.setItem("critical.settings.v1", JSON.stringify({ lang: l, mode: "dark", darkTheme: "arcane", lightTheme: "arcane" }));
    }, lang);
    const page = await ctx.newPage();
    await page.goto(base);
    await page.request.post(`${base}/api/auth/dev`, { data: {} });
    await page.goto(base);
    await page.waitForSelector('[data-testid="rail"]');
    const rail = page.getByTestId("rail").getByRole("button");
    const shot = async (name, full = false) => {
      await page.waitForTimeout(600);
      await page.screenshot({ path: path.join(out, `${name}--${lang}.png`) });
      if (!full) return;
      const hidden = await page.evaluate(() =>
        Math.max(0, ...[...document.querySelectorAll("*")].map((el) =>
          getComputedStyle(el).overflowY === "auto" ? el.scrollHeight - el.clientHeight : 0)));
      if (hidden > 0) {
        await page.setViewportSize({ width: 1440, height: 900 + hidden });
        await page.waitForTimeout(300);
        await page.screenshot({ path: path.join(out, `${name}-full--${lang}.png`) });
        await page.setViewportSize({ width: 1440, height: 900 });
      }
    };
    await rail.nth(0).click();
    await shot("act");
    const tabs = await page.getByRole("tab").allTextContents();
    const skillsTab = tabs.findIndex((t) => /comp|skill/i.test(t));
    if (skillsTab >= 0) {
      await page.getByRole("tab").nth(skillsTab).click();
      await shot("act-skills", true);
    }
    const n = await rail.count();
    for (let i = 0; i < n; i++) {
      const label = (await rail.nth(i).textContent()) ?? "";
      if (/comp[ée]tences|skills/i.test(label)) {
        await rail.nth(i).click();
        await page.waitForSelector('[data-testid="skills-panel"]');
        await shot("skills", true);
      }
    }
    await ctx.close();
  }
  await browser.close();
  console.log(`baseline shots in ${path.relative(root, out)}`);
} finally {
  stop();
}
