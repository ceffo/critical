import { describe, expect, it } from "vitest";
import { badgeOf, parseBuild } from "./build";

describe("the version badge", () => {
  // Production shows the version alone, staging and dev a pill before it.
  it.each([
    ["production", { version: "1.4.2", pill: null }],
    ["staging", { version: "1.4.2", pill: "staging" }],
    ["dev", { version: "1.4.2", pill: "dev" }],
  ])("%s", (env, want) => {
    expect(
      badgeOf(parseBuild({ status: "ok", version: "1.4.2", env })),
    ).toEqual(want);
  });

  it("keeps the dev version's commit suffix as sent", () => {
    expect(
      badgeOf(parseBuild({ version: "0.61.1+43d0ef8.dirty", env: "dev" }))
        ?.version,
    ).toBe("0.61.1+43d0ef8.dirty");
  });

  it("shows no pill for an unknown or missing environment, and nothing without any identity", () => {
    expect(
      badgeOf(parseBuild({ version: "1.4.2", env: "unknown" }))?.pill,
    ).toBeNull();
    expect(badgeOf(parseBuild({ version: "1.4.2" }))?.pill).toBeNull();
    expect(
      badgeOf(parseBuild({ version: "unknown", env: "unknown" })),
    ).toBeNull();
    expect(badgeOf(parseBuild(null))).toBeNull();
  });
});
