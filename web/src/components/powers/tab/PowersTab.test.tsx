import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { Power, PowerFilterOptions } from "@/types/powers";
import {
  adjustmentModifiers,
  groupsOf,
  inLearningOrder,
  rowTags,
  takesMind,
} from "./model";
import { PowersView } from "./PowersTab";

async function inst(lng: "en" | "fr") {
  const i = i18next.createInstance();
  await i.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return i;
}

function power(over: Partial<Power> & Pick<Power, "id" | "kind">): Power {
  return {
    name: { en: over.id.toUpperCase(), fr: `${over.id.toUpperCase()} (fr)` },
    discipline: "d1",
    initial_cost: 5,
    prerequisites: [],
    range: {},
    preparation_time: {},
    area: {},
    description: {},
    ...over,
  };
}

const options: PowerFilterOptions = {
  disciplines: [
    { id: "d1", name: { en: "First discipline", fr: "Première discipline" } },
    { id: "d2", name: { en: "Second discipline", fr: "Seconde discipline" } },
  ],
  kinds: ["science", "devotion", "defence_mode", "attack_mode"],
};

// Two sets of known powers with ids no code has ever heard of: the tab is a function of the data.
const withMinds: Power[] = [
  power({ id: "zz_strike", kind: "attack_mode", initial_cost: 4 }),
  power({
    id: "zz_shell",
    kind: "defence_mode",
    initial_cost: 2,
    maintenance: { cost: 0, period: 1 },
  }),
  power({ id: "zz_link", kind: "science", initial_cost: "contact" }),
  power({
    id: "zz_beam",
    kind: "devotion",
    discipline: "d2",
    maintenance: { cost: 3, period: 2 },
  }),
];
const withoutMinds: Power[] = [
  power({ id: "yy_glow", kind: "devotion", discipline: "d2", initial_cost: 1 }),
  power({
    id: "yy_wall",
    kind: "defence_mode",
    initial_cost: 7,
    maintenance: { cost: 1, period: 1 },
  }),
];

async function html(lng: "en" | "fr", powers: Power[], used?: Power) {
  const i = await inst(lng);
  return renderToStaticMarkup(
    <I18nextProvider i18n={i}>
      <PowersView powers={powers} used={used} options={options} />
    </I18nextProvider>,
  );
}

describe("the Powers tab", () => {
  it("shows the powers it is given, with the target band for a mind-taking set", async () => {
    const out = await html("en", withMinds);
    for (const id of ["ZZ_STRIKE", "ZZ_LINK", "ZZ_BEAM", "ZZ_SHELL"]) {
      expect(out).toContain(id);
    }
    expect(out).toContain('data-testid="target-band"');
    // The attack modes come first, then the disciplines in learning order.
    expect([...out.matchAll(/data-group="([^"]+)"/g)].map((m) => m[1])).toEqual(
      ["attack", "d-d1", "d-d2"],
    );
    expect(out).toContain("Attack modes");
    expect(out).toContain("Second discipline");
    expect(out).toContain("Power this round: free");
    expect(out).not.toContain("YY_GLOW");
  });

  it("shows a different set with no band for a mind and its own defence chips", async () => {
    const out = await html("fr", withoutMinds);
    expect(out).toContain("YY_GLOW");
    expect(out).toContain("YY_WALL");
    expect(out).not.toContain("ZZ_");
    expect(out).not.toContain('data-testid="target-band"');
    expect(out).not.toContain('data-group="attack"');
    expect(out).toContain("Ma défense");
    expect(out).toContain("Pouvoir du tour : libre");
    // Out of combat a paid defence is dashed and says when it can be raised.
    expect(out).toContain("7 PFP · en combat");
  });

  it("names the power of the round", async () => {
    const out = await html("en", withMinds, withMinds[0]);
    expect(out).toContain("Power this round: ZZ_STRIKE");
  });
});

describe("the tab's rules on a power's fields", () => {
  it("takes a mind for an attack mode or a cost on contact only", () => {
    expect(withMinds.filter(takesMind).map((p) => p.id)).toEqual([
      "zz_strike",
      "zz_link",
    ]);
    expect(withoutMinds.some(takesMind)).toBe(false);
  });

  it("tags a row from its kind, cost and maintenance", async () => {
    const t = (await inst("en")).t;
    expect(rowTags(t, withMinds[3])).toEqual([
      "Devotion",
      "5 PSP",
      "+3 every 2 rounds",
    ]);
    expect(rowTags(t, withMinds[1])).toEqual(["2 PSP", "standing"]);
  });

  it("groups by discipline in the order learned and keeps defences out", () => {
    expect(groupsOf(withMinds).map((g) => g.key)).toEqual([
      "attack",
      "d-d1",
      "d-d2",
    ]);
    expect(groupsOf(withoutMinds).map((g) => g.key)).toEqual(["d-d2"]);
  });

  it("lists the known powers in the order of the picks, a relearn once", () => {
    const byId = Object.fromEntries(withMinds.map((p) => [p.id, p]));
    const out = inLearningOrder(
      [
        { kind: "discipline", id: "d1" },
        { kind: "power", id: "zz_beam" },
        { kind: "power", id: "zz_link" },
        { kind: "relearn", id: "zz_beam" },
        { kind: "power", id: "gone" },
      ],
      byId,
    );
    expect(out.map((p) => p.id)).toEqual(["zz_beam", "zz_link"]);
  });
});

describe("the adjustment", () => {
  it("is one named modifier while nonzero and nothing at zero", () => {
    expect(adjustmentModifiers(0, "Adjustment")).toEqual([]);
    expect(adjustmentModifiers(3, "Adjustment")).toEqual([
      { value: "+3", reason: "Adjustment" },
    ]);
    expect(adjustmentModifiers(-2, "Adjustment")).toEqual([
      { value: "-2", reason: "Adjustment" },
    ]);
  });
});
