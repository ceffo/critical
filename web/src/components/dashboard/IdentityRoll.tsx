import { DiceFive } from "@phosphor-icons/react";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { rollIdentity } from "@/api/characters";
import { ApiError } from "@/api/http";
import { Button } from "@/components/ui/button";
import type { IdentityFigure, IdentityRollFigure } from "@/types/character";

/**
 * The roll button of a height, a weight or an age (ADR 0032 step 5.3): the server rolls the table's
 * dice for the race (and the sex, for the first two) and answers the breakdown, which is shown under
 * the field; the value fills the field and the player may still change it. Writes nothing.
 */
export function IdentityRoll({
  figure,
  raceCode,
  sex,
  onRolled,
}: {
  figure: IdentityFigure;
  raceCode: string;
  sex: string;
  /** The value to store (centimetres, kilograms or years) of the roll. */
  onRolled: (value: number) => void;
}) {
  const { t } = useTranslation();
  const [roll, setRoll] = useState<IdentityRollFigure | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // The race and sex the figure is rolled for: a reply for others (changed while it was pending)
  // is dropped. Anything else typed meanwhile stays: the callback merges into the latest details.
  const latest = useRef({ raceCode, sex });
  useEffect(() => {
    latest.current = { raceCode, sex };
  }, [raceCode, sex]);
  const needsSex = figure !== "age";
  const missing =
    raceCode === "" ? "race" : needsSex && sex === "" ? "sex" : null;

  const doRoll = async () => {
    if (missing || busy) return;
    setBusy(true);
    setError(null);
    try {
      const r = await rollIdentity(raceCode, sex, [figure]);
      const now = latest.current;
      if (now.raceCode !== raceCode || (needsSex && now.sex !== sex)) return;
      const rolled = r.rolls[0];
      setRoll(rolled);
      onRolled(rolled.value);
    } catch (e) {
      setError(
        e instanceof ApiError && e.field
          ? t(`details.roll.errors.${e.message}`, { defaultValue: e.message })
          : e instanceof Error
            ? e.message
            : String(e),
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <Button
        type="button"
        variant="quiet"
        size="sm"
        className="self-start"
        disabled={missing !== null || busy}
        onClick={() => void doRoll()}
        title={t(
          missing
            ? `details.roll.need.${missing}`
            : `details.roll.hint.${figure}`,
        )}
        data-testid={`details-roll-${figure}`}
      >
        <DiceFive aria-hidden />
        {roll ? t("creation.reroll") : t("creation.scores.roll")}
      </Button>
      {error && (
        <span role="alert" className="text-label text-danger">
          {error}
        </span>
      )}
      {roll && (
        <span
          className="text-label text-muted-foreground"
          data-testid={`details-roll-${figure}-breakdown`}
        >
          {t("details.roll.breakdown", {
            count: roll.dice.length,
            sides: roll.sides,
            dice: roll.dice.join(" + "),
            base: roll.base,
            total: roll.total,
            unit: t(`details.roll.units.${roll.unit}`),
          })}
        </span>
      )}
    </div>
  );
}
