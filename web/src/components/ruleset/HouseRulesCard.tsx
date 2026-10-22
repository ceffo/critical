import { useState } from "react";
import { useTranslation } from "react-i18next";
import { saveRacialLift } from "@/api/ruleset";
import type { RacialLift, Ruleset } from "@/types/ruleset";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/**
 * The house rules that are switches (ADR 0041 step 2): the racial lift for multi-class and
 * dual-class characters, saved as each box is ticked, and the level limits mode, shown read-only
 * while it has one value (step 8). What the lift changes for a character is for a later item; this
 * only stores the switches.
 */
export function HouseRulesCard({
  initial,
  levelLimits,
}: {
  initial: RacialLift;
  levelLimits: Ruleset["levelLimits"];
}) {
  const { t } = useTranslation();
  const [lift, setLift] = useState<RacialLift>(initial);
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
              disabled={busy}
              onChange={(e) => void toggle(f, e.target.checked)}
            />
            {t(`ruleset.houseRules.${f}`)}
          </label>
        ))}
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
