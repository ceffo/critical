import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { I18nextProvider } from "react-i18next";
import i18next from "i18next";
import { resources } from "@/i18n";
import { HouseRulesCard } from "./HouseRulesCard";

async function render(
  lng: "en" | "fr",
  weaponOn: boolean,
  editable: boolean | undefined,
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
      <HouseRulesCard
        initial={{ multiclassAnyRace: true, dualclassAnyRace: true }}
        weaponLift={{ multiclassAnyWeapon: weaponOn }}
        levelLimits="warn"
        editable={editable}
      />
    </I18nextProvider>,
  );
}

/** The weapon switch's own tag. */
const weaponBox = (html: string) =>
  /<input[^>]*data-testid="ruleset-weapon-lift"[^>]*>/.exec(html)?.[0] ?? "";

// ADR 0041 step 2.5: the multi-class weapon lift is a house-rule switch, off by default, an admin's
// to change and anyone's to read.
describe("house rules card: the weapon lift", () => {
  it("shows the switch unticked when the ruleset has it off, in French", async () => {
    const html = await render("fr", false, true);
    expect(html).toContain("Multi-classé : toute arme");
    const box = weaponBox(html);
    expect(box).not.toContain("checked");
    expect(box).not.toContain("disabled");
  });

  it("shows the switch ticked when the ruleset has it on", async () => {
    const box = weaponBox(await render("en", true, true));
    expect(box).toContain('checked=""');
  });

  it("shows every switch read-only to a non-editor, with the state kept", async () => {
    const html = await render("en", true, false);
    const box = weaponBox(html);
    expect(box).toContain('checked=""');
    expect(box).toContain('disabled=""');
    for (const input of html.match(/<input[^>]*>/g) ?? [])
      expect(input).toContain('disabled=""');
  });

  it("has a wording in both languages that says specialization is unchanged", async () => {
    expect(await render("en", false, true)).toContain(
      "Weapon specialization is not changed.",
    );
    expect(await render("fr", false, true)).toContain(
      "La spécialisation d&#x27;arme n&#x27;est pas modifiée.",
    );
  });
});
