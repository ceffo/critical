import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import { ProficiencyDetail, type ProficiencyFacts } from "./ProficiencyDialog";

const TEXT =
  "Invented first paragraph.\n\n- an item\n- another item\n\n| Roll | Result |\n| --- | --- |\n| 1 | red |";

const facts: ProficiencyFacts = {
  id: "tumbling",
  ability: "dex",
  modifier: -1,
  slots: 3,
  extraSlots: 2,
  cost: 1,
};

async function render(
  props: Partial<Parameters<typeof ProficiencyDetail>[0]> = {},
  lng = "en",
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
      <ProficiencyDetail
        facts={facts}
        groups={["rogue", "general"]}
        description={TEXT}
        {...props}
      />
    </I18nextProvider>,
  );
}

describe("ProficiencyDetail (the pop-up's body)", () => {
  it("renders the description with the spell renderer: paragraph, list and a table", async () => {
    const html = await render();
    expect(html).toContain("Invented first paragraph.");
    expect(html).toContain("<li>");
    expect(html).toContain("<table");
    expect(html).toContain("spell-table");
  });

  it("shows the server's figures: ability, modifier, cost, slots, extra slots and groups", async () => {
    const html = await render();
    const figure = (key: string) =>
      new RegExp(`proficiency-figure-${key}"[^>]*>.*?</div>`).exec(html)?.[0];
    expect(figure("ability")).toContain("DEX");
    expect(figure("modifier")).toContain("−1");
    expect(figure("cost")).toContain(">1<");
    expect(figure("slots")).toContain(">3<");
    expect(figure("extra")).toContain(">2<");
    expect(figure("groups")).toContain("rogue, general");
  });

  it("leaves out a figure the caller does not have, and the modifier of a proficiency with no check", async () => {
    const html = await render(
      { facts: { id: "religion", ability: "", modifier: 0 }, groups: [] },
      "en",
    );
    expect(html).toContain("no check");
    expect(html).not.toContain("proficiency-figure-modifier");
    expect(html).not.toContain("proficiency-figure-slots");
    expect(html).not.toContain("proficiency-figure-cost");
    expect(html).not.toContain("proficiency-figure-groups");
  });

  it("words the figures in French", async () => {
    const html = await render({}, "fr");
    expect(html).toContain("Emplacements dépensés");
    expect(html).toContain("Coût pour votre classe");
  });

  it("says it is loading, then that it failed", async () => {
    expect(await render({ description: undefined })).toContain("Loading");
    const failed = await render({ description: undefined, failed: true });
    expect(failed).toContain('role="alert"');
  });
});
