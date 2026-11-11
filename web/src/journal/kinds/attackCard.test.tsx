// The attack card's damage block per outcome: a settled miss or fumble shows the die, target,
// margin and verdict only; a hit, a critical hit, an automatic hit and an attack still waiting for
// the GM's verdict keep it.
import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { I18nextProvider } from "react-i18next";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { RollResult } from "@/types/combat";
import fixtures from "../fixtures/attack.json";
import { attackKind } from "./attack";

const hit = fixtures.hit.payload as unknown as RollResult;
const crit = fixtures.crit.payload as unknown as RollResult;
const unknownAc = fixtures.unknownAc.payload as unknown as RollResult;

const miss: RollResult = { ...hit, d20Roll: 4, hit: false, hitMargin: -9 };
const fumble: RollResult = { ...hit, d20Roll: 1, hit: false, hitMargin: -12 };
const autoHit: RollResult = { ...hit, d20Roll: 20 };

async function render(r: RollResult, lng: "en" | "fr") {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      {attackKind.card?.({
        payload: r,
        entry: null,
        text: "",
        interactive: false,
      })}
    </I18nextProvider>,
  );
}

const damageParts = [
  'data-testid="total-damage"',
  'data-testid="damage-dice"',
  'data-testid="damage-formula"',
];

describe("attack card damage block", () => {
  for (const lng of ["en", "fr"] as const) {
    const verdict = lng === "en" ? "Miss" : "Raté";
    for (const [name, r] of [
      ["a miss", miss],
      ["a fumble", fumble],
    ] as const) {
      it(`${name} shows no damage block (${lng})`, async () => {
        const html = await render(r, lng);
        for (const part of damageParts) expect(html).not.toContain(part);
        expect(html).toContain(verdict);
        expect(html).toContain('data-testid="attack-tiles"');
        expect(html).toContain(`>${r.d20Roll}<`);
      });
    }
    for (const [name, r] of [
      ["a hit", hit],
      ["a critical hit", crit],
      ["an automatic hit", autoHit],
      ["an attack awaiting the verdict", unknownAc],
    ] as const) {
      it(`${name} keeps its damage block (${lng})`, async () => {
        const html = await render(r, lng);
        for (const part of damageParts) expect(html).toContain(part);
      });
    }
  }
});

// A natural 20 or 1 gets its own treatment on the card: a bordered roll row and a chip with words
// and an icon, never colour alone. Other dice show neither. Driven by the served d20Roll only.
describe("attack card natural 20 and natural 1", () => {
  for (const [lng, n20, n1] of [
    ["en", "Natural 20", "Natural 1"],
    ["fr", "20 naturel", "1 naturel"],
  ] as const) {
    it(`a natural 20 shows the gold border and its chip (${lng})`, async () => {
      const html = await render(autoHit, lng);
      expect(html).toContain('data-natural="20"');
      expect(html).toContain("border-highlight");
      expect(html).toContain('data-testid="natural-chip"');
      expect(html).toContain(n20);
      expect(html).toContain("<svg");
    });
    it(`a natural 1 shows the danger border and its chip (${lng})`, async () => {
      const html = await render(fumble, lng);
      expect(html).toContain('data-natural="1"');
      expect(html).toContain("border-danger");
      expect(html).toContain(n1);
    });
  }
  it("an ordinary roll shows no natural treatment", async () => {
    for (const r of [hit, miss, crit]) {
      const html = await render({ ...r, d20Roll: 12 }, "en");
      expect(html).not.toContain("data-natural");
      expect(html).not.toContain('data-testid="natural-chip"');
    }
  });
});
