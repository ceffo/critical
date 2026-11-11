import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { buttonVariants } from "./button";

describe("button looks (ADR 0027 step 3.4, amended 2026-11-11)", () => {
  it("a disabled button has no dashed edge, in any variant", () => {
    for (const variant of ["main", "quiet", "ghost", "destructive"] as const) {
      expect(buttonVariants({ variant })).not.toMatch(/dashed/);
    }
  });

  it("a disabled button is the raised tone with muted ink, apart from a quiet one", () => {
    const quiet = buttonVariants({ variant: "quiet" });
    expect(quiet).toContain("disabled:bg-raised");
    expect(quiet).toContain("disabled:text-ink-muted");
    expect(quiet).toContain("disabled:border-transparent");
  });

  it("Cast is not a main button: the Spells tab shows one filled button at most", () => {
    const src = readFileSync(
      join(__dirname, "..", "actions", "SpellsPanel.tsx"),
      "utf8",
    );
    expect(src).not.toMatch(/variant="main"/);
  });
});
