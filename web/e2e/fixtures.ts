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
// The fixture spell catalogue (ADR 0021), always: counts and results stay deterministic, and CI has
// no access to the real artifact.
const catalogueFile = path.join(repoRoot, "api", "testdata", "catalogue.json");
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
}

/**
 * Copy the fixture seeds into dir with fields of character.json replaced, for tests that need a specific
 * sheet (e.g. STR 12 so a Strength buff is what reaches 18). Returns the seeds dir to use.
 */
async function seedsWith(
  dir: string,
  character: Record<string, unknown>,
): Promise<string> {
  if (Object.keys(character).length === 0) return seedsDir;
  const out = path.join(dir, "seeds");
  await cp(seedsDir, out, { recursive: true });
  const file = path.join(out, "character.json");
  const sheet = JSON.parse(await readFile(file, "utf8")) as Record<
    string,
    unknown
  >;
  await writeFile(file, JSON.stringify({ ...sheet, ...character }));
  return out;
}

/** The fixture catalogue with extra spells (artifact format, each with its id) appended. */
async function catalogueWith(
  dir: string,
  extraSpells: unknown[],
): Promise<string> {
  if (extraSpells.length === 0) return catalogueFile;
  const doc = JSON.parse(await readFile(catalogueFile, "utf8")) as {
    spells: unknown[];
  };
  const out = path.join(dir, "catalogue.json");
  await writeFile(
    out,
    JSON.stringify({ ...doc, spells: [...doc.spells, ...extraSpells] }),
  );
  return out;
}

export interface StackOptions {
  /** Fields overriding the fixture character.json for this test, e.g. `test.use({ character: { str: 12 } })`. */
  character: Record<string, unknown>;
  /** Spells appended to the fixture catalogue, in artifact format with an id (e.g. enough of them to need several lookup pages; see `catalogueSpell`); wrapped because Playwright mangles array-valued options. */
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
 * Serves the built bundle (SPA fallback) and forwards /api to the current test's API, as the
 * production image's server does. One per worker: a worker runs one test at a time, so a mutable target is enough.
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
 * A throwaway stack: Go API on a free port with its in-memory storage (ADR 0025 step 8: no database,
 * file or server) seeded from api/testdata/seeds, behind the worker's web server (built bundle, /api
 * proxied to this API). Never touches .dev/ or fixed ports.
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
    // Holds only the seeds and catalogue a test overrides.
    const tmp = await mkdtemp(path.join(os.tmpdir(), "critical-e2e-"));
    const apiPort = await freePort();
    const apiUrl = `http://127.0.0.1:${apiPort}`;
    let output = "";
    // No CRITICAL_DSN: the in-memory storage opens no database.
    const env = { ...process.env };
    delete env.CRITICAL_DSN;
    const api = spawn(apiBinary, [], {
      env: {
        ...env,
        CRITICAL_STORAGE: "memory",
        CRITICAL_SEEDS_DIR: await seedsWith(tmp, character),
        CRITICAL_CATALOGUE: await catalogueWith(tmp, extraSpells.rows),
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
      await provide({ webUrl: webServer.url, apiUrl });
    } finally {
      api.kill("SIGTERM");
      await rm(tmp, { recursive: true, force: true });
    }
  },

  // Open the dashboard of this test's stack.
  baseURL: async ({ stack }, provide) => provide(stack.webUrl),
});

export { expect };

/** Fields of a spell appended to the fixture catalogue; the stats default to plain values. */
export interface ExtraSpell {
  id: string;
  /** The English name (the books'). */
  name: string;
  /** Its name in other languages, by language code (ADR 0023: a language map, as the artifact). */
  names?: Readonly<Record<string, string>>;
  class: "Wizard" | "Priest";
  level: number;
  school: string;
  sphere?: string;
  casting_time?: string;
  range?: string;
  duration?: string;
  area_of_effect?: string;
  components?: string;
  saving_throw?: string;
}

/** A catalogue spell (ADR 0021 format) with invented text, for `extraSpells`. */
export function catalogueSpell(s: ExtraSpell) {
  const { id, name, names, sphere, ...rest } = s;
  return {
    id,
    casting_time: "1",
    range: "10 yds.",
    duration: "1 rd.",
    area_of_effect: "1 creature",
    components: "V, S",
    saving_throw: "None",
    ...rest,
    ...(sphere ? { sphere } : {}),
    name: { ...names, en: name },
    description: { en: `${name} fixture text.` },
    source: "WEB",
  };
}

/** Save a full-page PNG to web/e2e-results/screens/<spec>/<name>.png. */
export async function snap(page: Page, name: string): Promise<string> {
  const spec = path.basename(test.info().file).replace(/\.spec\.ts$/, "");
  const dir = path.join(screensDir, spec);
  await mkdir(dir, { recursive: true });
  const file = path.join(dir, `${name}.png`);
  await page.screenshot({ path: file, animations: "disabled" });
  return file;
}
