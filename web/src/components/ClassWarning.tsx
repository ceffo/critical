import { useTranslation } from "react-i18next";
import { Warning } from "@phosphor-icons/react";
import { formatSigned } from "@/lib/format";
import { useCharacterStore } from "@/stores/character";
import type { ClassWarningCode } from "@/types/items";
import { primaryClass } from "@/lib/classEntries";

/**
 * The words of a class warning (ADR 0033 step 20, ADR 0035 item 9): "Pas pour un barde". The server
 * sends the code; the class is the character's own.
 */
function useClassWarningText(): (code: ClassWarningCode) => string {
  const { t } = useTranslation();
  const cls = useCharacterStore((s) => primaryClass(s.character?.classes));
  return (code) =>
    t(`equipment.warnings.${code}`, {
      who: t(`equipment.warnings.who.${cls}`),
    });
}

/** A warning in amber with its icon, wrapping its words (never cut, ADR 0022 step 1). */
export function ClassWarningTag({
  code,
  className,
}: {
  code: ClassWarningCode;
  className?: string;
}) {
  const text = useClassWarningText()(code);
  return (
    <span
      className={`flex items-center gap-1 text-label text-warning ${className ?? ""}`}
      data-testid="class-warning"
      data-code={code}
    >
      <Warning aria-hidden className="size-3.5 shrink-0" />
      <span className="min-w-0 break-words">{text}</span>
    </span>
  );
}

/**
 * A weapon without proficiency (ADR 0037 step 8): "Non maîtrisée : −2 au toucher", the server's
 * penalty for the piece (`nonproficientPenalty`), so the penalty is no surprise at the roll.
 * Nothing for a penalty of 0.
 */
export function NonproficientTag({ penalty }: { penalty: number }) {
  const { t } = useTranslation();
  if (penalty >= 0) return null;
  const value = formatSigned(penalty);
  return (
    <span
      className="flex items-center gap-1 text-label text-warning"
      title={t("weaponProficiencies.nonproficientHint", { value })}
      data-testid="nonproficient"
    >
      <Warning aria-hidden className="size-3.5 shrink-0" />
      <span className="min-w-0 break-words">
        {t("weaponProficiencies.nonproficient", { value })}
      </span>
    </span>
  );
}
