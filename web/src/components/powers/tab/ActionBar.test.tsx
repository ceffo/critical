import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { Power } from "@/types/powers";
import { attackRequest, EMPTY_ATTACK } from "../attack";
import { requestOf, EMPTY_DRAFT } from "../powerUse";
import { ActionBar, type Selection } from "./ActionBar";
import { NO_TARGET, formOf, stepLevel, type TargetDraft } from "./bar";

async function html(
  lng: "en" | "fr",
  selection: Selection,
  target: TargetDraft = NO_TARGET,
) {
  const i = i18next.createInstance();
  await i.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={i}>
      <ActionBar
        selection={selection}
        target={target}
        modifiers={[]}
        onClose={() => undefined}
      />
    </I18nextProvider>,
  );
}

function power(over: Partial<Power> & Pick<Power, "id" | "kind">): Power {
  return {
    name: { en: over.id.toUpperCase(), fr: `${over.id.toUpperCase()} (fr)` },
    discipline: "d1",
    initial_cost: 5,
    score: { ability: "wis", modifier: 0 },
    prerequisites: [],
    range: {},
    preparation_time: {},
    area: {},
    description: {},
    ...over,
  };
}

// One power per shape, with ids no code has ever heard of: the form is a function of the data.
const attackMode = power({ id: "zz_strike", kind: "attack_mode" });
const variableBoth = power({
  id: "zz_lift",
  kind: "science",
  initial_cost: "variable",
  maintenance: { cost: "variable", period: 1 },
});
const variableInitial = power({
  id: "zz_push",
  kind: "devotion",
  initial_cost: "variable",
});
const variableKeep = power({
  id: "zz_hold",
  kind: "devotion",
  maintenance: { cost: "variable", period: 1 },
});
const numeric = power({
  id: "zz_glow",
  kind: "devotion",
  initial_cost: 3,
  maintenance: { cost: 2, period: 1 },
});
const byTable = power({
  id: "zz_scale",
  kind: "devotion",
  initial_cost: { table: "t" } as unknown as Power["initial_cost"],
});
const noScore = power({ id: "zz_text", kind: "science", score: undefined });

describe("the form the bar opens", () => {
  it("is picked from kind and the shape of the costs", () => {
    expect(formOf(attackMode)).toBe("attack");
    expect(formOf(variableBoth)).toBe("variable");
    expect(formOf(variableInitial)).toBe("variable");
    expect(formOf(variableKeep)).toBe("variable");
    expect(formOf(numeric)).toBe("confirm");
    expect(formOf(byTable)).toBe("confirm");
    expect(formOf(noScore)).toBe("confirm");
  });

  it("shows the attack form with the target's roll fields only while a defence is set", async () => {
    const none = await html("en", { kind: "power", power: attackMode });
    expect(none).toContain('data-form="attack"');
    expect(none).toContain('data-testid="attack-form"');
    expect(none).not.toContain("Roll 1");
    expect(none).toContain("Name the opponent in the Target band.");
    const defended = await html(
      "fr",
      { kind: "power", power: attackMode },
      { ...NO_TARGET, target: "Orc", defence: "zz_shell" },
    );
    expect(defended).toContain("Jet 1");
    expect(defended).toContain("Jet 2");
    expect(defended).not.toContain("Nommez l’adversaire");
  });

  it("shows the cost and maintenance fields only for a variable one", async () => {
    const both = await html("en", { kind: "power", power: variableBoth });
    expect(both).toContain('data-form="variable"');
    expect(both).toContain('aria-label="Initial cost"');
    expect(both).toContain('aria-label="Upkeep per turn"');
    const initial = await html("en", { kind: "power", power: variableInitial });
    expect(initial).toContain('aria-label="Initial cost"');
    expect(initial).not.toContain('aria-label="Upkeep per turn"');
    const keep = await html("en", { kind: "power", power: variableKeep });
    expect(keep).not.toContain('aria-label="Initial cost"');
    expect(keep).toContain('aria-label="Upkeep per turn"');
  });

  it("asks nothing of a numeric or by-table cost: the confirmation only", async () => {
    const num = await html("en", { kind: "power", power: numeric });
    expect(num).toContain('data-form="confirm"');
    expect(num).toContain("3 PSP");
    expect(num).not.toContain("Initial cost");
    expect(num).not.toContain("Power score");
    const table = await html("en", { kind: "power", power: byTable });
    expect(table).toContain('data-form="confirm"');
    expect(table).toContain("Cost set by the power.");
  });

  it("asks for the score of a power the book gives none", async () => {
    const out = await html("en", { kind: "power", power: noScore });
    expect(out).toContain('aria-label="Power score"');
  });

  it("opens the incoming attack with the DM's mode, rolls and the roll to keep", async () => {
    const out = await html("fr", { kind: "incoming" });
    expect(out).toContain('data-form="incoming"');
    expect(out).toContain("Attaque subie");
    expect(out).toContain("Mode d’attaque");
    expect(out).toContain("Jet 1");
    expect(out).toContain("Garder un jet pour le tour");
    expect(out).toContain("Résoudre");
  });
});

