import type { TFunction } from "i18next";
import type { XpLedger } from "@/types/xp";

/**
 * The XP bonus rule in words (ADR 0042 step 6): every prime requisite of every active class at the
 * server's minimum. The classes, their prime requisites and the minimum are the server's
 * (`bonusClasses`, `bonusMinScore`); this only words them. Null when no active class has a bonus
 * rule: the form then shows no sentence and no bonus checkbox.
 */
export function bonusHint(
  t: TFunction,
  lang: string,
  ledger: Pick<XpLedger, "bonusClasses" | "bonusMinScore">,
): string | null {
  const primes = [...new Set(ledger.bonusClasses.flatMap((c) => c.primes))];
  if (primes.length === 0) return null;
  const names = primes.map((a) =>
    t(`xp.bonusAbility.${a}`, { defaultValue: a }),
  );
  return t("xp.bonusRule", {
    count: primes.length,
    abilities: new Intl.ListFormat(lang, { type: "conjunction" }).format(names),
    min: ledger.bonusMinScore,
  });
}
