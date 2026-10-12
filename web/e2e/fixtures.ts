import { test as base, expect, type Page } from "@playwright/test";
import { spawn, type ChildProcess } from "node:child_process";
import { mkdtemp, mkdir, rm } from "node:fs/promises";
import net from "node:net";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const webDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = path.resolve(webDir, "..");
const apiBinary = path.join(repoRoot, "bin", "critical-srv"); // `just build-api`
const seedsDir = path.join(repoRoot, "seeds");
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
  /** Base URL of the Vite server (what the browser opens). */
  webUrl: string;
  /** Base URL of this stack's own Go API. */
  apiUrl: string;
}

/**
 * A throwaway stack: Go API on a free port with a fresh temp SQLite DB seeded
 * from the repo's real seeds/, plus a Vite dev server on a free port proxying
 * /api to that API. Never touches .dev/ or fixed ports.
 */
export const test = base.extend<{ stack: Stack }>({
  // eslint-disable-next-line no-empty-pattern -- Playwright requires a destructuring pattern
  stack: async ({}, provide) => {
    const tmp = await mkdtemp(path.join(os.tmpdir(), "critical-e2e-"));
    const apiPort = await freePort();
    const apiUrl = `http://127.0.0.1:${apiPort}`;
    let output = "";
    const api = spawn(apiBinary, [], {
      env: {
        ...process.env,
        CRITICAL_DSN: path.join(tmp, "critical.db"),
        CRITICAL_SEEDS_DIR: seedsDir,
        CRITICAL_ADDR: `127.0.0.1:${apiPort}`,
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
    api.stdout?.on("data", (d) => (output += d));
    api.stderr?.on("data", (d) => (output += d));
    api.on("error", (e) => (output += String(e)));

    let vite: Awaited<ReturnType<typeof createServer>> | undefined;
    try {
      await waitForApi(apiUrl, api, () => output);
      vite = await createServer({
        root: webDir,
        logLevel: "silent",
        server: {
          port: await freePort(),
          strictPort: true,
          host: "127.0.0.1",
          proxy: { "/api": apiUrl },
        },
      });
      await vite.listen();
      const webUrl = vite.resolvedUrls?.local[0]?.replace(/\/$/, "");
      if (!webUrl) throw new Error("Vite did not report a URL");
      await provide({ webUrl, apiUrl });
    } finally {
      await vite?.close();
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
