// Phosphor icons, read from the app's own package (web/node_modules/@phosphor-icons/react, the
// set the app renders with) so the pages show the real glyphs: the v3 mock's Drop for damage,
// Heart for healing, Sword for the attack. Regular weight, 256-unit viewBox, `currentColor`.
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const defs = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../../../web/node_modules/@phosphor-icons/react/dist/defs",
);
const cache = new Map();

/** The regular-weight path data of one Phosphor icon, by its React name (`Drop`, `Sword`…). */
export function phosphorPath(name) {
  if (cache.has(name)) return cache.get(name);
  let src;
  try {
    src = readFileSync(path.join(defs, `${name}.es.js`), "utf8");
  } catch {
    throw new Error(`phosphor: no icon ${name} (run \`cd web && npm ci\` first)`);
  }
  const m = src.match(/"regular",[\s\S]*?d:\s*"([^"]+)"/);
  if (!m) throw new Error(`phosphor: no regular path in ${name}`);
  cache.set(name, m[1]);
  return m[1];
}

/** An inline 16 px Phosphor icon in the current colour; `cls` is the class on the svg. */
export const ph = (name, cls = "ico") =>
  `<svg class="${cls}" width="16" height="16" viewBox="0 0 256 256" fill="currentColor" aria-hidden="true"><path d="${phosphorPath(name)}"/></svg>`;
