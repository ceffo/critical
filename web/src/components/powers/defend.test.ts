import { describe, expect, it } from "vitest";
import { defendRequest } from "./defend";

describe("defendRequest", () => {
  const base = {
    attack: "ego_whip",
    rolls: [{ roll: "14", success: true }],
    keep: null,
  };
  it("sends one or two rolls and the kept index", () => {
    expect(defendRequest(base)).toEqual({
      attack: "ego_whip",
      rolls: [{ roll: 14, success: true }],
    });
    const two = {
      ...base,
      rolls: [...base.rolls, { roll: "9", success: false }],
      keep: 1,
    };
    expect(defendRequest(two)).toEqual({
      attack: "ego_whip",
      rolls: [
        { roll: 14, success: true },
        { roll: 9, success: false },
      ],
      keep: 1,
    });
  });
  it("waits for an attack and for rolls of 1 to 20", () => {
    expect(defendRequest({ ...base, attack: "" })).toBeNull();
    expect(
      defendRequest({ ...base, rolls: [{ roll: "21", success: true }] }),
    ).toBeNull();
    expect(
      defendRequest({ ...base, rolls: [{ roll: "", success: true }] }),
    ).toBeNull();
  });
});
