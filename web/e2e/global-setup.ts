import { build } from "vite";
import path from "node:path";
import { fileURLToPath } from "node:url";

// Build the web app once per run; every worker serves this bundle (see fixtures.ts).
export default async function globalSetup() {
  const webDir = path.resolve(
    path.dirname(fileURLToPath(import.meta.url)),
    "..",
  );
  await build({
    root: webDir,
    logLevel: "warn",
    build: {
      outDir: path.join(webDir, "e2e-results", "dist"),
      emptyOutDir: true,
    },
  });
}
