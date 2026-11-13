import { describe, expect, it } from "vitest";
import { pieceFixture } from "@/test/pieceFixture";
import { combatItems } from "./combatItems";

// The Act · Items tab lists what the server says can be used, never ammunition.
describe("combatItems", () => {
  it("keeps only the potion for a fighter with plate mail, a shield, a longsword and a potion", () => {
    const items = [
      pieceFixture({ id: "plate", category: "armour", usable: false }),
      pieceFixture({ id: "shield", category: "shield", usable: false }),
      pieceFixture({ id: "sword", category: "weapon", usable: false }),
      pieceFixture({ id: "potion", category: "magic", magicKind: "potion" }),
    ];
    expect(combatItems(items).map((i) => i.id)).toEqual(["potion"]);
  });

  it("drops a weapon whatever its behaviour or count, and ammunition", () => {
    const items = [
      pieceFixture({
        id: "dagger",
        category: "weapon",
        usable: false,
        behaviour: "charges",
        count: 3,
        effectDice: "1d4",
      }),
      pieceFixture({ id: "arrows", category: "ammunition", ammoKind: "arrow" }),
      pieceFixture({ id: "torch", category: "gear" }),
    ];
    expect(combatItems(items).map((i) => i.id)).toEqual(["torch"]);
  });
});
