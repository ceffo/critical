import { useTranslation } from "react-i18next";

/**
 * The mark of a field the open form still needs: an asterisk (a shape, not a colour) with its
 * spoken word. Shown only while the field is missing, from the same condition that disables the action.
 */
export function Required() {
  const { t } = useTranslation();
  return (
    <span className="relative text-danger" title={t("powers.tab.bar.required")}>
      <span aria-hidden>*</span>
      <span className="sr-only">{t("powers.tab.bar.required")}</span>
    </span>
  );
}

/** The reason beside a disabled action: what the form still needs, named. */
export function Needs({
  needs,
  testId,
}: {
  needs: ("target" | "mind" | "level" | "rolls")[];
  testId: string;
}) {
  const { t } = useTranslation();
  if (needs.length === 0) return null;
  return (
    <span className="text-label text-ink-muted" data-testid={testId}>
      {t("powers.tab.bar.needs", {
        fields: needs.map((n) => t(`powers.tab.bar.need.${n}`)).join(", "),
      })}
    </span>
  );
}
