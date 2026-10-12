import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Swords } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useRollStore } from "@/stores/roll";
import { captionClass, fieldClass } from "@/components/dashboard/fieldStyles";

/** Weapons the API resolves today (seeds/weapons.json); real weapons arrive with critical-qki.7. */
const WEAPON_IDS = ["longsword", "shortbow"] as const;

export function AttackPanel() {
  const { t } = useTranslation();
  const { rollAttack, rolling } = useRollStore();
  const [weaponId, setWeaponId] = useState<string>(WEAPON_IDS[0]);
  const [targetAc, setTargetAc] = useState("5");
  const ac = Number.parseInt(targetAc, 10);

  return (
    <form
      className="grid max-w-xl grid-cols-[minmax(0,1fr)_120px] items-end gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (!Number.isNaN(ac)) void rollAttack(weaponId, ac);
      }}
    >
      <label className="flex min-w-0 flex-col gap-1">
        <span className={captionClass}>{t("attack.weapon")}</span>
        <select
          className={fieldClass}
          value={weaponId}
          onChange={(e) => setWeaponId(e.target.value)}
        >
          {WEAPON_IDS.map((id) => (
            <option key={id} value={id}>
              {t(`attack.weapons.${id}`)}
            </option>
          ))}
        </select>
      </label>
      <label className="flex min-w-0 flex-col gap-1">
        <span className={captionClass}>{t("attack.targetAc")}</span>
        <input
          className={fieldClass}
          type="number"
          value={targetAc}
          onChange={(e) => setTargetAc(e.target.value)}
        />
      </label>
      <Button
        type="submit"
        size="lg"
        className="col-span-2 h-10 text-base"
        disabled={rolling || Number.isNaN(ac)}
      >
        <Swords />
        {rolling ? t("attack.rolling") : t("attack.roll")}
      </Button>
    </form>
  );
}
