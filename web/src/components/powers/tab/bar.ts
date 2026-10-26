// What the action bar of the Powers tab shows for a power, as rules on its fields (never an id): which
// form opens, and the target the attack names. Pure and store-free.
import type { AttackDraft } from "../attack";
import type { Power } from "@/types/powers";
import { asks } from "../powerUse";

/**
 * The form a power opens in the bar (ADR 0047 item 2), from the shape of its data:
 * - `attack`: an attack mode, used on an opponent with the DM's two rolls;
 * - `variable`: a variable initial cost or maintenance, the cost fields the data asks for;
 * - `contact`: a cost paid on contact, used on a mind the character made contact with earlier;
 * - `table`: a cost a rule table gives (`initial_cost_table`, whatever its name), used on an opponent
 *   with the table's modifier rows picked;
 * - `confirm`: a numeric or by-level cost, the confirmation only (the score field still shows for a
 *   power the book gives no score).
 */
export type BarForm = "attack" | "contact" | "table" | "variable" | "confirm";

export function formOf(p: Power): BarForm {
  if (p.kind === "attack_mode") return "attack";
  if (p.initial_cost_table) return "table";
  if (p.initial_cost === "contact") return "contact";
  const need = asks(p);
  return need.cost || need.maintenanceCost ? "variable" : "confirm";
}

/** The PSPs a power costs at once when its data gives a number; undefined while the cost is entered or set elsewhere. */
export const fixedCost = (p: Power): number | undefined =>
  typeof p.initial_cost === "number" ? p.initial_cost : undefined;

/** The target the attack names, kept by the tab and entered in the "Target" band. */
export type TargetDraft = Pick<
  AttackDraft,
  "target" | "mind" | "level" | "defence"
>;

export const NO_TARGET: TargetDraft = {
  target: "",
  mind: "closed",
  level: "",
  defence: "",
};

/** The level stepper's bounds: a level is a whole number from 0 (unknown) up. */
export const MAX_TARGET_LEVEL = 30;

export const stepLevel = (level: string, by: number): string => {
  const n = Number.parseInt(level, 10);
  const next = Math.max(
    0,
    Math.min(MAX_TARGET_LEVEL, (Number.isNaN(n) ? 0 : n) + by),
  );
  return String(next);
};
