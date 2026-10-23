import { describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import { ClassWarningTag } from "./ClassWarning";

// Server rendering reads a zustand store's initial state only, so the character is mocked in.
const state = vi.hoisted(() => ({
  character: null as unknown,
}));
vi.mock("@/stores/character", () => ({
  useCharacterStore: (sel: (s: { character: unknown }) => unknown) =>
    sel({ character: state.character }),
}));

async function render(character: unknown) {
  const inst = i18next.createInstance();
  await inst.init({
    lng: "en",
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  state.character = character;
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>
      <ClassWarningTag code="armour_not_allowed" />
    </I18nextProvider>,
  );
}

describe("ClassWarningTag names the served primary class", () => {
  it("reads primaryClass, not the first listed entry (a dual-class's frozen first entry)", async () => {
    const html = await render({
      primaryClass: "mage",
      classes: [
        { class: "fighter", level: 9, state: "frozen" },
        { class: "mage", level: 10, state: "active" },
      ],
    });
    const mage = await render({ primaryClass: "mage", classes: [] });
    const fighter = await render({ primaryClass: "fighter", classes: [] });
    expect(html).toBe(mage);
    expect(html).not.toBe(fighter);
  });
});
