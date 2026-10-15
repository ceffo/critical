import type { TFunction } from "i18next";

/** A split of extra slots as text: [1, 1, 1] → "3 × level 1", [1, 2] → "level 1 + level 2". */
export function splitText(t: TFunction, levels: number[]): string {
  const counts = new Map<number, number>();
  for (const l of levels) counts.set(l, (counts.get(l) ?? 0) + 1);
  return [...counts]
    .map(([level, count]) =>
      count > 1 ? t("spellSlots.splitCount", { count, level }) : t("spellSlots.splitLevel", { level }),
    )
    .join(" + ");
}
