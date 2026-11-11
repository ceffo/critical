import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { HpCondition, HpState } from "@/types/hp";
import { HpBar } from "./HpBar";

// Static rendering reads a store's initial state, so the card's HP store is replaced by a plain one.
let hp: HpState & { loaded: boolean };
vi.mock("@/stores/hp", () => ({ useHpStore: () => hp }));

async function render(lng: string) {
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
      <HpBar />
    </I18nextProvider>,
  );
}

function setHp(currentHp: number, condition: HpCondition) {
  hp = {
    currentHp,
    maxHp: 19,
    floor: -10,
    condition,
    events: [],
    loaded: true,
  };
}

// The condition is the server's (ADR 0011 step 2); the card words what it sends.
const BANDS: [number, HpCondition, string, string][] = [
  [10, "ok", "", ""],
  [4, "badlyWounded", "Badly wounded", "Gravement blessé"],
  [0, "unconscious", "Unconscious", "Inconscient"],
  [-9, "unconscious", "Unconscious", "Inconscient"],
  [-10, "dead", "Dead", "Mort"],
];

describe("HpBar condition line", () => {
  beforeEach(() => setHp(19, "ok"));

  for (const [hp, condition, en, fr] of BANDS) {
    it(`at ${hp} HP (${condition}) the line is ${en || "absent"}`, async () => {
      setHp(hp, condition);
      for (const [lng, text] of [
        ["en", en],
        ["fr", fr],
      ]) {
        const html = await render(lng);
        if (condition === "ok") {
          expect(html).not.toContain("hp-condition");
        } else {
          expect(html).toContain(`data-condition="${condition}"`);
          expect(html).toContain(`>${text}</span>`);
        }
      }
    });
  }

  it("a heal back above 0 removes the line", async () => {
    setHp(-4, "unconscious");
    expect(await render("fr")).toContain("Inconscient");
    setHp(12, "ok");
    expect(await render("fr")).not.toContain("hp-condition");
  });
});
