import { describe, expect, it } from "vitest";
import { railActivities } from "./Rail";

describe("railActivities", () => {
  it("lists Powers only for a character with known powers", () => {
    expect(railActivities(false)).not.toContain("powers");
    expect(railActivities(true)).toContain("powers");
  });
});
