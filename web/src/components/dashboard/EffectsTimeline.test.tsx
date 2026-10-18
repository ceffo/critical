import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import { EffectsTimeline } from "./EffectsTimeline";

// Server rendering reads a zustand store's initial state only, so the round is mocked in.
const state = vi.hoisted(() => ({ round: 0 }));
vi.mock("@/stores/buffs", async (orig) => {
  const mod = await orig<typeof import("@/stores/buffs")>();
  const real = mod.useBuffsStore;
  const useBuffsStore = Object.assign(
    (sel: (s: ReturnType<typeof real.getState>) => unknown) =>
      sel({ ...real.getState(), round: state.round }),
    real,
  );
  return { ...mod, useBuffsStore };
});

async function render(round: number, lng: string) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  state.round = round;
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
