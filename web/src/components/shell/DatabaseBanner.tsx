import { useTranslation } from "react-i18next";
import { CircleNotch } from "@phosphor-icons/react";
import { useDatabaseStore } from "@/stores/database";

/**
 * The one notice of an unavailable database (ADR 0025 step 10.4), top centre, over the sheet:
 * reads are retried meanwhile, writes are refused. After a write whose outcome is unknown it also
 * says that writes wait for the sheet to be reloaded, and to check the journal before retrying.
 */
export function DatabaseBanner() {
  const { t } = useTranslation();
  const { unavailable, uncertainWrite } = useDatabaseStore();
  if (!unavailable) return null;
  return (
    <div className="pointer-events-none fixed inset-x-0 top-3 z-50 flex justify-center px-3">
      <div
        role="status"
        data-testid="database-banner"
        className="pointer-events-auto flex max-w-[640px] items-start gap-2 rounded-lg border border-destructive/40 bg-card p-3 text-body shadow-lg"
      >
        <CircleNotch
          aria-hidden
          className="mt-0.5 size-4 shrink-0 animate-spin text-destructive"
        />
        <div className="min-w-0 flex-1 break-words">
          <div className="font-semibold">{t("database.unavailable")}</div>
          {uncertainWrite && <div>{t("database.uncertainWrite")}</div>}
        </div>
      </div>
    </div>
  );
}
