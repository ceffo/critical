import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type {
  ProficiencySlots,
  WeaponPickInput,
  WeaponTypeOption,
} from "@/types/character";
import { WeaponPicker } from "./WeaponPicker";

// A mocked server answer whose figures follow no rule the web could recompute: every number on the
// screen must be one of these (ADR 0037 steps 3, 7 and 9; the web words them, computes none).
const figures = (left: number, spent: number): ProficiencySlots => ({
  nonWeapon: {
    initial: 0,
    perLevels: 0,
    levelGained: 0,
    intelligence: 0,
    available: 0,
    spent: 0,
    left: 0,
    languages: 0,
  },
  picks: [],
  weapon: {
    initial: 7,
    perLevels: 5,
    levelGained: 9,
    available: 17,
    spent,
    left,
  },
  weaponPicks: [
    { type: "long_sword", slots: 6, allowed: true },
    { type: "heavy_crossbow", slots: 1, allowed: false },
  ],
});

// The specialization costs come from the options; these are no book figure.
const types: WeaponTypeOption[] = [
  { type: "long_bow", specializationSlots: 11 },
  { type: "long_sword", specializationSlots: 6 },
  { type: "dagger", specializationSlots: 4 },
];

async function render(
  f: ProficiencySlots,
  picks: WeaponPickInput[],
  lng = "en",
  list = types,
  error: string | null = null,
) {
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
      <WeaponPicker
        types={list}
        picks={picks}
        onChange={() => {}}
        figures={f}
        errorFor={(field) => (field === "weapon.long_bow" ? error : null)}
      />
    </I18nextProvider>,
  );
}

const picks = [
  { type: "long_sword", slots: 6 },
  { type: "heavy_crossbow", slots: 1 },
  { type: "dagger", slots: 1 },
];

describe("WeaponPicker", () => {
  it("shows the server's pool, the specialization, its cost and a kept type the class may not pick", async () => {
    const html = await render(figures(3, 14), picks);
    expect(html).toContain(
      "Slots: 7 to start + 9 for the levels (one every 5) = 17",
    );
    expect(html).toContain("Spent 14, left 3");
    expect(html).toContain("3 slots to spend");
    expect(html).toContain("Specialization · 6 slots");
    // The dagger's specialization cost, as the options give it.
    expect(html).toContain(
      "Proficient · 1 slot · specialization: 4 slots in all",
    );
    expect(html).toContain("Specialize in Dagger: 4 slots in all");
    // An unpicked type is added first; its specialization comes with the pick.
    expect(html).not.toContain("Specialize in Long bow");
    // A kept type outside the class's list is shown, marked, removable.
    expect(html).toContain('data-testid="weapon-row-heavy_crossbow"');
    expect(html).toContain(
      "Outside your class’s weapons: kept, you may remove it",
    );
    expect(html).toContain("Remove Heavy crossbow");
  });

  it("warns of an overspend in French and shows a row's refusal", async () => {
    const html = await render(
      figures(-2, 19),
      picks,
      "fr",
      types,
      "Une seule spécialisation, et seulement pour un combattant.",
    );
    expect(html).toContain(
      "Budget dépassé : 19 emplacements pour 17 disponibles. Vous pouvez en retirer, pas en ajouter.",
    );
    expect(html).toContain("Spécialisation · 6 emplacements");
    expect(html).toContain("Spécialisée");
    expect(html).toContain(
      "Une seule spécialisation, et seulement pour un combattant.",
    );
  });

  it("offers no specialization to a class whose options give none", async () => {
    const html = await render(
      figures(0, 1),
      [{ type: "dagger", slots: 1 }],
      "en",
      [
        { type: "dagger", specializationSlots: 0 },
        { type: "dart", specializationSlots: 0 },
      ],
    );
    expect(html).not.toContain("weapon-specialize");
    expect(html).not.toContain("specialization");
    expect(html).toContain("Become proficient with Dart (1 slot)");
  });
});
