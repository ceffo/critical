import type { TFunction } from "i18next";

/** "20 + 15 × 14 = 230": a budget's formula, written with the figures the server sends. */
export function formula(
  t: TFunction,
  first: number,
  perLevel: number,
  level: number,
  total: number,
) {
  return t("skills.formula", { first, perLevel, steps: level - 1, total });
}

/** The creation step's budget formula for the figures the draft's derived block sends. */
export function budgetFormula(
  t: TFunction,
  b: { level: number; first: number; perLevel: number; available: number },
) {
  return formula(t, b.first, b.perLevel, b.level, b.available);
}
