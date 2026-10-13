import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Swords } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useRollStore } from "@/stores/roll";
import { useItemsStore } from "@/stores/items";
import { useTableStore } from "@/stores/table";
import { captionClass, fieldClass } from "@/components/dashboard/fieldStyles";
import { CritTableEditor } from "@/components/actions/CritTableEditor";
import type { AttackRequest, TargetSize } from "@/types/combat";

const SIZES: TargetSize[] = ["sm", "l"];

/**
 * Attack tab: pick a weapon (and the ammunition a bow fires), the target's AC and size, and an
 * optional situational modifier. The API resolves everything; the result card shows the breakdown.
 */
export function AttackPanel() {
  const { t } = useTranslation();
  const { rollAttack, rolling } = useRollStore();
  const { weapons, items, fetchWeapons } = useItemsStore();
  const [weaponId, setWeaponId] = useState("");
  const [ammoId, setAmmoId] = useState("");
  const [targetAc, setTargetAc] = useState("5");
  const [size, setSize] = useState<TargetSize>("sm");
  const [sitValue, setSitValue] = useState("0");
  const [sitLabel, setSitLabel] = useState("");

  useEffect(() => {
    fetchWeapons().catch((e: unknown) =>
      useTableStore.setState({
        error: e instanceof Error ? e.message : String(e),
      }),
    );
  }, [fetchWeapons]);

  const weapon = weapons.find((w) => w.id === weaponId) ?? weapons[0];
  // Ammo the weapon can fire. Before the player picks, the first with anything left is
  // preselected; once picked (or fired), the choice is never switched behind their back: when it
  // runs out the panel says so and waits for an explicit pick.
  const ammoChoices = weapon?.ammo_kind
    ? items.filter((i) => i.kind === "ammo" && i.ammo_kind === weapon.ammo_kind)
    : [];
  const ammo =
    ammoChoices.find((i) => i.id === ammoId) ??
    (ammoId === "" ? ammoChoices.find((i) => i.count > 0) : undefined);
  const ammoSpent = ammo !== undefined && ammo.count === 0;
  const needsAmmo = Boolean(weapon?.ammo_kind);

  const ac = Number.parseInt(targetAc, 10);
  const sit = sitValue.trim() === "" ? 0 : Number.parseInt(sitValue, 10);
  const valid =
    weapon !== undefined &&
    !Number.isNaN(ac) &&
    !Number.isNaN(sit) &&
    (!needsAmmo || (ammo && !ammoSpent));

  function submit() {
    if (!valid || !weapon) return;
    const req: AttackRequest = {
      weapon_id: weapon.id,
      target_ac: ac,
      target_size: size,
    };
    if (needsAmmo && ammo) {
      req.ammo_item_id = ammo.id;
      setAmmoId(ammo.id); // a fired default becomes the player's choice
    }
    if (sit !== 0) req.situational = { value: sit, label: sitLabel.trim() };
    void rollAttack(req);
  }

  if (weapons.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">{t("attack.loading")}</p>
    );
  }

  return (
    <>
      <form
        className="grid max-w-xl grid-cols-[minmax(0,1fr)_minmax(0,1fr)] items-end gap-3"
        aria-label={t("actions.tabs.attack")}
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <label className="col-span-2 flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("attack.weapon")}</span>
          <select
            className={fieldClass}
            value={weapon?.id}
            onChange={(e) => setWeaponId(e.target.value)}
          >
            {weapons.map((w) => (
              <option key={w.id} value={w.id}>
                {t("attack.weaponOption", {
                  name: w.name,
                  sm: w.damage_sm,
                  l: w.damage_l,
                })}
              </option>
            ))}
          </select>
        </label>

        {needsAmmo && (
          <label className="col-span-2 flex min-w-0 flex-col gap-1">
            <span className={captionClass}>{t("attack.ammo")}</span>
            {ammoChoices.length === 0 ? (
              <span className="text-sm text-destructive">
                {t("attack.noAmmo")}
              </span>
            ) : (
              <select
                className={fieldClass}
                value={ammo?.id ?? ""}
                aria-invalid={ammoSpent || ammo === undefined}
                onChange={(e) => setAmmoId(e.target.value)}
              >
                {ammo === undefined && (
                  <option value="" disabled>
                    {t("attack.pickAmmo")}
                  </option>
                )}
                {ammoChoices.map((i) => (
                  <option
                    key={i.id}
                    value={i.id}
                    disabled={i.count === 0 && i.id !== ammo?.id}
                  >
                    {t("attack.ammoOption", { name: i.name, count: i.count })}
                  </option>
                ))}
              </select>
            )}
            {ammoSpent && (
              <span
                className="text-sm text-destructive"
                data-testid="ammo-spent"
              >
                {t("attack.ammoSpent", { name: ammo.name })}
              </span>
            )}
          </label>
        )}

        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("attack.targetAc")}</span>
          <input
            className={fieldClass}
            type="number"
            value={targetAc}
            onChange={(e) => setTargetAc(e.target.value)}
          />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("attack.targetSize")}</span>
          <select
            className={fieldClass}
            value={size}
            onChange={(e) => setSize(e.target.value as TargetSize)}
          >
            {SIZES.map((s) => (
              <option key={s} value={s}>
                {t(`attack.sizes.${s}`)}
              </option>
            ))}
          </select>
        </label>

        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("attack.situational")}</span>
          <input
            className={fieldClass}
            type="number"
            value={sitValue}
            onChange={(e) => setSitValue(e.target.value)}
          />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <span className={captionClass}>{t("attack.situationalLabel")}</span>
          <input
            className={fieldClass}
            type="text"
            placeholder={t("attack.situationalPlaceholder")}
            value={sitLabel}
            onChange={(e) => setSitLabel(e.target.value)}
          />
        </label>

        <Button
          type="submit"
          size="lg"
          className="col-span-2 h-10 text-base"
          disabled={rolling || !valid}
        >
          <Swords />
          {rolling ? t("attack.rolling") : t("attack.roll")}
        </Button>
      </form>
      <CritTableEditor />
    </>
  );
}
