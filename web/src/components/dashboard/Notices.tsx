import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { buffName, useSpellName } from "@/i18n/spellName";
import { useTableStore } from "@/stores/table";

/** Server errors (verbatim, role=alert) and round notices (role=status), bottom-left. */
export function Notices() {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const { error, notice, clearError, clearNotice } = useTableStore();

  useEffect(() => {
    if (!notice) return;
    const id = setTimeout(clearNotice, 8000);
    return () => clearTimeout(id);
  }, [notice, clearNotice]);

  return (
    <div className="pointer-events-none fixed bottom-3 left-3 z-50 flex w-[420px] flex-col gap-2">
      {error && (
        <div
          role="alert"
          className="pointer-events-auto flex items-start gap-2 rounded-lg border border-destructive/40 bg-card p-3 text-body shadow-lg"
        >
          <div className="min-w-0 flex-1">
            <div className="font-semibold text-destructive">
              {t("app.error")}
            </div>
            <div className="break-words">{error}</div>
          </div>
          <Button
            size="icon-xs"
            variant="ghost"
            aria-label={t("app.dismiss")}
            onClick={clearError}
          >
            <X />
          </Button>
        </div>
      )}
      {notice && (
        <div
          role="status"
          className="pointer-events-auto flex items-start gap-2 rounded-lg border bg-card p-3 text-body shadow-lg"
        >
          <div className="min-w-0 flex-1">
            {t("buffs.expired", {
              names: notice.buffs.map((b) => buffName(spellName, b)).join(", "),
            })}
          </div>
          <Button
            size="icon-xs"
            variant="ghost"
            aria-label={t("app.dismiss")}
            onClick={clearNotice}
          >
            <X />
          </Button>
        </div>
      )}
    </div>
  );
}
