import { useTranslation } from "react-i18next";
import { Check, X } from "@phosphor-icons/react";
import { lastSaves } from "@/lib/moiLog";
import { cn } from "@/lib/utils";
import { useCharacterStore } from "@/stores/character";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { SAVE_CATEGORIES, type SaveCategory } from "@/types/combat";
import type { Character } from "@/types/character";

const TARGET_FIELD: Record<SaveCategory, keyof Character> = {
  para_poison: "saveParaPoison",
  rod_staff_wand: "saveRodStaffWand",
  petri_poly: "savePetriPoly",
  breath: "saveBreath",
  spell: "saveSpell",
};

/**
 * The five saving throws, always visible: one row each, name | target (the sheet's) | last roll
 * (the newest logged save in that category, with the server's verdict). A click rolls it through
 * the save endpoint; the result card shows the breakdown.
 */
export function Saves() {
  const { t } = useTranslation();
  const character = useCharacterStore((s) => s.character);
  const { rollSave, rolling } = useRollStore();
  const last = lastSaves(useLogStore((s) => s.entries));

  return (
    <section
      className="flex min-w-0 flex-col gap-1"
      aria-labelledby="moi-saves-title"
    >
      <h2 id="moi-saves-title" className="text-title">
        {t("saves.title")}
      </h2>
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
                title={t("saves.rollLabel", { name })}
                data-testid="save-row"
                data-save={cat}
                onClick={() => void rollSave(cat)}
                className="grid h-7 w-full grid-cols-[minmax(0,1fr)_auto_3.25rem] items-center gap-2 rounded-md bg-neu-900/60 px-2 text-left text-body transition-colors outline-none hover:bg-neu-800 focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50"
              >
                <span className="truncate">{name}</span>
                <span className="text-muted-foreground tabular-nums">
                  {character
                    ? t("saves.target", { value: character[TARGET_FIELD[cat]] })
                    : ""}
                </span>
                <span
                  className={cn(
                    "flex items-center justify-end gap-1 tabular-nums",
                    roll && (roll.passed ? "text-success" : "text-danger"),
                  )}
                  data-testid="save-last"
                >
                  {roll ? (
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
