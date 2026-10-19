import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BookOpen } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { learnSpell } from "@/api/spells";
import { useCharacterStore } from "@/stores/character";
import { useLogStore } from "@/stores/log";
import { useSpellStore } from "@/stores/spells";
import { useTableStore } from "@/stores/table";
import type { LearnResult } from "@/types/character";

/**
 * Learning the selected wizard spell (ADR 0030 step 20) for a class with a spellbook: the
 * server rolls the chance to learn and answers the breakdown, or refuses (the table's error
 * notice words the code). The answer stays while the roll's journal entry is the top one: an
 * undo (or any later action) clears it. Shows "in the spellbook" once it is there. The rules are the server's:
 * this only offers the button and words the answer.
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
  // The answer and the journal entry it was logged as (the log's top once the roll is in).
  const [result, setResult] = useState<{
    res: LearnResult;
    entry: number | null;
  } | null>(null);

  useEffect(() => {
    if (rules) void fetchInfo();
  }, [rules, fetchInfo]);

  if (!rules || !wizard) return null;
  const known = book.some((e) => e.id === spellId);
  const shown =
    result?.res.spellId === spellId && result.entry === topId
      ? result.res
      : null;

  const learn = async () => {
    const got: { res: LearnResult | null } = { res: null };
    // The spellbook is read again by the key the answer names (a refusal reads everything).
    await run(async () => {
      got.res = await learnSpell(spellId);
    });
    // run has re-read the log: its top is this roll's entry.
    setResult(
      got.res ? { res: got.res, entry: useLogStore.getState().topId } : null,
    );
  };

  return (
    <div
      className="flex flex-wrap items-center gap-x-3 gap-y-1 pt-3"
      data-testid="learn-spell"
    >
      {known ? (
        <span className="inline-flex items-center gap-1 text-label text-magic">
          <BookOpen aria-hidden />
          {t("lookup.learn.known")}
        </span>
      ) : (
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
