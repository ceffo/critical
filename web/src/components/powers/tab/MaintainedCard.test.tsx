import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { BuffEffect, NextRoundPreview } from "@/types/buffs";
import type { Power } from "@/types/powers";
import { MaintainedCardView } from "./MaintainedCard";
import { maintainedRows } from "./maintained";

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

function buff(over: Partial<BuffEffect> & Pick<BuffEffect, "id">): BuffEffect {
  return {
    name: over.id,
    charges: 0,
    duration: 0,
    effects: {},
    note: "",
    startRound: 0,
    chargesTotal: 0,
    ...over,
  };
}

// Three kinds of row, with ids no code has heard of: a paid power on a 2-round period, a standing
// power, and a contact that a power keeps free.
const rows = [
  buff({
    id: "b1",
    powerId: "zz_beam",
    maintenanceCost: 4,
    maintenancePeriod: 2,
  }),
  buff({ id: "b2", powerId: "zz_shell", maintenanceCost: 0 }),
  buff({ id: "b3", target: "Gardien", maintenanceCost: 1 }),
  buff({ id: "b4", name: "Bless", duration: 5 }),
];
const powers = [
  {
    id: "zz_beam",
    name: { en: "Beam", fr: "Rayon" },
  },
  { id: "zz_shell", name: { en: "Shell", fr: "Coque" } },
] as unknown as Power[];
const preview: NextRoundPreview = {
  pspSpent: 4,
  pspBefore: 67,
  pspAfter: 63,
  rows: [
    { buffId: "b1", charge: 4 },
    { buffId: "b3", charge: 0 },
  ],
};

describe("maintainedRows", () => {
  it("lists the rows that keep something up, whatever their power, with the three cost kinds", () => {
    const r = maintainedRows(rows, preview.rows);
    expect(r.map((x) => [x.id, x.kind])).toEqual([
      ["b1", "paid"],
      ["b2", "standing"],
      ["b3", "contact-free"],
    ]);
    expect(r[0].charge).toBe(4);
  });

  it("a row whose period is not due next round pays nothing then", () => {
    const [r] = maintainedRows(rows, [{ buffId: "b1", charge: 0 }]);
    expect(r.kind).toBe("paid");
    expect(r.notDue).toBe(true);
  });

  it("a row the PSPs cannot pay is flagged, not counted as not due", () => {
    const [r] = maintainedRows(rows, [{ buffId: "b1", charge: 0, ends: true }]);
    expect(r.ends).toBe(true);
    expect(r.notDue).toBe(false);
  });
});

describe("MaintainedCardView", () => {
  async function html(lng: "en" | "fr", p: NextRoundPreview | null = preview) {
    return renderToStaticMarkup(
      <I18nextProvider i18n={await inst(lng)}>
        <MaintainedCardView
          rows={maintainedRows(rows, p?.rows ?? [])}
          powers={powers}
          preview={p}
          busy={false}
          onStop={() => undefined}
        />
      </I18nextProvider>,
    );
  }

  it("draws one row per kind with its cost, free ones as free, and Stop on each", async () => {
    const out = await html("en");
    expect(out.match(/data-testid="maintained-row"/g)).toHaveLength(3);
    expect(out).toContain("Beam");
    expect(out).toContain("4 PSP every 2 rounds");
    expect(out).toContain("Stop Beam");
    expect(out).toContain("Contact · Gardien");
    expect(out).toContain("free while a power uses it");
    expect(out).toContain('data-kind="standing"');
    expect(out.match(/>free</g)).toHaveLength(2);
  });

  it("shows the server's balance as it is, no sum of its own", async () => {
    expect(await html("en")).toContain("67 → 63 PSPs");
    expect(await html("fr")).toContain("67 → 63 PFP");
    expect(await html("fr")).toContain("Rayon");
    expect(await html("en", null)).not.toContain("maintained-balance");
  });
});
