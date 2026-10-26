// Serves the psionics mock-ups (docs/design/psionics/) on the app's own Vite, Tailwind and
// `@/` alias, so the pages use the real stylesheet, components and theme code.
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

export default defineConfig({
  root: import.meta.dirname,
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(import.meta.dirname, "../../src") },
  },
  server: { host: "127.0.0.1", strictPort: false },
  logLevel: "warn",
});
