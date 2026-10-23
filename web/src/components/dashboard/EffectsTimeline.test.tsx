import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import { EffectsTimeline } from "./EffectsTimeline";

// Server rendering reads a zustand store's initial state only, so the round is mocked in.
const state = vi.hoisted(() => ({
  round: 0,
  buffs: [] as import("@/types/buffs").BuffEffect[],
}));
vi.mock("@/stores/buffs", async (orig) => {
  const mod = await orig<typeof import("@/stores/buffs")>();
  const real = mod.useBuffsStore;
  const useBuffsStore = Object.assign(
    (sel?: (s: ReturnType<typeof real.getState>) => unknown) => {
      const s = { ...real.getState(), round: state.round, buffs: state.buffs };
      return sel ? sel(s) : s;
    },
    real,
  );
  return { ...mod, useBuffsStore };
});

async function render(
  round: number,
  lng: string,
  buffs: import("@/types/buffs").BuffEffect[] = [],
) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  state.round = round;
  state.buffs = buffs;
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      <EffectsTimeline />
    </I18nextProvider>,
  );
}

describe("EffectsTimeline round numbers (ADR 0027 step 4.3)", () => {
  it("out of combat (round 0) draws no round numbers and no current column", async () => {
    for (const lng of ["fr", "en"]) {
      const html = await render(0, lng);
      expect(html).toContain('data-testid="effects-track"');
      expect(html).not.toMatch(/>[RT]\d+</);
      expect(html).not.toContain("effects-current-column");
      expect(html).not.toContain("aria-current");
    }
  });

  it("in combat shows the numbers and the current column", async () => {
    const fr = await render(6, "fr");
    expect(fr).toContain(">T6<");
    expect(fr).toContain(">T4<");
    expect(fr).toContain("effects-current-column");
    expect(fr).toContain('aria-current="step"');
    expect(await render(6, "en")).toContain(">R6<");
  });
});

describe("EffectsTimeline rows not on the character (ADR 0004 step 20.5)", () => {
  const web = {
    id: "w",
    name: "Web",
    charges: 0,
    duration: 30,
    effects: {},
    note: "",
    startRound: 2,
    chargesTotal: 0,
    onOther: true,
    applies: false,
  };
  it("draws the row dashed with a lock, worded as not on you, still counting down", async () => {
    const en = await render(3, "en", [web]);
    expect(en).toContain('data-applies="false"');
    expect(en).toContain("not on you");
    expect(en).toContain("only counts down");
    expect(en).toContain("border-dashed");
    expect(en).toContain('data-testid="buff-rounds"');
    expect(en).not.toContain("needs STR");
    const fr = await render(3, "fr", [web]);
    expect(fr).toContain("pas sur vous");
  });
});

describe("EffectsTimeline mind links (ADR 0044 step 3)", () => {
  const link = (tangents?: number) =>
    ({
      id: "t1",
      name: "Orc",
      charges: 0,
      duration: 0,
      effects: {},
      note: "",
      startRound: 1,
      chargesTotal: 0,
      target: "Orc",
      tangents,
    }) as import("@/types/buffs").BuffEffect;

  it("shows the opponent and the tangents a row holds, or its contact", async () => {
    expect(await render(2, "en", [link(2)])).toContain("2 tangents");
    expect(await render(2, "en", [link(1)])).toContain("1 tangent<");
    expect(await render(2, "fr", [link(2)])).toContain("2 tangentes");
    const contact = await render(2, "en", [link()]);
    expect(contact).toContain("Orc");
    expect(contact).toContain("contact");
  });
});
