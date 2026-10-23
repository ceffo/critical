import { test as base, expect, type Page } from "@playwright/test";
import { spawn, type ChildProcess } from "node:child_process";
import { cp, mkdtemp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import http from "node:http";
import { trackApi } from "./helpers";
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
      const res = await fetch(`${url}/api/health`);
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
  /** The session cookie: the stack's dev login (ADR 0029 step 8.1). */
  session: SessionCookie;
}

/** A session cookie's name and value. */
interface SessionCookie {
  name: string;
  value: string;
}

/**
 * The API's session cookie (ADR 0029 step 4.2) is named per app, `critical_session_<app>` (each stack's
 * API port), so the name is read from the dev login's answer.
 */
const sessionCookiePrefix = "critical_session";

/** Logs into the API through its dev login (no OAuth settings: the root admin) and returns the session cookie. */
async function devLogin(apiUrl: string): Promise<SessionCookie> {
  const res = await fetch(`${apiUrl}/api/auth/dev`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
  if (!res.ok) throw new Error(`dev login: ${res.status} ${await res.text()}`);
  for (const c of res.headers.getSetCookie()) {
    const [pair] = c.split(";");
    const [name, value] = pair.split("=");
    if (name.startsWith(sessionCookiePrefix) && value) return { name, value };
  }
  throw new Error("dev login: no session cookie");
}

const nodeFetch = globalThis.fetch;
let signedIn: { apiUrl: string; session: SessionCookie } | null = null;

/**
 * Every request a spec sends to its stack's API from Node carries the stack's session, as the browser's do:
 * a worker runs one test at a time, and the stack fixture sets and clears `signedIn`.
 */
globalThis.fetch = (input, init) => {
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  if (!signedIn || !url.startsWith(`${signedIn.apiUrl}/`))
    return nodeFetch(input, init);
  const headers = new Headers(
    init?.headers ?? (input instanceof Request ? input.headers : undefined),
  );
  headers.set("Cookie", `${signedIn.session.name}=${signedIn.session.value}`);
  return nodeFetch(input, { ...init, headers });
};

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

/**
 * Format 6 (ADR 0021 step 12) gives every priest spell its sphere codes: an extra priest spell
 * without them gets them from its raw sphere text ("Elemental (Fire)" → elemental_fire).
 */
function withSphereCodes(spell: unknown): unknown {
  const s = spell as { class?: string; sphere?: string; spheres?: string[] };
  if (s.class !== "Priest" || s.spheres) return spell;
  const codes = (s.sphere ?? "All")
    .split(/[,/]/)
    .map((p) =>
      p
        .trim()
        .toLowerCase()
        .replace(/[^a-z]+/g, "_")
        .replace(/^_|_$/g, ""),
    )
    .filter((c) => c !== "");
  return { ...s, spheres: [...new Set(codes.length ? codes : ["all"])] };
}

/** The fixture catalogue with extra spells (artifact format, each with its id) appended; an extra
 * spell replaces the fixture's spell of the same id. */
async function catalogueWith(
  dir: string,
  extraSpells: unknown[],
): Promise<string> {
  if (extraSpells.length === 0) return catalogueFile;
  const doc = JSON.parse(await readFile(catalogueFile, "utf8")) as {
    spells: unknown[];
  };
  const extraIds = new Set(extraSpells.map((s) => (s as { id: string }).id));
  const out = path.join(dir, "catalogue.json");
  await writeFile(
    out,
    JSON.stringify({
      ...doc,
      spells: [
        ...doc.spells.filter((s) => !extraIds.has((s as { id: string }).id)),
        ...extraSpells.map(withSphereCodes),
      ],
    }),
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
          // The browser's Host is kept, as the production server sees it: the API's CSRF check
          // compares it with the Origin of every mutation (ADR 0029 step 4.5).
          headers: req.headers,
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
    // The request path is untrusted: resolve it and check it stays inside distDir before any
    // file system access (a bare prefix test would also accept a sibling like `dist-evil`).
    const root = path.resolve(distDir);
    let file = path.join(root, "index.html");
    try {
      const candidate = path.resolve(root, "." + decodeURIComponent(urlPath));
      if (
        candidate.startsWith(root + path.sep) &&
        existsSync(candidate) &&
        statSync(candidate).isFile()
      ) {
        file = candidate;
      }
    } catch {
      // Malformed percent-encoding: serve the SPA entry point.
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
        // The dev login (ADR 0029 step 8.2): no OAuth settings in the e2e runs.
        CRITICAL_ENV: "dev",
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
      const session = await devLogin(apiUrl);
      signedIn = { apiUrl, session };
      webServer.setApi(apiUrl);
      await provide({ webUrl: webServer.url, apiUrl, session });
    } finally {
      signedIn = null;
      api.kill("SIGTERM");
      await rm(tmp, { recursive: true, force: true });
    }
  },

  // Open the dashboard of this test's stack.
  baseURL: async ({ stack }, provide) => provide(stack.webUrl),

  // The browser starts logged in (the harness's dev login): the session cookie is host-only, so it
  // also reaches the API on its own port.
  context: async ({ context, stack }, provide) => {
    trackApi(context);
    await context.addCookies([
      {
        name: stack.session.name,
        value: stack.session.value,
        domain: new URL(stack.webUrl).hostname,
        path: "/",
        httpOnly: true,
        sameSite: "Lax",
      },
    ]);
    // No animation or transition: a popover or dialog that fades out stays in the DOM for its
    // duration, and every `toBeHidden` / `toHaveCount(0)` / click-while-stable waited for it (about a
    // fifth of the sweeps' time). What the specs check is the final state, never the motion; the
    // screenshots already disable animations.
    await context.addInitScript(() => {
      const style = document.createElement("style");
      style.textContent =
        "*,*::before,*::after{animation-duration:0s!important;animation-delay:0s!important;transition-duration:0s!important;transition-delay:0s!important}";
      document.addEventListener("DOMContentLoaded", () =>
        document.head.appendChild(style),
      );
    });
    // Flake hunting: `E2E_LAT=400 just e2e <spec>` delays every API answer by 0 to 400 ms (random), which
    // exposes a spec that reads state before the page has it. Off by default.
    if (process.env.E2E_LAT) {
      const max = Number(process.env.E2E_LAT);
      await context.route("**/api/**", async (route) => {
        await new Promise((r) => setTimeout(r, Math.random() * max));
        await route.continue();
      });
    }
    await provide(context);
  },
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
