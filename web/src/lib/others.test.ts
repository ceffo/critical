import { describe, expect, it } from "vitest";
import type { SharedCharacter } from "@/types/character";
import { groupByOwner } from "./others";

const c = (
  id: number,
  ownerId: string,
  ownerName: string,
): SharedCharacter => ({
  id,
  name: `C${id}`,
  classes: [],
  owner: { id: ownerId, name: ownerName },
});

describe("groupByOwner", () => {
  it("groups by owner and keeps the server's order of owners and characters", () => {
    const groups = groupByOwner([
      c(5, "a", "Ada"),
      c(2, "a", "Ada"),
      c(9, "b", "Bo"),
      c(1, "c", ""),
    ]);
    expect(
      groups.map((g) => [g.id, g.name, g.characters.map((x) => x.id)]),
    ).toEqual([
      ["a", "Ada", [5, 2]],
      ["b", "Bo", [9]],
      ["c", "", [1]],
    ]);
  });

  it("keeps two owners with the same name apart", () => {
    expect(
      groupByOwner([c(1, "a", "Sam"), c(2, "b", "Sam")]).map((g) => g.id),
    ).toEqual(["a", "b"]);
  });

  it("trims a name and is empty for no characters", () => {
    expect(groupByOwner([c(1, "a", "  Ada ")])[0]!.name).toBe("Ada");
    expect(groupByOwner([])).toEqual([]);
  });
});
