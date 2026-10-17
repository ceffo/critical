// Copies the curated UI-review screenshots (docs/design/ui-review/curated.txt, one path per line
// relative to web/e2e-results/ui-review/, `#` comments allowed) into docs/design/ui-review/shots/,
// flattening the theme directory into the file name (<theme>--<screen>-<WxH>.png). Run by
// `just ui-shots` after the capture; fails when a listed screenshot was not produced.
import { copyFileSync, mkdirSync, readFileSync, existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const webDir = path.resolve(here, "..");
const root = path.resolve(webDir, "..");
const src = path.join(webDir, "e2e-results", "ui-review");
const docs = path.join(root, "docs", "design", "ui-review");
const dst = path.join(docs, "shots");

const wanted = readFileSync(path.join(docs, "curated.txt"), "utf8")
  .split("\n")
  .map((l) => l.trim())
  .filter((l) => l && !l.startsWith("#"));

mkdirSync(dst, { recursive: true });
let missing = 0;
for (const rel of wanted) {
  const from = path.join(src, rel);
  if (!existsSync(from)) {
    console.error(`missing: ${rel}`);
    missing++;
    continue;
  }
  const flat = rel.split("/").join("--");
  copyFileSync(from, path.join(dst, flat));
}
console.log(
  `${wanted.length - missing} curated screenshots in ${path.relative(root, dst)}`,
);
if (missing) process.exit(1);