describe("what the bar sends", () => {
  it("carries the adjustment as the request's modifier and the target of the band", () => {
    const mods = [{ value: "+2", reason: "Adjustment" }];
    expect(
      requestOf(variableInitial, {
        ...EMPTY_DRAFT,
        cost: "4",
        modifiers: mods,
      }),
    ).toEqual({
      cost: 4,
      modifiers: [{ value: 2, reason: "Adjustment" }],
    });
    expect(
      attackRequest({
        ...EMPTY_ATTACK,
        target: "Orc",
        level: "6",
        defence: "zz_shell",
        rolls: [
          { roll: "8", success: true },
          { roll: "11", success: false },
        ],
        modifiers: mods,
      }),
    ).toEqual({
      target: "Orc",
      mind: "closed",
      level: 6,
      defence: "zz_shell",
      rolls: [
        { roll: 8, success: true },
        { roll: 11, success: false },
      ],
      modifiers: [{ value: 2, reason: "Adjustment" }],
    });
  });

  it("keeps the target's level a whole number from 0", () => {
    expect(stepLevel("", 1)).toBe("1");
    expect(stepLevel("6", -1)).toBe("5");
    expect(stepLevel("0", -1)).toBe("0");
    expect(stepLevel("x", 1)).toBe("1");
  });
});

describe("no power id in the web source", () => {
  it("names none of the catalogue's powers", () => {
    const catalogue = JSON.parse(
      readFileSync(
        join(__dirname, "../../../../../api/testdata/catalogue.json"),
        "utf8",
      ),
    ) as { powers: { id: string }[] };
    // "contact" is also the book's word for a cost paid on contact (a field value, not a power).
    const ids = catalogue.powers
      .map((p) => p.id)
      .filter((id) => id !== "contact");
    const root = join(__dirname, "../../..");
    const sources = (dir: string): string[] =>
      readdirSync(dir).flatMap((f) => {
        const p = join(dir, f);
        if (statSync(p).isDirectory()) return sources(p);
        return /\.(ts|tsx)$/.test(p) && !/\.test\.tsx?$/.test(p) ? [p] : [];
      });
    const found: string[] = [];
    for (const file of sources(root)) {
      // Prose naming a power in a comment is not code: only quoted ids outside comments count.
      const src = readFileSync(file, "utf8")
        .replace(/\/\*[\s\S]*?\*\//g, "")
        .replace(/^\s*\/\/.*$/gm, "");
      for (const id of ids) {
        if (new RegExp(`["'\`]${id}["'\`]`).test(src)) {
          found.push(`${file.slice(root.length + 1)}: ${id}`);
        }
      }
    }
    expect(found).toEqual([]);
  });
});
