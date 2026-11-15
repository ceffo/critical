import { describe, expect, it } from "vitest";
import type { SharedCharacter } from "@/types/character";
import { whoIs } from "./viewBanner";

const list: SharedCharacter[] = [
  { id: 4, name: "Ilyana", classes: [], owner: { id: "a", name: " Ada " } },
  { id: 6, name: "Bram", classes: [], owner: { id: "b", name: "" } },
];

describe("whoIs", () => {
  it("names the character and its owner", () => {
    expect(whoIs(list, 4)).toEqual({ name: "Ilyana", owner: "Ada" });
  });
  it("leaves an owner without a display name blank, for the web's own word", () => {
    expect(whoIs(list, 6)).toEqual({ name: "Bram", owner: "" });
  });
  it("names nothing while the list is loading or when it lacks the character", () => {
    expect(whoIs(null, 4)).toEqual({ name: null, owner: "" });
    expect(whoIs(list, 99)).toEqual({ name: null, owner: "" });
  });
});
