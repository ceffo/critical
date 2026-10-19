import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { I18nextProvider } from "react-i18next";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { TurnResult } from "@/types/character";
import { turnUndeadKind } from "./kinds/turnUndead";

async function render(turn: TurnResult) {
  const inst = i18next.createInstance();
  await inst.init({
    lng: "fr",
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      {turnUndeadKind.card?.({
        payload: turn,
        entry: null,
        text: "",
        interactive: true,
      })}
    </I18nextProvider>,
  );
}

// ADR 0030 step 16.4: a D* turn destroys the 2d6 and turns the 2d4, two counts the server works out,
// each with its dice; a T turn has the turned count only.
describe("turning undead card", () => {
  it("shows a D* turn's destroyed and turned counts apart", async () => {
    const html = await render({
      undead: "skeleton",
      clericLevel: 8,
      outcome: "destroy_more",
      success: true,
      effect: "destroy",
      affected: [2, 3],
      more: [1, 2],
      destroyed: 5,
      turned: 3,
    });
    expect(html).toContain("Détruits : 5 (2 + 3)");
    expect(html).toContain("Repoussés : 3 (1 + 2)");
    expect(html).not.toMatch(/\dd\d/);
  });

  it("shows a turned count only for a T cell", async () => {
    const html = await render({
      undead: "skeleton",
      clericLevel: 3,
      outcome: "turn",
      success: true,
      effect: "turn",
      affected: [3, 4],
      destroyed: 0,
      turned: 7,
    });
    expect(html).toContain("Repoussés : 7 (3 + 4)");
    expect(html).not.toContain("turn-destroyed");
  });
});
