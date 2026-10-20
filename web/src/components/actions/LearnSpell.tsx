import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BookmarkSimple, BookOpen } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { addSpell, learnSpell } from "@/api/spells";
import { useCharacterStore } from "@/stores/character";
import { useLogStore } from "@/stores/log";
import { useLookupStore } from "@/stores/lookup";
import { useSpellStore } from "@/stores/spells";
import { useTableStore } from "@/stores/table";
import type { LearnResult } from "@/types/character";
import { spellbookOffer } from "./spellbookOffer";

/**
 * Learning the selected wizard spell (ADR 0030 step 20) for a class with a spellbook: the
 * server rolls the chance to learn and answers the breakdown, or refuses (the table's error
 * notice words the code). The answer stays while the roll's journal entry is the top one: an
 * undo (or any later action) clears it. Shows "in the spellbook" once it is there. Add (Ajouter,
 * step 20.6) writes the spell with no roll; the spellbook is read again by the answer's state key.
 * Opened from a free spell's link (ADR 0030 step 20.8), while the server still lists that free spell
 * as owed, Ajouter claims it (the add's `grantLevel`); the server refuses a spell of another level.
 * The rules are the server's: this only offers the buttons and words the answer.
 */
export function LearnSpell({
  spellId,
  wizard,
}: {
  spellId: string;
  wizard: boolean;
}) {
  const { t } = useTranslation();
  const rules = useCharacterStore(
    (s) => s.character?.derived.spellbook ?? null,
  );
  const book = useSpellStore((s) => s.info.spellbook);
  const fetchInfo = useSpellStore((s) => s.fetchInfo);
  const run = useTableStore((s) => s.run);
  const busy = useTableStore((s) => s.busy);
  const topId = useLogStore((s) => s.topId);
  const owed = useCharacterStore(
    (s) => s.character?.derived.levelUp.toSpend.freeSpells,
  );
  const grant = useLookupStore((s) => s.grant);
  const clearGrant = useLookupStore((s) => s.clearGrant);
  // The free spell being claimed, while the server still owes it.
  const claim =
    grant &&
    owed?.some(
      (f) => f.level === grant.level && f.spellLevel === grant.spellLevel,
    )
      ? grant
      : null;
  // The answer and the journal entry it was logged as (the log's top once the roll is in).
  const [result, setResult] = useState<{
    res: LearnResult;
    entry: number | null;
  } | null>(null);

  useEffect(() => {
    if (rules) void fetchInfo();
  }, [rules, fetchInfo]);

  const known = book.some((e) => e.id === spellId);
  const offer = spellbookOffer(rules, wizard, known);
  if (offer === "none" || !rules) return null;
  const shown =
    result?.res.spellId === spellId && result.entry === topId
      ? result.res
      : null;

  const learn = async () => {
    const got: { res: LearnResult | null } = { res: null };
    // The spellbook is read again by the key the answer names (a conflict reads everything).
    await run(async () => {
      got.res = await learnSpell(spellId);
    });
    // run has re-read the log: its top is this roll's entry.
    setResult(
      got.res ? { res: got.res, entry: useLogStore.getState().topId } : null,
    );
  };

  const add = async () => {
    setResult(null);
    const ok = await run(async () => {
      await addSpell(spellId, claim?.level);
    });
    if (ok && claim) clearGrant();
  };
  const addTitle = claim
    ? t("lookup.learn.addFreeTitle", { level: claim.level })
    : t("lookup.learn.addTitle");

  return (
    <div
      className="flex flex-wrap items-center gap-x-3 gap-y-1 pt-3"
      data-testid="learn-spell"
    >
      {offer === "known" ? (
        <span className="inline-flex items-center gap-1 text-label text-magic">
          <BookOpen aria-hidden />
          {t("lookup.learn.known")}
        </span>
      ) : (
        <>
          <Button
            size="sm"
            disabled={busy}
            onClick={() => void learn()}
            title={t("lookup.learn.title", { chance: rules.learnChance })}
            aria-label={t("lookup.learn.title", { chance: rules.learnChance })}
          >
            <BookOpen aria-hidden />
            {t("lookup.learn.button")}
          </Button>
          <Button
            size="sm"
            disabled={busy}
            onClick={() => void add()}
            title={addTitle}
            aria-label={addTitle}
            data-testid="add-spell"
          >
            <BookmarkSimple aria-hidden />
            {t("lookup.learn.add")}
          </Button>
          {claim && (
            <span
              className="text-label text-magic"
              data-testid="free-spell-claim"
            >
              {t("lookup.learn.free", { level: claim.level })}
            </span>
          )}
        </>
      )}
      {shown && (
        <span
          className="text-label text-muted-foreground"
          data-testid="learn-result"
        >
          {shown.learned
            ? t("lookup.learn.learned", {
                roll: shown.roll,
                chance: shown.chance,
              })
            : t("lookup.learn.failed", {
                roll: shown.roll,
                chance: shown.chance,
                level: shown.retryLevel,
              })}
        </span>
      )}
    </div>
  );
}
