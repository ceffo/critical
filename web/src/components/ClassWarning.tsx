import { useTranslation } from "react-i18next";
import { Warning } from "@phosphor-icons/react";
import { useCharacterStore } from "@/stores/character";
import type { ClassWarningCode } from "@/types/items";

/**
 * The words of a class warning (ADR 0033 step 20, ADR 0035 item 9): "Pas pour un barde". The server
 * sends the code; the class is the character's own.
 */
function useClassWarningText(): (code: ClassWarningCode) => string {
  const { t } = useTranslation();
  const cls = useCharacterStore((s) => s.character?.class ?? "");
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
