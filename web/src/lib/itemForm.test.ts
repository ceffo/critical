import { describe, expect, it } from "vitest";
import type { Item } from "@/types/items";
import {
  changedFields,
  draftFromItem,
  draftToBody,
  emptyDraft,
  parseInteger,
  typeSuggestions,
} from "./itemForm";

const wand: Item = {
  id: "wand_lightning",
  name: { en: "Wand of lightning", fr: "Baguette de foudre" },
  type: "wand",
  behaviour: "charges",
  magicalBonus: 0,
  ammoKind: "",
  count: 4,
  maxCount: 4,
  effectDice: "6d6",
  effectSave: "rod_staff_wand",
  notes: "",
  powers: [],
};

describe("parseInteger", () => {
  it("accepts whole numbers only", () => {
    expect(parseInteger("6")).toBe(6);
    expect(parseInteger(" -3 ")).toBe(-3);
    expect(parseInteger("+2")).toBe(2);
    for (const bad of ["", "x", "1.5", "1e3", "2x6"])
      expect(parseInteger(bad)).toBeNaN();
  });
});

describe("draftToBody", () => {
  it("builds the add body for Arrows +3", () => {
    const { body, errors } = draftToBody(
      {
        ...emptyDraft,
        name: "Arrows +3",
        type: "arrow",
        count: "6",
        magicalBonus: "3",
        ammoKind: "arrow",
      },
      "en",
    );
    expect(errors).toEqual({});
    expect(body).toEqual({
      name: { en: "Arrows +3" },
      type: "arrow",
      behaviour: "stock",
      count: 6,
      magicalBonus: 3,
      ammoKind: "arrow",
      effectDice: "",
      effectSave: "",
      notes: "",
    });
  });

  it("sends the daily number for daily items only", () => {
    expect(
      draftToBody({ ...emptyDraft, behaviour: "daily", maxCount: "2" }, "en")
        .body.maxCount,
    ).toBe(2);
    expect(
      draftToBody({ ...emptyDraft, behaviour: "stock", maxCount: "2" }, "en")
        .body,
    ).not.toHaveProperty("maxCount");
  });

  it("flags text that is not a number instead of sending NaN; a blank bonus is 0", () => {
    const { errors } = draftToBody(
      {
        ...emptyDraft,
        count: "six",
        behaviour: "daily",
        maxCount: "",
      },
      "en",
    );
    expect(errors).toEqual({ count: "notANumber", maxCount: "notANumber" });
    const blank = draftToBody({ ...emptyDraft, magicalBonus: " " }, "en");
    expect(blank.errors).toEqual({});
    expect(blank.body.magicalBonus).toBe(0);
  });

  it("leaves a negative count and bad dice to the server", () => {
    const { body, errors } = draftToBody(
      {
        ...emptyDraft,
        count: "-1",
        effectDice: "2x6",
      },
      "en",
    );
    expect(errors).toEqual({});
    expect(body.count).toBe(-1);
    expect(body.effectDice).toBe("2x6");
  });
});

describe("changedFields", () => {
  it("sends only what changed: the wand's charges 4 to 3", () => {
    const { body } = draftToBody(
      { ...draftFromItem(wand, "en"), count: "3" },
      "en",
    );
    expect(changedFields(wand, body, "en")).toEqual({ count: 3 });
  });

  it("sends the daily number when an item becomes daily, even if equal to its old max", () => {
    const stock: Item = { ...wand, behaviour: "stock", count: 4, maxCount: 4 };
    const { body } = draftToBody(
      {
        ...draftFromItem(stock, "en"),
        behaviour: "daily",
        maxCount: "4",
      },
      "en",
    );
    expect(changedFields(stock, body, "en")).toEqual({
      behaviour: "daily",
      maxCount: 4,
    });
  });

  it("is empty when nothing changed", () => {
    expect(
      changedFields(
        wand,
        draftToBody(draftFromItem(wand, "en"), "en").body,
        "en",
      ),
    ).toEqual({});
  });
});

// ADR 0023 step 11: the form edits the name in the UI language and sends that language only.
describe("names in the player's language", () => {
  it("shows and sends the French name in French", () => {
    const d = draftFromItem(wand, "fr-CA");
    expect(d.name).toBe("Baguette de foudre");
    const { body } = draftToBody({ ...d, name: "Baguette d'éclairs" }, "fr-CA");
    expect(changedFields(wand, body, "fr-CA")).toEqual({
      name: { fr: "Baguette d'éclairs" },
    });
  });

  it("never writes the English fallback into French when the name is untouched", () => {
    const english: Item = { ...wand, name: { en: "Wand of lightning" } };
    const d = draftFromItem(english, "fr");
    expect(d.name).toBe("Wand of lightning");
    expect(changedFields(english, draftToBody(d, "fr").body, "fr")).toEqual({});
  });

  it("creates an item in the UI language", () => {
    expect(
      draftToBody({ ...emptyDraft, name: "Corde" }, "fr").body.name,
    ).toEqual({
      fr: "Corde",
    });
  });
});

describe("typeSuggestions", () => {
  it("lists each existing type once, sorted, without blanks", () => {
    const items = [
      wand,
      { ...wand, id: "a", type: "arrow" },
      { ...wand, id: "b", type: "arrow" },
      { ...wand, id: "c", type: "" },
    ];
    expect(typeSuggestions(items)).toEqual(["arrow", "wand"]);
  });
});
