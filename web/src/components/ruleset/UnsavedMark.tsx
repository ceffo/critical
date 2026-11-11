import { useTranslation } from "react-i18next";

/** The tag on a card with edits the page's Save has not sent yet. */
export function UnsavedMark({ show }: { show: boolean }) {
  const { t } = useTranslation();
  if (!show) return null;
  return (
    <span
      data-testid="unsaved-mark"
      className="rounded-full border border-warning px-2 text-label text-warning"
    >
      {t("ruleset.unsaved")}
    </span>
  );
}
