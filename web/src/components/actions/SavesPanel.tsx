import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useCharacterStore } from "@/stores/character";
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

export function SavesPanel() {
  const { t } = useTranslation();
  const character = useCharacterStore((s) => s.character);
  const { rollSave, rolling } = useRollStore();

  return (
    <ul className="max-w-xl divide-y rounded-lg border">
      {SAVE_CATEGORIES.map((cat) => {
        const name = t(`saves.categories.${cat}`);
        return (
          <li key={cat} className="flex items-center justify-between gap-3 px-3 py-2">
            <div className="min-w-0">
              <div className="font-medium">{name}</div>
              {character && (
                <div className="text-xs text-muted-foreground">
                  {t("saves.need", { value: character[TARGET_FIELD[cat]] })}
                </div>
              )}
            </div>
            <Button
              size="sm"
              variant="secondary"
              disabled={rolling}
              aria-label={t("saves.rollLabel", { name })}
              onClick={() => void rollSave(cat)}
            >
              {t("saves.roll")}
            </Button>
          </li>
        );
      })}
    </ul>
  );
}
