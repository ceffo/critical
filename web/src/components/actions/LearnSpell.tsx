import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { BookOpen } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { learnSpell } from "@/api/spells";
import { useCharacterStore } from "@/stores/character";
import { useSpellStore } from "@/stores/spells";
import { useTableStore } from "@/stores/table";
import type { LearnResult } from "@/types/character";

/**
 * Learning the selected wizard spell (ADR 0030 step 20) for a class with a spellbook: the
 * server rolls the chance to learn and answers the breakdown, or refuses (the table's error
 * notice words the code). Shows "in the spellbook" once it is there. The rules are the server's:
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
  const [result, setResult] = useState<LearnResult | null>(null);

  useEffect(() => {
    if (rules) void fetchInfo();
  }, [rules, fetchInfo]);

  if (!rules || !wizard) return null;
  const known = book.some((e) => e.id === spellId);
  const shown = result?.spellId === spellId ? result : null;

  const learn = () =>
    run(async () => {
      try {
        setResult(await learnSpell(spellId));
      } finally {
        await fetchInfo();
      }
    });

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
                level: shown.characterLevel + 1,
              })}
        </span>
      )}
    </div>
  );
}
