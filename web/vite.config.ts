import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  // Playwright specs live in e2e/ and run via `just e2e`, not vitest.
  // Two projects: the pure-logic tests stay in node (no DOM to start); only `*.dom.test.tsx`
  // files get happy-dom, Testing Library and the jest-dom matchers (docs/frontend.md, "Testing").
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: "node",
          include: ["src/**/*.test.{ts,tsx}"],
          exclude: ["src/**/*.dom.test.tsx"],
        },
      },
      {
        extends: true,
        test: {
          name: "dom",
          environment: "happy-dom",
          include: ["src/**/*.dom.test.tsx"],
          setupFiles: ["src/test/setup.ts"],
        },
      },
    ],
  },
  server: {
    // All interfaces (IPv4 too: a tunnel to 127.0.0.1:5173 must work); browsable by tailnet name.
    host: true,
    allowedHosts: [".ts.net"],
    // `just dev` runs the API on :8080; forward /api to it (in the image the API serves the app itself).
    // The browser's Host is kept (the string shorthand would rewrite it): the API's CSRF check compares
    // it with the Origin of every mutation (ADR 0029 step 4.5).
    proxy: {
      "/api": {
        target: process.env.CRITICAL_API_URL ?? "http://localhost:8080",
        changeOrigin: false,
      },
    },
  },
});
