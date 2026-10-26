// The form of a power paid by a rule table (ADR 0044 step 9): its controls are built from the table's
// modifier rows (data, never a kind or a code written here), and what it sends is the level, the rows
// picked and the labelled modifiers. The cost, the score and the refusals are the server's preview:
// nothing here searches the table for a level.
import type { ContactRequest, CostTable, PowerModifier } from "@/api/psionics";
import type { TargetDraft } from "./bar";

/** One control: a kind of the table's modifier rows. One row is a toggle, several a menu. */
export interface Control {
  kind: string;
  rows: { code: string; modifier: number }[];
  as: "toggle" | "menu";
}

/** The controls of a table, one per kind in the order the rows first name it. */
export function controlsOf(table: CostTable): Control[] {
  const byKind = new Map<string, Control>();
  for (const r of table.modifiers) {
    let c = byKind.get(r.kind);
    if (!c) {
      c = { kind: r.kind, rows: [], as: "toggle" };
      byKind.set(r.kind, c);
    }
    c.rows.push({ code: r.code, modifier: r.modifier });
  }
  const out = [...byKind.values()];
  for (const c of out) c.as = c.rows.length > 1 ? "menu" : "toggle";
  return out;
}

/** The code chosen for each kind: a menu starts on its first row, a toggle off (""). */
export type Picks = Record<string, string>;

export function initialPicks(controls: Control[]): Picks {
  return Object.fromEntries(
    controls.map((c) => [c.kind, c.as === "menu" ? c.rows[0].code : ""]),
  );
}

/** The rows picked, in the order of the controls; a toggle that is off sends nothing. */
export function pickedRows(
  controls: Control[],
  picks: Picks,
): { kind: string; code: string }[] {
  return controls.flatMap((c) =>
    picks[c.kind] ? [{ kind: c.kind, code: picks[c.kind] }] : [],
  );
}

const whole = (s: string): number | null =>
  /^\d+$/.test(s.trim()) ? Number.parseInt(s.trim(), 10) : null;

/**
 * The request the form makes, or null while the target is unnamed or its level is not a whole number
 * from 1 (the table's cost needs it). `cost` is the DM's, sent only once the server said the table
 * has no row for the level.
 */
export function tableRequest(
  target: TargetDraft,
  controls: Control[],
  picks: Picks,
  labelled: PowerModifier[],
  cost?: string,
): ContactRequest | null {
  const name = target.target.trim();
  const level = whole(target.level);
  if (!name || level === null || level < 1) return null;
  const req: ContactRequest = {
    target: name,
    mind: target.mind,
    level,
    modifiers: pickedRows(controls, picks),
    labelled,
  };
  if (cost !== undefined) {
    const n = whole(cost);
    if (n === null || n < 1) return null;
    req.cost = n;
  }
  return req;
}
