import { useState } from "react";
import { useTranslation } from "react-i18next";
import { saveRacialLift, saveWeaponLift } from "@/api/ruleset";
import type { RacialLift, Ruleset, WeaponLift } from "@/types/ruleset";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/**
 * The house rules that are switches (ADR 0041 step 2): the racial lift for multi-class and
 * dual-class characters, saved as each box is ticked, and the level limits mode, shown read-only
 * while it has one value (step 8), and the weapon lift for a multi-class character (step 2.5). An
 * admin ticks the boxes; anyone else sees the state of each in words (`editable` false). What a lift
 * changes for a character is the server's; this only stores the switches.
 */
export function HouseRulesCard({
  initial,
  weaponLift,
  levelLimits,
  editable = true,
}: {
  initial: RacialLift;
  weaponLift: WeaponLift;
  levelLimits: Ruleset["levelLimits"];
  editable?: boolean;
}) {
  const { t } = useTranslation();
  const [lift, setLift] = useState<RacialLift>(initial);
  const [weapon, setWeapon] = useState<WeaponLift>(weaponLift);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);

  async function toggle(field: keyof RacialLift, value: boolean) {
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      setLift(await saveRacialLift({ ...lift, [field]: value }));
      setSaved(true);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }

  async function toggleWeapon(value: boolean) {
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      setWeapon(await saveWeaponLift({ multiclassAnyWeapon: value }));
      setSaved(true);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section
      aria-labelledby="ruleset-rules"
      className="zone-card flex min-w-0 flex-col"
      data-testid="ruleset-house-rules"
    >
      <div className="card-band flex items-center px-4 py-2">
        <h2 id="ruleset-rules" className="section-label">
          {t("ruleset.houseRules.title")}
        </h2>
      </div>
      <div className="flex flex-col gap-3 px-4 pt-3 pb-3">
        <p className="text-body text-muted-foreground">
          {t("ruleset.houseRules.liftHelp")}
        </p>
        {(["multiclassAnyRace", "dualclassAnyRace"] as const).map((f) => (
          <label key={f} className="flex w-fit items-center gap-2 text-body">
            <input
              type="checkbox"
              className="size-4"
              checked={lift[f]}
              disabled={busy || !editable}
              onChange={(e) => void toggle(f, e.target.checked)}
            />
            {t(`ruleset.houseRules.${f}`)}
          </label>
        ))}
        <p className="text-body text-muted-foreground">
          {t("ruleset.houseRules.weaponHelp")}
        </p>
        <label className="flex w-fit items-center gap-2 text-body">
          <input
            type="checkbox"
            className="size-4"
            data-testid="ruleset-weapon-lift"
            checked={weapon.multiclassAnyWeapon}
            disabled={busy || !editable}
            onChange={(e) => void toggleWeapon(e.target.checked)}
          />
          {t("ruleset.houseRules.multiclassAnyWeapon")}
        </label>
        {error && (
          <p role="alert" className="text-body text-danger">
            {error}
          </p>
        )}
        {saved && (
          <p role="status" className="text-body text-muted-foreground">
            {t("ruleset.houseRules.saved")}
          </p>
        )}
        <div className="flex flex-col gap-1">
          <span className="text-label font-medium text-muted-foreground">
            {t("ruleset.houseRules.levelLimits")}
          </span>
          <p className="text-body" data-testid="ruleset-level-limits">
            {t(`ruleset.houseRules.levelLimitsMode.${levelLimits}`)}
          </p>
        </div>
      </div>
    </section>
  );
}
