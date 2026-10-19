import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";
import { defineConfig, globalIgnores } from "eslint/config";

export default defineConfig([
  globalIgnores(["dist", "e2e-results"]),
  {
    files: ["**/*.{ts,tsx}"],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
  },
  {
    // One file per journal entry kind holds its record and its result card (docs/frontend.md,
    // "Journal registry"): an edit there reloads the page instead of hot-swapping the card.
    files: ["src/journal/kinds/*.tsx"],
    rules: { "react-refresh/only-export-components": "off" },
  },
]);
