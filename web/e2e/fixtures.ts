import { test as base, expect, type Page } from "@playwright/test";
import { spawn, type ChildProcess } from "node:child_process";
import { cp, mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import http from "node:http";
import { createReadStream, existsSync, statSync } from "node:fs";

const webDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = path.resolve(webDir, "..");
const apiBinary = path.join(repoRoot, "bin", "critical-srv"); // `just build-api`
// The frozen test fixture, never the campaign seeds/ (editing those must not break e2e).
const seedsDir = path.join(repoRoot, "api", "testdata", "seeds");
// Built once per run by e2e/global-setup.ts (the production bundle, not a dev server).
export const distDir = path.join(webDir, "e2e-results", "dist");
const screensDir = path.join(webDir, "e2e-results", "screens");

/** Ask the OS for a free TCP port. */
function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.once("error", reject);
    srv.listen(0, "127.0.0.1", () => {
      const { port } = srv.address() as net.AddressInfo;
      srv.close(() => resolve(port));
    });
  });
}

async function waitForApi(url: string, proc: ChildProcess, log: () => string) {
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    if (proc.exitCode !== null) {
      throw new Error(
        `API exited (${proc.exitCode}) during startup:\n${log()}`,
      );
    }
    try {
      const res = await fetch(`${url}/api/character`);
      if (res.ok) return;
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error(`API did not become ready at ${url}:\n${log()}`);
}

export interface Stack {
  /** Base URL of the web server (what the browser opens). */
  webUrl: string;
  /** Base URL of this stack's own Go API. */
  apiUrl: string;
  /** This stack's SQLite database, for tests that must plant data the API can't create (old rows). */
  dbPath: string;
}

/**
 * Copy the fixture seeds into dir with fields of character.json replaced, for tests that need a specific
 * sheet (e.g. STR 12 so a Strength buff is what reaches 18). Returns the seeds dir to use.
 */
async function seedsWith(
  dir: string,
  character: Record<string, unknown>,
  extraSpells: unknown[],
): Promise<string> {
  if (Object.keys(character).length === 0 && extraSpells.length === 0)
    return seedsDir;
  const out = path.join(dir, "seeds");
  await cp(seedsDir, out, { recursive: true });
  if (Object.keys(character).length > 0) {
    const file = path.join(out, "character.json");
    const sheet = JSON.parse(await readFile(file, "utf8")) as Record<
      string,
      unknown
    >;
    await writeFile(file, JSON.stringify({ ...sheet, ...character }));
  }
  if (extraSpells.length > 0) {
    const file = path.join(out, "spells.json");
    const spells = JSON.parse(await readFile(file, "utf8")) as unknown[];
    await writeFile(file, JSON.stringify([...spells, ...extraSpells]));
  }
  return out;
}

export interface StackOptions {
  /** Fields overriding the fixture character.json for this test, e.g. `test.use({ character: { str: 12 } })`. */
  character: Record<string, unknown>;
  /** Spell rows appended to the fixture spells.json (e.g. enough of them to need several lookup pages); wrapped because Playwright mangles array-valued options. */
  extraSpells: { rows: unknown[] };
}

const mime: Record<string, string> = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".json": "application/json",
  ".png": "image/png",
  ".woff2": "font/woff2",
  ".ico": "image/x-icon",
};

interface WebServer {
  url: string;
  /** Where /api goes: the API of the test currently running on this worker. */
  setApi(url: string): void;
}

/**
 * Serves the built bundle (SPA fallback) and forwards /api to the current test's API, like nginx
 * does in compose. One per worker: a worker runs one test at a time, so a mutable target is enough.
 */
async function startWebServer(): Promise<{
  server: WebServer;
  close(): Promise<void>;
}> {
  let apiTarget = new URL("http://127.0.0.1:1");
  const srv = http.createServer((req, res) => {
    const urlPath = (req.url ?? "/").split("?")[0];
    if (urlPath.startsWith("/api/") || urlPath === "/api") {
      const up = http.request(
        {
          host: apiTarget.hostname,
          port: apiTarget.port,
          method: req.method,
          path: req.url,
          headers: { ...req.headers, host: apiTarget.host },
        },
        (r) => {
          res.writeHead(r.statusCode ?? 502, r.headers);
          r.pipe(res);
        },
      );
      up.on("error", () => {
        res.writeHead(502).end();
      });
      req.pipe(up);
      return;
    }
    let file = path.join(distDir, path.normalize(decodeURIComponent(urlPath)));
    if (
      !file.startsWith(distDir) ||
      !existsSync(file) ||
      !statSync(file).isFile()
    ) {
      file = path.join(distDir, "index.html");
    }
    res.writeHead(200, {
      "content-type": mime[path.extname(file)] ?? "application/octet-stream",
    });
    createReadStream(file).pipe(res);
  });
  await new Promise<void>((r) => srv.listen(0, "127.0.0.1", r));
  const { port } = srv.address() as net.AddressInfo;
  return {
    server: {
      url: `http://127.0.0.1:${port}`,
      setApi: (u) => {
        apiTarget = new URL(u);
      },
    },
    close: () =>
      new Promise<void>((r) => {
        srv.closeAllConnections();
        srv.close(() => r());
      }),
  };
}

/**
 * A throwaway stack: Go API on a free port with a fresh temp SQLite DB seeded from
 * api/testdata/seeds, behind the worker's web server (built bundle, /api proxied to this API).
 * Never touches .dev/ or fixed ports.
 */
export const test = base.extend<
  { stack: Stack } & StackOptions,
  { webServer: WebServer }
>({
  character: [{}, { option: true }],
  extraSpells: [{ rows: [] }, { option: true }],

  webServer: [
    // eslint-disable-next-line no-empty-pattern -- Playwright requires a destructuring pattern
    async ({}, provide) => {
      const { server, close } = await startWebServer();
      try {
        await provide(server);
      } finally {
        await close();
      }
    },
    { scope: "worker" },
  ],

  stack: async ({ character, extraSpells, webServer }, provide) => {
    const tmp = await mkdtemp(path.join(os.tmpdir(), "critical-e2e-"));
    const dbPath = path.join(tmp, "critical.db");
    const apiPort = await freePort();
    const apiUrl = `http://127.0.0.1:${apiPort}`;
    let output = "";
    const api = spawn(apiBinary, [], {
      env: {
        ...process.env,
        CRITICAL_DSN: dbPath,
        CRITICAL_SEEDS_DIR: await seedsWith(tmp, character, extraSpells.rows),
        CRITICAL_ADDR: `127.0.0.1:${apiPort}`,
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
    api.stdout?.on("data", (d) => (output += d));
    api.stderr?.on("data", (d) => (output += d));
    api.on("error", (e) => (output += String(e)));

    try {
      await waitForApi(apiUrl, api, () => output);
      webServer.setApi(apiUrl);
      await provide({ webUrl: webServer.url, apiUrl, dbPath });
    } finally {
      api.kill("SIGTERM");
      await rm(tmp, { recursive: true, force: true });
    }
  },

  // Open the dashboard of this test's stack.
  baseURL: async ({ stack }, provide) => provide(stack.webUrl),
});

export { expect };

/** Save a full-page PNG to web/e2e-results/screens/<spec>/<name>.png. */
export async function snap(page: Page, name: string): Promise<string> {
  const spec = path.basename(test.info().file).replace(/\.spec\.ts$/, "");
  const dir = path.join(screensDir, spec);
  await mkdir(dir, { recursive: true });
  const file = path.join(dir, `${name}.png`);
  await page.screenshot({ path: file });
  return file;
}
