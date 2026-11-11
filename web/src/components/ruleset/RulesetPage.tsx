import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft } from "@phosphor-icons/react";
import {
  fetchRuleset,
  saveCritTable,
  saveRacialLift,
  saveSuccessEffects,
  saveWeaponLift,
} from "@/api/ruleset";
import { Button } from "@/components/ui/button";
import { buttonVariants } from "@/components/ui/button";
import { PageHeader } from "@/components/shell/PageHeader";
import { APP_NAME } from "@/lib/brand";
import { useSessionStore } from "@/stores/session";
import type { Ruleset } from "@/types/ruleset";
import { CritTableCard } from "./CritTableCard";
import { HouseRulesCard } from "./HouseRulesCard";
import { SuccessEffectsCard } from "./SuccessEffectsCard";
import {
  critRowsOf,
  dirtyParts,
  draftOf,
  effectsOf,
  toCritDrafts,
  toEffectDrafts,
  type PartKey,
  type RulesetDraft,
} from "./rulesetDraft";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/**
 * The ruleset's screen (ADR 0041 step 8), a page of its own at /ruleset reached from the cog menu by
 * every user (a player reads it, an admin edits it): the crit table on the left, the house-rule switches and the success effects on the
 * right. Every card is edited in a draft and one Save sends what changed, each part through its own
 * PUT; a refused part keeps its edits and shows the refusal at its card, the parts already accepted
 * stay saved (an edit has no undo, ADR 0041 step 7). Discard returns the draft to the saved state. Back returns to the character sheet. The ruleset is the table's, not a character's, so
 * nothing here is on an action stack or in a journal.
 */
export function RulesetPage() {
  const { t } = useTranslation();
  const [ruleset, setRuleset] = useState<Ruleset | null>(null);
  const [error, setError] = useState<string | null>(null);
  const editable = useSessionStore((s) => s.me?.role === "admin");
  const [placeholder, setPlaceholder] = useState(false);
  // `saved` is the last state the server accepted; `draft` is what the inputs hold.
  const [saved, setSaved] = useState<RulesetDraft | null>(null);
  const [draft, setDraft] = useState<RulesetDraft | null>(null);
  const [errors, setErrors] = useState<Partial<Record<PartKey, string>>>({});
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);

  const dirty = saved && draft ? dirtyParts(draft, saved) : [];

  function edit(patch: Partial<RulesetDraft>) {
    setDone(false);
    setDraft((d) => (d ? { ...d, ...patch } : d));
  }

  function discard() {
    setDone(false);
    setErrors({});
    setDraft(saved);
  }

  async function save() {
    if (!saved || !draft) return;
    setBusy(true);
    setDone(false);
    const next: RulesetDraft = { ...saved };
    const after: RulesetDraft = { ...draft };
    const errs: Partial<Record<PartKey, string>> = {};
    for (const part of dirtyParts(draft, saved)) {
      try {
        if (part === "crit") {
          const table = await saveCritTable(critRowsOf(draft.crit));
          next.crit = after.crit = toCritDrafts(table.rows);
          setPlaceholder(table.placeholder);
        } else if (part === "racial") {
          next.racial = after.racial = await saveRacialLift(draft.racial);
        } else if (part === "weapon") {
          next.weapon = after.weapon = await saveWeaponLift(draft.weapon);
        } else {
          next.effects = after.effects = toEffectDrafts(
            await saveSuccessEffects(effectsOf(draft.effects)),
          );
        }
      } catch (e) {
        errs[part] = message(e);
      }
    }
    setSaved(next);
    setDraft(after);
    setErrors(errs);
    setDone(Object.keys(errs).length === 0);
    setBusy(false);
  }

  useEffect(() => {
    fetchRuleset()
      .then((r) => {
        setRuleset(r);
        setSaved(draftOf(r));
        setDraft(draftOf(r));
        setPlaceholder(r.critTable.placeholder);
      })
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
      <PageHeader
        title={t("ruleset.title")}
        action={
          <a
            href="/"
            className={buttonVariants({ variant: "quiet" })}
            aria-label={t("ruleset.backHint")}
            title={t("ruleset.backHint")}
          >
            <ArrowLeft aria-hidden />
            {t("ruleset.back")}
          </a>
        }
      />
      {error && (
        <p role="alert" className="zone-card px-4 py-3 text-body text-danger">
          {t("ruleset.loadFailed", { message: error })}
        </p>
      )}
      {ruleset && draft && (
        <>
          {editable && (
            <div
              className="zone-card flex flex-wrap items-center gap-2 px-4 py-2"
              data-testid="ruleset-save-bar"
            >
              <Button
                type="button"
                disabled={busy || dirty.length === 0}
                title={t("ruleset.saveHint")}
                aria-label={t("ruleset.saveHint")}
                onClick={() => void save()}
              >
                {t("ruleset.save")}
              </Button>
              <Button
                type="button"
                disabled={busy || dirty.length === 0}
                title={t("ruleset.discardHint")}
                aria-label={t("ruleset.discardHint")}
                onClick={discard}
              >
                {t("ruleset.discard")}
              </Button>
              {dirty.length > 0 && (
                <span className="text-body text-warning">
                  {t("ruleset.unsavedChanges")}
                </span>
              )}
              {done && (
                <span role="status" className="text-body text-muted-foreground">
                  {t("ruleset.saved")}
                </span>
              )}
            </div>
          )}
          <main className="grid min-w-0 grid-cols-[minmax(20rem,32rem)_minmax(0,1fr)] items-start gap-3">
            <CritTableCard
              rows={draft.crit}
              placeholder={placeholder}
              onChange={(crit) => edit({ crit })}
              error={errors.crit ?? ""}
              dirty={dirty.includes("crit")}
              editable={editable}
            />
            <div className="flex min-w-0 flex-col gap-3">
              <HouseRulesCard
                lift={draft.racial}
                weapon={draft.weapon}
                levelLimits={ruleset.levelLimits}
                onLift={(racial) => edit({ racial })}
                onWeapon={(weapon) => edit({ weapon })}
                error={errors.racial ?? errors.weapon ?? ""}
                dirty={dirty.includes("racial") || dirty.includes("weapon")}
                editable={editable}
              />
              <SuccessEffectsCard
                rows={draft.effects}
                onChange={(effects) => edit({ effects })}
                error={errors.effects ?? ""}
                dirty={dirty.includes("effects")}
                editable={editable}
              />
            </div>
          </main>
        </>
      )}
    </div>
  );
}
