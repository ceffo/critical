import { describe, expect, it } from "vitest";
import { userLabel } from "./userName";

describe("userLabel", () => {
  it("shows the name when there is one", () => {
    expect(
      userLabel({ displayName: "Ann Lovelace", email: "ann@example.com" }),
    ).toBe("Ann Lovelace");
  });
  it("falls back to the email when the name is absent, empty or blank", () => {
    expect(userLabel({ email: "ann@example.com" })).toBe("ann@example.com");
    expect(userLabel({ displayName: "", email: "ann@example.com" })).toBe(
      "ann@example.com",
    );
    expect(userLabel({ displayName: "  ", email: "ann@example.com" })).toBe(
      "ann@example.com",
    );
  });
});
