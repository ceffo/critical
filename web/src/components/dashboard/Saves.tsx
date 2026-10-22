import { Fragment, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, X } from "@phosphor-icons/react";
import { lastSaves } from "@/journal/hpAndSaves";
import { cn } from "@/lib/utils";
import { groupHeadClass } from "./fieldStyles";
import { SaveSpell } from "./SaveSpell";
import { useCharacterStore } from "@/stores/character";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { SAVE_CATEGORIES, type SaveCategory } from "@/types/combat";
import type { SaveNumbers } from "@/types/character";

const TARGET_FIELD: Record<SaveCategory, keyof SaveNumbers> = {
  para_poison: "paraPoison",
  rod_staff_wand: "rodStaffWand",
  petri_poly: "petriPoly",
  breath: "breath",
  spell: "spell",
};

/**
 * The five saving throws under their section label (ADR 0022 steps 2.3 and 4.2, ADR 0027 step 5.2:
 * "click to roll" is the rows' tooltip), always visible: one row each, name | target (the server's, derived from class and level, ADR 0030 step 6) |
 * last roll
 * (the newest logged save in that category, with the server's verdict). A click rolls it through
 * the save endpoint; the result card shows the breakdown. "Against the mind" flags the next save as
 * against magic that attacks the mind (ADR 0006 step 9.3): the server adds the Wisdom modifier. It
 * is the player's call for one save, so it clears once that save is rolled (not when
 * the request fails). Ticked, it shows a line to name the spell the save is against (ADR 0006 step
 * 10.2; every PHB Table 5 immunity is a spell that attacks the mind), cleared with it: against one
 * the character's Wisdom makes them immune to, the server rolls nothing and the last-roll column
 * shows the check with no die.
 */
export function Saves() {
  const { t } = useTranslation();
  const character = useCharacterStore((s) => s.character);
  const { rollSave, rolling } = useRollStore();
  const last = lastSaves(useLogStore((s) => s.entries));
  const [mindAffecting, setMindAffecting] = useState(false);
  const [spellId, setSpellId] = useState<string | undefined>();
  const setMind = (on: boolean) => {
    setMindAffecting(on);
    if (!on) setSpellId(undefined);
  };
  const rollOne = async (cat: SaveCategory) => {
    // Kept on a refused request, so the player doesn't have to set them again.
    if (await rollSave(cat, mindAffecting, mindAffecting ? spellId : undefined))
      setMind(false);
  };

  return (
    <section
      className="flex min-w-0 flex-col gap-2"
      aria-label={t("saves.title")}
    >
      {/* The mind flag sits on the label's line (as HpBar's last change), at the label's line height:
          a rare option, one click away, that costs the card no height (ADR 0027 step 1). */}
      <div
        className={cn(
          groupHeadClass,
          "flex min-w-0 flex-wrap items-center justify-between gap-x-2",
        )}
        data-testid="saves-head"
      >
        <h2 className="shrink-0 section-label">{t("saves.sectionLabel")}</h2>
        <label
          className="ml-auto flex items-center gap-1.5 text-label"
          title={t("saves.mindAffectingHint")}
        >
          <input
            type="checkbox"
            className="size-3.5"
            checked={mindAffecting}
            disabled={rolling}
            onChange={(e) => setMind(e.target.checked)}
            data-testid="save-mind-affecting"
          />
          {t("saves.mindAffecting")}
        </label>
      </div>
      {mindAffecting && (
        <div className="flex min-w-0 justify-end" data-testid="save-spell-line">
          <SaveSpell
            spellId={spellId}
            onChange={setSpellId}
            disabled={rolling}
          />
        </div>
      )}
      <ul className="flex flex-col gap-1">
        {SAVE_CATEGORIES.map((cat) => {
          const name = t(`saves.categories.${cat}`);
          const roll = last[cat];
          return (
            <li key={cat}>
              <button
                type="button"
                disabled={rolling}
                aria-label={t("saves.rollLabel", { name })}
                title={t("saves.rowHint", { name })}
                data-testid="save-row"
                data-save={cat}
                onClick={() => void rollOne(cat)}
                className="grid min-h-7 w-full grid-cols-[minmax(min-content,1fr)_auto_2.5rem] items-center gap-2 rounded-md bg-raised px-2.5 py-0.5 text-left text-body transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted"
              >
                <span>
                  {/* A slash is a place to wrap, so a long name breaks between its words. */}
                  {name.split("/").map((part, i) => (
                    <Fragment key={i}>
                      {i > 0 && (
                        <>
                          /<wbr />
                        </>
                      )}
                      {part}
                    </Fragment>
                  ))}
                </span>
                <span className="font-semibold tabular-nums">
                  {character
                    ? t("saves.target", {
                        value: character.derived.saves[TARGET_FIELD[cat]],
                      })
                    : ""}
                </span>
                <span
                  className={cn(
                    "flex items-center justify-end gap-1 tabular-nums",
                    roll && (roll.passed ? "text-success" : "text-danger"),
                  )}
                  data-testid="save-last"
                >
                  {roll?.immuneWisdom !== undefined ? (
                    // Immune: passed with no die (ADR 0006 step 10.2); the card and the journal
                    // say why.
                    <Check
                      aria-hidden
                      weight="bold"
                      data-testid="save-last-immune"
                    />
                  ) : roll ? (
                    <>
                      {roll.passed ? (
                        <Check aria-hidden weight="bold" />
                      ) : (
                        <X aria-hidden weight="bold" />
                      )}
                      {roll.d20}
                    </>
                  ) : (
                    <span className="text-muted-foreground">–</span>
                  )}
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
