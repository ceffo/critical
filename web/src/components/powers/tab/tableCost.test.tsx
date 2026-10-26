import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import i18next from "i18next";
import { I18nextProvider } from "react-i18next";
import { resources } from "@/i18n";
import type { CostTable } from "@/api/psionics";
import type { Power } from "@/types/powers";
import { NO_TARGET, formOf } from "./bar";
import { TableControls, TablePreview } from "./ActionBar";
import {
  controlsOf,
  initialPicks,
  pickedRows,
  tableRequest,
} from "./tableCost";

// A made-up table: kinds the web has never heard of.
const table: CostTable = {
  table: "zz_cost",
  modifiers: [
    { kind: "weather", code: "calm", modifier: 0 },
    { kind: "weather", code: "storm", modifier: -3 },
    { kind: "weather", code: "fog", modifier: -1 },
    { kind: "mood", code: "angry", modifier: -2 },
  ],
};

async function render(node: React.ReactNode) {
  const i = i18next.createInstance();
  await i.init({
    lng: "en",
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return renderToStaticMarkup(
    <I18nextProvider i18n={i}>{node}</I18nextProvider>,
  );
}

describe("the table-cost form", () => {
  it("builds a menu for a kind with several rows and a toggle for one row", async () => {
    const controls = controlsOf(table);
    expect(controls.map((c) => [c.kind, c.as])).toEqual([
      ["weather", "menu"],
      ["mood", "toggle"],
    ]);
    const html = await render(
      <TableControls
        controls={controls}
        picks={initialPicks(controls)}
        onPick={() => undefined}
      />,
    );
    expect(html).toContain("<select");
    expect(html).toContain('type="checkbox"');
    expect(html).toContain("storm -3");
  });

  it("sends the level and the rows picked, no band lookup", () => {
    const controls = controlsOf(table);
    const picks = {
      ...initialPicks(controls),
      weather: "storm",
      mood: "angry",
    };
    expect(pickedRows(controls, picks)).toEqual([
      { kind: "weather", code: "storm" },
      { kind: "mood", code: "angry" },
    ]);
    const target = {
      ...NO_TARGET,
      target: "Orc",
      mind: "open" as const,
      level: "7",
    };
    expect(tableRequest(target, controls, picks, [])).toEqual({
      target: "Orc",
      mind: "open",
      level: 7,
      modifiers: pickedRows(controls, picks),
      labelled: [],
    });
    expect(
      tableRequest(target, controls, initialPicks(controls), [])?.modifiers,
    ).toEqual([{ kind: "weather", code: "calm" }]);
    expect(
      tableRequest({ ...target, level: "" }, controls, picks, []),
    ).toBeNull();
    expect(tableRequest(target, controls, picks, [], "13")?.cost).toBe(13);
  });

  it("shows the server's cost, as it came", async () => {
    const html = await render(
      <TablePreview
        preview={{
          cost: 9,
          fromTable: true,
          target: 12,
          lines: [],
          current: 4,
          enough: false,
        }}
      />,
    );
    expect(html).toContain("Cost 9 PSPs");
    expect(html).toContain("score 12");
    expect(html).toContain("Not enough PSPs (4 left)");
  });

  it("is the form of a power whose cost names a table, whatever the table's name", () => {
    const p = {
      id: "zz_x",
      kind: "devotion",
      initial_cost: "variable",
      initial_cost_table: "no_such_table",
    } as unknown as Power;
    expect(formOf(p)).toBe("table");
  });
});
