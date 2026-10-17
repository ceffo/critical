import { useTranslation } from "react-i18next";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { useXpStore } from "@/stores/xp";
import { CharacterDetails } from "./CharacterDetails";

const ABILITIES = ["str", "dex", "con", "int", "wis", "cha"] as const;
const CELL = "flex min-w-0 flex-col items-center rounded-md bg-raised py-1";

/**
 * Who: name with the level right-aligned on the same line (a button opening the Expérience workspace, underlined with the server's
 * progress to the next level, ADR 0015), then the abilities in one row of seven cells and THAC0 last.
 * STR is the server's effective value; changed by an effect, it shows only that value, in the
 * success colour, and the sheet value is in a tooltip on hover and keyboard focus, never under it
 * (ADR 0027 step 5.4: a click on a stat is kept for the ability check).
 */
export function Identity({ onOpenXp }: { onOpenXp: () => void }) {
  const { t, i18n } = useTranslation();
  const xp = useXpStore((s) => s.ledger);
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  const strBuffed = strength !== null && strength.effective !== strength.base;

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex min-w-0 items-baseline justify-between gap-2">
        <h1 className="min-w-0 flex-1">
          <CharacterDetails character={character} />
        </h1>
        {character && (
          <button
            type="button"
            onClick={onOpenXp}
            title={
              xp?.nextLevel
                ? t("xp.levelLinkTitle", {
                    xp: new Intl.NumberFormat(i18n.language).format(xp.total),
                    percent: xp.progressPercent,
                    level: xp.nextLevel,
                  })
                : undefined
            }
            className="relative shrink-0 pb-1 text-body text-muted-foreground tabular-nums outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            {t("app.levelValue", { level: character.level })}
            {xp && (
              <span
                aria-hidden
                className="absolute inset-x-0 bottom-0 h-0.5 rounded-full bg-input"
              >
                <span
                  className="absolute inset-y-0 left-0 rounded-full bg-tint"
                  style={{ width: `${xp.progress * 100}%` }}
                  data-testid="level-progress"
                />
              </span>
            )}
          </button>
        )}
      </div>
      {character && (
        <dl
          className="grid grid-cols-[repeat(6,minmax(0,1fr))_minmax(0,1.35fr)] gap-1"
          aria-label={t("app.abilities")}
        >
          {ABILITIES.map((k) => {
            const isStr = k === "str" && strength !== null;
            const cell = (
              <>
                <dt className="text-label text-muted-foreground">
                  {t(`attr.${k}`)}
                </dt>
                <dd
                  className={cn(
                    "text-key tabular-nums",
                    isStr && strBuffed && "text-success",
                  )}
                >
                  {isStr ? (
                    <span data-testid="str-value">{strength.effective}</span>
                  ) : (
                    character[k]
                  )}
                </dd>
              </>
            );
            if (!(isStr && strBuffed))
              return (
                <div key={k} className={CELL}>
                  {cell}
                </div>
              );
            return (
              <Tooltip key={k}>
                <TooltipTrigger
                  delay={200}
                  render={
                    <div
                      tabIndex={0}
                      data-testid="str-cell"
                      className={cn(
                        CELL,
                        "outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                      )}
                    />
                  }
                >
                  {cell}
                </TooltipTrigger>
                <TooltipContent>
                  {t("app.strTitle", { ...strength })}
                </TooltipContent>
              </Tooltip>
            );
          })}
          <div className={CELL}>
            <dt className="text-label text-muted-foreground">
              {t("app.thac0")}
            </dt>
            <dd className="text-key tabular-nums" data-testid="thac0-value">
              {character.thac0Base}
            </dd>
          </div>
        </dl>
      )}
    </div>
  );
}
