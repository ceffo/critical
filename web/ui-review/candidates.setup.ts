import { THEMES, type Theme } from "../src/lib/theme";
import { CANDIDATES } from "./candidate-themes";

// Adds the round's candidate themes to the app's THEMES so theme.test.ts measures them with the
// rules of ADR 0026 (vitest.candidates.ts). Its tests that pin the exact theme list fail by design.
(THEMES as unknown as Theme[]).push(...CANDIDATES);
