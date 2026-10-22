import type { TFunction } from "i18next";
import type { ClassAmount } from "@/types/xp";

/**
 * The words of an award's split (ADR 0042 step 5.1): "Guerrier 500 · Clerc 500". The shares are
 * the server's (`split`); this only words them. Empty for one class (no split).
 */
export function splitParts(
  t: TFunction,
  split: ClassAmount[] | undefined,
): string {
  return (split ?? [])
    .map((s) =>
      t("xp.split.part", {
        class: t(`details.classes.${s.class}`, { defaultValue: s.class }),
        amount: s.amount,
      }),
    )
    .join(" · ");
}
