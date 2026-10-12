import { defineConfig } from "vite";
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
  server: {
    // `just dev` runs the API on :8080; forward /api like nginx does in compose.
    proxy: { "/api": process.env.CRITICAL_API_URL ?? "http://localhost:8080" },
  },
});
