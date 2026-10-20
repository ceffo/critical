import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { CreationOptions } from "@/types/character";
import { LanguagesPool } from "./LanguagesPool";

// The catalogue and an elf's PHB list, as the creation options serve them.
const options = {
  languages: [
    { code: "common", name: { en: "Common", fr: "Commun" } },
    { code: "elvish", name: { en: "Elvish", fr: "Elfique" } },
    { code: "gnomish", name: { en: "Gnomish", fr: "Gnome" } },
    { code: "burrowers", name: { en: "Burrowers", fr: "Fouisseurs" } },
  ],
  raceLanguages: [
    {
      race: "elf",
      native: "elvish",
      languages: ["common", "elvish", "gnomish"],
    },
  ],
} as unknown as CreationOptions;

async function render(
  props: Partial<Parameters<typeof LanguagesPool>[0]>,
  lng = "fr",
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
      <LanguagesPool
        options={options}
        race="elf"
        picked={["elvish", "common", "gnomish"]}
        slots={1}
        error={null}
        onChange={() => {}}
        {...props}
      />
    </I18nextProvider>,
  );
}

const checkbox = (html: string, code: string) =>
  html.match(
    new RegExp(`<input[^>]*id="proficiency-languages-${code}"[^>]*>`),
  )?.[0] ?? "";

describe("LanguagesPool (ADR 0037 step 18)", () => {
  it("offers the catalogue, the race's list first, the native one ticked and fixed", async () => {
    const html = await render({});
    expect(html).toContain("Langues");
    expect(html.indexOf("Gnome")).toBeLessThan(html.indexOf("Fouisseurs"));
    expect(checkbox(html, "elvish")).toContain("disabled");
    expect(checkbox(html, "elvish")).toContain("checked");
    expect(checkbox(html, "gnomish")).toContain("checked");
    expect(checkbox(html, "burrowers")).not.toContain("checked");
  });

  it("shows the slots the server says the languages spend, none when 0", async () => {
    const html = await render({ slots: 7 });
    expect(html).toMatch(
      /data-testid="proficiency-languages-slots"[^>]*>[^<]*7/,
    );
    expect(await render({ slots: 0 })).not.toContain(
      "proficiency-languages-slots",
    );
  });

  it("shows the refusal on field languages as worded", async () => {
    const html = await render({ error: "Over the slots" });
    expect(html).toContain('role="alert"');
    expect(html).toContain("Over the slots");
  });

  it("offers nothing without a catalogue", async () => {
    const html = await render({ options: {} as CreationOptions });
    expect(html).toBe("");
  });
});
