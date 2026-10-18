import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Skull } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { turnUndead } from "@/api/character";
import { undeadName } from "@/lib/logSummary";
import { useCharacterStore } from "@/stores/character";
import { useFilStore } from "@/stores/fil";
import { useTableStore } from "@/stores/table";
import type { TurningTarget } from "@/types/character";

/** The row's Hit Dice as the book lists them: none (by type only), one value, a range or "and more". */
function hitDice(t: TFunction, r: TurningTarget): string {
  if (r.hdMin === 0) return "";
  if (r.hdMax === 0) return t("turning.hdMore", { hd: r.hdMin });
  if (r.hdMin === r.hdMax) return t("turning.hd", { hd: r.hdMin });
  return t("turning.hdRange", { min: r.hdMin, max: r.hdMax });
}

/** The cell at the cleric's level, as the server serves it: the d20 target or the outcome. */
function cellText(t: TFunction, r: TurningTarget): string {
  if (r.roll) return t("turning.target", { target: r.roll });
  return t(`turning.outcome.${r.outcome ?? "cannot"}`);
}

/**
 * Turning undead (ADR 0030 step 16): PHB Table 61 at the cleric's realized level, one row per undead
 * type with its cell. Picking a type asks the server to turn it; the result card and the journal show
 * the breakdown. A dash ("cannot") is not offered. The table and the dice are the server's.
 */
export function TurnUndeadPanel() {
  const { t } = useTranslation();
  const rows = useCharacterStore((s) => s.character?.derived.turning ?? null);
  const run = useTableStore((s) => s.run);
  const busy = useTableStore((s) => s.busy);
  const showLatest = useFilStore((s) => s.showLatest);
  if (!rows) return null;

  const turn = async (undead: string) => {
    await run(async () => {
      await turnUndead(undead);
    });
    showLatest();
  };

  return (
    <div className="flex flex-col gap-3" data-testid="turn-undead">
      <div>
        <h3 className="section-label">{t("turning.title")}</h3>
        <p className="text-label text-muted-foreground">{t("turning.hint")}</p>
      </div>
      <ul className="flex flex-col gap-1">
        {rows.map((r) => {
          const name = undeadName(t, r.undead);
          const hd = hitDice(t, r);
          const cannot = !r.roll && (!r.outcome || r.outcome === "cannot");
          return (
            <li
              key={r.undead}
              className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg bg-raised px-3 py-1.5"
              data-testid={`turn-row-${r.undead}`}
            >
              <span className="min-w-0 flex-1 basis-32 text-body font-medium">
                {name}
                {hd && (
                  <span className="ml-2 text-label text-muted-foreground">
                    {hd}
                  </span>
                )}
              </span>
              <span
                className="text-body tabular-nums"
                data-testid={`turn-cell-${r.undead}`}
              >
                {cellText(t, r)}
              </span>
              <Button
                size="sm"
                disabled={busy || cannot}
                onClick={() => void turn(r.undead)}
                aria-label={t("turning.turnLabel", { undead: name })}
              >
                <Skull aria-hidden />
                {t("turning.turn")}
              </Button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
