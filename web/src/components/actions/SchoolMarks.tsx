import { useTranslation } from "react-i18next";

/**
 * A specialist's school slot beside the level's allotment, "2 + 1" (ADR 0050 step 10): both figures
 * are the server's (`max`, `school`); nothing is added here.
 */
export function SchoolSlot({
  max,
  school,
  testId,
}: {
  max: number;
  school: number;
  testId: string;
}) {
  const { t } = useTranslation();
  const hint = t("spellSlots.schoolSlotHint", { max, school });
  return (
    <span
      className="text-label text-muted-foreground tabular-nums"
      title={hint}
      aria-label={hint}
      data-testid={testId}
    >
      {t("spellSlots.schoolSlot", { max, school })}
    </span>
  );
}

/** The mark of a spell of a school the specialist opposes (the server's `opposed`, ADR 0050 step 15.3). */
export function OpposedMark({ id }: { id: string }) {
  const { t } = useTranslation();
  return (
    <span
      className="shrink-0 text-label text-warning"
      title={t("spellSlots.opposedHint")}
      data-testid={`opposed-${id}`}
    >
      {t("spellSlots.opposed")}
    </span>
  );
}
