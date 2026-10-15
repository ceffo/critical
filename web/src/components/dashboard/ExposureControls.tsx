// Rolling one round of exposure to a spell's area (ADR 0008 step 16). The player picks the mode,
// the round of exposure and whether the Bard is the one exposed; the server rolls the dice it
// serves for that round, logs the roll and, on the Bard, takes the damage off his HP.
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useRollStore } from "@/stores/roll";
import { exposureText, modeName } from "@/lib/spellMode";
import type { SpellExposure } from "@/types/spells";
import { fieldClass, labelClass } from "./fieldStyles";
import { cn } from "@/lib/utils";

/**
 * The exposure block of a card: the dice per round of exposure and the roll controls, starting
 * from `initial` (an exposure card offers the next round, same mode and target).
 */
export function ExposureControls({
  spellId,
  casterLevel,
  exposure,
  initial,
}: {
  spellId: string;
  casterLevel: number;
  exposure: SpellExposure;
  initial?: { mode?: string; round?: number; onSelf?: boolean };
}) {
  const { t } = useTranslation();
  const rolling = useRollStore((s) => s.rolling);
  const rollExposure = useRollStore((s) => s.rollExposure);
  const modes = exposure.modes ?? [];
  const [mode, setMode] = useState(initial?.mode ?? modes[0]?.key ?? "");
  const [round, setRound] = useState(String(initial?.round ?? 1));
  const [onSelf, setOnSelf] = useState(initial?.onSelf ?? false);
  const roundNumber = Number.parseInt(round, 10);

  return (
    <div
      className="flex flex-col gap-2 rounded-lg border p-3"
      data-testid="cast-exposure"
    >
      <div className={labelClass}>{t("exposure.title")}</div>
      <p className="text-body" data-testid="exposure-dice">
        {exposureText(t, spellId, exposure)}
      </p>
      <div className="flex flex-wrap items-end gap-2">
        {modes.length > 0 && (
          <label className="flex min-w-0 flex-col gap-1">
            <span className={labelClass}>{t("exposure.mode")}</span>
            <select
              className={cn(fieldClass, "w-auto")}
              value={mode}
              onChange={(e) => setMode(e.target.value)}
            >
              {modes.map((m) => (
                <option key={m.key} value={m.key}>
                  {modeName(t, spellId, m.key)}
                </option>
              ))}
            </select>
          </label>
        )}
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{t("exposure.round")}</span>
          <input
            type="number"
            min={1}
            className={cn(fieldClass, "w-16 tabular-nums")}
            value={round}
            onChange={(e) => setRound(e.target.value)}
          />
        </label>
        <label className="flex h-7 items-center gap-2 text-body">
          <input
            type="checkbox"
            checked={onSelf}
            onChange={(e) => setOnSelf(e.target.checked)}
          />
          {t("exposure.onMe")}
        </label>
        <Button
          size="sm"
          disabled={rolling || !Number.isFinite(roundNumber)}
          onClick={() =>
            void rollExposure({
              spellId,
              casterLevel,
              mode: modes.length > 0 ? mode : undefined,
              round: roundNumber,
              onSelf,
            })
          }
        >
          {t("exposure.roll")}
        </Button>
      </div>
    </div>
  );
}
