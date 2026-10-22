import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { PowerCheckResult } from "@/api/psionics";
import type { Power } from "@/types/powers";
import { CheckResult, UsePowerForm } from "./UsePowerForm";

async function render(lng: "en" | "fr", node: React.ReactNode) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={inst}>{node}</I18nextProvider>,
  );
}

const power: Power = {
  id: "teleport",
  name: { en: "Teleport", fr: "Téléportation" },
  discipline: "psychoportation",
  kind: "science",
  initial_cost: "variable",
  maintenance: { cost: 3, period: 2 },
  prerequisites: [],
  range: {},
  preparation_time: {},
  area: {},
  description: {},
};

const form = (psp: number, p: Power = power) => (
  <UsePowerForm
    power={p}
    powerName="Teleport"
    currentPsp={psp}
    busy={false}
    onUse={async () => ({ refused: "" })}
  />
);

describe("UsePowerForm", () => {
  it("asks for the variable cost and the score the book leaves to the text", async () => {
    const html = await render("en", form(20));
    expect(html).toContain('aria-label="Initial cost (PSPs)"');
    expect(html).toContain('aria-label="Power score"');
    expect(html).not.toContain('aria-label="Maintenance cost (PSPs)"');
    expect(html).toContain('aria-label="Add a modifier"');
    // Nothing to send yet: the cost is blank.
    expect(html).toMatch(
      /<button[^>]*disabled[^>]*aria-label="Use Teleport"|aria-label="Use Teleport"[^>]*disabled/,
    );
  });

  it("words a power that needs contact in plain words, and in French", async () => {
    const contact = { ...power, initial_cost: "contact" as const };
    expect(await render("en", form(20, contact))).toContain(
      "This power needs contact, which is not available yet.",
    );
    expect(await render("fr", form(20, contact))).toContain(
      "Ce pouvoir demande un contact, pas encore disponible.",
    );
  });

  it("words a fixed cost the PSPs cannot cover", async () => {
    const fixed = {
      ...power,
      initial_cost: 12,
      score: { ability: "int", modifier: 0 },
    };
    const html = await render("en", form(5, fixed));
    expect(html).toContain(
      "Not enough PSPs for this power&#x27;s initial cost.",
    );
    expect(html).toContain("You have 5.");
    expect(await render("en", form(12, fixed))).not.toContain(
      "Not enough PSPs",
    );
  });
});

describe("CheckResult", () => {
  const check: PowerCheckResult = {
    power: "teleport",
    target: 13,
    d20: 9,
    outcome: "success",
    success: true,
    cost: 12,
    spent: 12,
    maintenanceCost: 3,
    maintenancePeriod: 2,
    warnings: ["second_power_this_round"],
  };

  it("shows the roll, the verdict, the spend, the maintenance and the warning", async () => {
    const html = await render("en", <CheckResult result={check} />);
    expect(html).toContain("d20 9 vs 13");
    expect(html).toContain("Success");
    expect(html).toContain("PSPs −12");
    expect(html).toContain("kept up: 3 PSPs every 2 rounds");
    expect(html).toContain("A second power in the same combat round.");
  });

  it("words a failure with half the cost, in French", async () => {
    const html = await render(
      "fr",
      <CheckResult
        result={{
          ...check,
          outcome: "failure",
          success: false,
          spent: 6,
          warnings: [],
        }}
      />,
    );
    expect(html).toContain("d20 9 contre 13");
    expect(html).toContain("Échec");
    expect(html).toContain("PFP −6");
    expect(html).not.toContain("maintenu");
  });
});
