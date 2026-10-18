import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { changeSkillArmor, fetchCreationOptions } from "@/api/characters";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { useCharacterStore } from "@/stores/character";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";

/**
 * The armour a thief wears for its skills (ADR 0030 step 1), changeable after creation since armour
 * changes in play. The choices come from the server (the creation options); a pick saves at once
 * and the skills are read again, the server deriving each percentage from the new armour.
 */
export function ThiefArmorPicker() {
  const { t } = useTranslation();
  const id = useCharacterStore((s) => s.character?.id);
  const armor = useCharacterStore((s) => s.character?.skillArmor);
  const fetchSkills = useSkillsStore((s) => s.fetchSkills);
  const run = useTableStore((s) => s.run);
  const [armors, setArmors] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  useEffect(() => {
    let live = true;
    fetchCreationOptions()
      .then((o) => {
        if (live)
          setArmors(
            o.classes.find((c) => c.skillArmors.length > 0)?.skillArmors ?? [],
          );
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, []);
  if (id === undefined || !armor || armors.length === 0) return null;

  const pick = async (next: string) => {
    setSaving(true);
    try {
      await run(() => changeSkillArmor(id, next), { character: true });
      await fetchSkills();
    } finally {
      setSaving(false);
    }
  };

  return (
    <div
      className="flex min-w-0 flex-col gap-1 px-2.5"
      data-testid="thief-armor-picker"
    >
      <label htmlFor="thief-armor" className={labelClass}>
        {t("characters.skillArmor")}
      </label>
      <select
        id="thief-armor"
        className={fieldClass}
        value={armor}
        disabled={saving}
        title={t("skills.armorHint")}
        data-testid="thief-armor"
        onChange={(e) => void pick(e.target.value)}
      >
        {armors.map((a) => (
          <option key={a} value={a}>
            {t(`characters.armors.${a}`, { defaultValue: a })}
          </option>
        ))}
      </select>
    </div>
  );
}
