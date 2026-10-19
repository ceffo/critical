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
    // The action module (stores/table.ts) owns the error, the notice, the busy flag and what is read
    // again after a write (ADR 0018 step 11): a data store runs its writes through it, never sets
    // its state nor calls its refresh.
    files: ["src/stores/**/*.ts"],
    ignores: ["src/stores/table.ts", "src/stores/**/*.test.ts"],
    rules: {
      "no-restricted-syntax": [
        "error",
        {
          selector:
            "CallExpression[callee.object.name='useTableStore'][callee.property.name='setState']",
          message:
            "A store does not write the action module's state: run the write through run/submit, or hand a failure to fail().",
        },
        {
          selector: "CallExpression[callee.property.name='refresh']",
          message:
            "A store does not read other stores again: the action module reads what a write's state keys name.",
        },
      ],
    },
  },
  {
    // One file per journal entry kind holds its record and its result card (docs/frontend.md,
    // "Journal registry"): an edit there reloads the page instead of hot-swapping the card.
    files: ["src/journal/kinds/*.tsx"],
    rules: { "react-refresh/only-export-components": "off" },
  },
]);
