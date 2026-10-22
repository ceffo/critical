import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft } from "@phosphor-icons/react";
import { fetchRuleset } from "@/api/ruleset";
import { buttonVariants } from "@/components/ui/button";
import { VersionBadge } from "@/components/shell/VersionBadge";
import { APP_NAME } from "@/lib/brand";
import type { Ruleset } from "@/types/ruleset";
import { CritTableCard } from "./CritTableCard";
import { HouseRulesCard } from "./HouseRulesCard";
import { SuccessEffectsCard } from "./SuccessEffectsCard";

/**
 * The ruleset's screen (ADR 0041 step 8), a page of its own at /ruleset reached from the cog menu by
 * admins only: the crit table on the left, the house-rule switches and the success effects on the
 * right. Back returns to the character sheet. The ruleset is the table's, not a character's, so
 * nothing here is on an action stack or in a journal.
 */
export function RulesetPage() {
  const { t } = useTranslation();
  const [ruleset, setRuleset] = useState<Ruleset | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchRuleset()
      .then(setRuleset)
      .catch((e: unknown) =>
        setError(e instanceof Error ? e.message : String(e)),
      );
  }, []);
  useEffect(() => {
    document.title = `${t("ruleset.title")} · ${APP_NAME}`;
  }, [t]);

  return (
    <div
      className="mx-auto flex min-h-dvh w-full max-w-[120rem] flex-col gap-3 p-3"
      data-screen="ruleset"
    >
      <header className="zone-card flex items-center gap-3 px-4 py-2">
        <h1 className="section-label min-w-0 flex-1">{t("ruleset.title")}</h1>
        <VersionBadge />
        <a
          href="/"
          className={buttonVariants({ variant: "quiet" })}
          aria-label={t("ruleset.backHint")}
          title={t("ruleset.backHint")}
        >
          <ArrowLeft aria-hidden />
          {t("ruleset.back")}
        </a>
      </header>
      {error && (
        <p role="alert" className="zone-card px-4 py-3 text-body text-danger">
          {t("ruleset.loadFailed", { message: error })}
        </p>
      )}
      {ruleset && (
        <main className="grid min-w-0 grid-cols-[minmax(20rem,32rem)_minmax(0,1fr)] items-start gap-3">
          <CritTableCard initial={ruleset.critTable} />
          <div className="flex min-w-0 flex-col gap-3">
            <HouseRulesCard
              initial={ruleset.racialLift}
              levelLimits={ruleset.levelLimits}
            />
            <SuccessEffectsCard initial={ruleset.successEffects} />
          </div>
        </main>
      )}
    </div>
  );
}
