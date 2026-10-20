import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { ProficiencySlots } from "@/types/character";
import { ProficiencyPicker, type PickerOptions } from "./ProficiencyPicker";

// A mocked server answer whose figures follow no rule the web could recompute: every number on the
// screen must be one of these (ADR 0037 steps 3 to 5; the web words them, computes none).
const figures = (left: number, spent: number): ProficiencySlots => ({
  nonWeapon: {
    initial: 3,
    perLevels: 4,
    levelGained: 11,
    intelligence: 13,
    available: 41,
    spent,
    left,
    languages: 0,
  },
  picks: [
    {
      id: "tumbling",
      slots: 6,
      cost: 5,
      extraSlots: 2,
      ability: "dex",
      modifier: 0,
      known: true,
      target: 23,
    },
  ],
});

const options: PickerOptions = {
  proficiencies: [
    {
      id: "tumbling",
      ability: "dex",
      modifier: 0,
      slots: 1,
      groups: ["rogue"],
    },
    {
      id: "tracking",
      ability: "wis",
      modifier: 0,
      slots: 2,
      groups: ["warrior"],
    },
  ],
  proficiencyGroups: { bard: ["rogue", "general"] },
  proficiencyCosts: { bard: { tumbling: 5, tracking: 8 } },
};

async function render(f: ProficiencySlots, lng = "en") {
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
      <ProficiencyPicker
        options={options}
        cls="bard"
        picks={[{ id: "tumbling", slots: 6 }]}
        onChange={() => {}}
        figures={f}
        errorFor={() => null}
      />
    </I18nextProvider>,
  );
}

describe("ProficiencyPicker", () => {
  it("shows the server's pool, the pick's cost, extra slots and target, and the catalogue's cost", async () => {
    const html = await render(figures(7, 34));
    expect(html).toContain(
      "Slots: 3 initial + 11 for the levels (one every 4) + 13 for Intelligence = 41",
    );
    expect(html).toContain("Spent 34, left 7");
    expect(html).toContain("7 slots to spend");
    expect(html).toContain("cost 5 · +2 to the check · target DEX 23");
    // The catalogue: the class's groups first, the other's cost as the server sent it.
    expect(html).toContain("Your class&#x27;s groups: rogue, general");
    expect(html).toContain("Add Tracking (costs 8 slots)");
    expect(html).toContain("8 slots");
    expect(html).toContain("Tumbling: already chosen");
  });

  it("warns of an overspend with the server's figures, in French", async () => {
    const html = await render(figures(-2, 43), "fr");
    expect(html).toContain(
      "Budget dépassé : 43 emplacements pour 41 disponibles. Vous pouvez en retirer, pas en ajouter.",
    );
    expect(html).not.toContain("à dépenser");
    expect(html).toContain("coût 5 · +2 au jet · cible DEX 23");
  });

  it("shows the languages' slots on their own line, as the server counts them (ADR 0037 step 6)", async () => {
    const f = figures(7, 34);
    const html = await render({
      ...f,
      nonWeapon: { ...f.nonWeapon, languages: 3 },
    });
    expect(html).toContain("Languages: 3 slots");
    expect(await render(figures(7, 34))).not.toContain("Languages:");
    const fr = await render(
      { ...f, nonWeapon: { ...f.nonWeapon, languages: 1 } },
      "fr",
    );
    expect(fr).toContain("Langues : 1 emplacement");
  });

  it("opens the description from the name and adds from its own button, in the catalogue and in the picks", async () => {
    const html = await render(figures(1, 40));
    const row =
      /<li[^>]*><button[^>]*data-testid="proficiency-name-tracking"[\s\S]*?<\/li>/.exec(
        html,
      )?.[0];
    expect(row).toContain('aria-label="Description of Tracking"');
    expect(row).toContain('data-testid="proficiency-add-tracking"');
    expect(row?.match(/<button/g)).toHaveLength(2);
    // A pick's name opens it too.
    expect(html).toContain('data-testid="proficiency-name-tumbling"');
  });
});
