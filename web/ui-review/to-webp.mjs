// Converts the mock-up round's PNGs to commit (web/e2e-results/ui-mockups/commit/, written by
// mockups.spec.ts) into lossless WebP in docs/design/ui-review/mockups/shots/, replacing what was
// there: identical pixels at about 40 % of the PNG's weight. Needs ImageMagick (`magick`). Run by
// `just ui-mockups` after the capture.
import { spawnSync } from "node:child_process";
import { mkdirSync, readdirSync, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
const src = path.join(root, "web", "e2e-results", "ui-mockups", "commit");
const dst = path.join(root, "docs", "design", "ui-review", "mockups", "shots");

const pngs = readdirSync(src).filter((f) => f.endsWith(".png"));
if (pngs.length === 0) {
  console.error(`no PNG in ${path.relative(root, src)}: run the capture first`);
  process.exit(1);
}
rmSync(dst, { recursive: true, force: true });
mkdirSync(dst, { recursive: true });
for (const f of pngs) {
  const out = path.join(dst, f.replace(/\.png$/, ".webp"));
  const r = spawnSync(
    "magick",
    [
      path.join(src, f),
      "-define",
      "webp:lossless=true",
      "-define",
      "webp:method=6",
      out,
    ],
    { stdio: "inherit" },
  );
  if (r.error || r.status !== 0) {
    console.error(
      `magick failed on ${f}${r.error ? `: ${r.error.message}` : ""}`,
    );
    process.exit(1);
  }
}
console.log(`${pngs.length} shots in ${path.relative(root, dst)}`);
