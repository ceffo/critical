import { useTranslation } from "react-i18next";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { useRollStore } from "@/stores/roll";
import { useXpStore } from "@/stores/xp";
import type { Character } from "@/types/character";
import { CharacterDetails } from "./CharacterDetails";

const ABILITIES = ["str", "dex", "con", "int", "wis", "cha"] as const;
const CELL = "flex min-w-0 flex-col items-center rounded-md bg-raised py-1";

/**
 * Who: the name, the class under it (muted), the level right-aligned on the name's line (a button opening the Expérience workspace, underlined with the server's
 * progress to the next level, ADR 0015), then the abilities in one row of seven cells and THAC0 last.
 * Each ability tile rolls its check on the server (ADR 0028): the result card shows it.
 * STR is the server's effective value; changed by an effect, it shows only that value, in the
 * success colour, and the sheet value is in a tooltip on hover and keyboard focus, never under it
 * (ADR 0027 step 5.4: a click on a stat is kept for the ability check). The level and THAC0 are the
 * server's, derived from class and XP (ADR 0030 steps 3 and 5); a difference with the imported
 * sheet is listed under the abilities (step 24).
 */
export function Identity({ onOpenXp }: { onOpenXp: () => void }) {
  const { t, i18n } = useTranslation();
  const xp = useXpStore((s) => s.ledger);
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  const strBuffed = strength !== null && strength.effective !== strength.base;
  const rollAbility = useRollStore((s) => s.rollAbility);

  return (
    <div className="flex min-w-0 flex-col gap-2">
      {/* The identity's sub-band at the card's top, in place of its top padding (ADR 0026 6.3, 6.5). */}
      <div
        className={cn(
          "group-band bleed-x-4 flex min-w-0 items-baseline justify-between gap-2 py-2",
        )}
      >
        <div className="flex min-w-0 flex-1 flex-col">
          <h1 className="min-w-0">
            <CharacterDetails character={character} />
          </h1>
          {character && (
            <p className="text-body text-muted-foreground" data-testid="class">
              {i18n.exists(`details.classes.${character.class}`)
                ? t(`details.classes.${character.class}`)
                : character.class}
            </p>
          )}
        </div>
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
            {t("app.levelValue", { level: character.derived.level })}
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
        <div
          role="group"
          className="grid grid-cols-[repeat(6,minmax(0,1fr))_minmax(0,1.35fr)] gap-1"
          aria-label={t("app.abilities")}
        >
          {ABILITIES.map((k) => {
            const isStr = k === "str" && strength !== null;
            const buffed = isStr && strBuffed;
            const score = isStr ? strength.effective : character[k];
            const cell = (
              <>
                <span className="text-label text-muted-foreground">
                  {t(`attr.${k}`)}
                </span>
                <span
                  className={cn(
                    "text-key tabular-nums",
                    buffed && "text-success",
                  )}
                >
                  {isStr ? (
                    <span data-testid="str-value">{strength.effective}</span>
                  ) : (
                    character[k]
                  )}
                </span>
              </>
            );
            // The tile is the check's roll button (ADR 0028: click, Enter or Space); the sheet's
            // score of a changed STR is in the tooltip.
            const button = (
              <button
                type="button"
                aria-label={t("abilityCheck.roll", {
                  name: t(`abilityCheck.names.${k}`),
                  score,
                })}
                data-testid={buffed ? "str-cell" : `ability-${k}`}
                onClick={() => void rollAbility(k)}
                className={cn(
                  CELL,
                  "cursor-pointer outline-none hover:bg-input focus-visible:ring-3 focus-visible:ring-ring/50",
                )}
              />
            );
            if (!buffed)
              return (
                <Tooltip key={k}>
                  <TooltipTrigger delay={400} render={button}>
                    {cell}
                  </TooltipTrigger>
                  <TooltipContent>
                    {t("abilityCheck.roll", {
                      name: t(`abilityCheck.names.${k}`),
                      score,
                    })}
                  </TooltipContent>
                </Tooltip>
              );
            return (
              <Tooltip key={k}>
                <TooltipTrigger delay={200} render={button}>
                  {cell}
                </TooltipTrigger>
                <TooltipContent>
                  {t("app.strTitle", { ...strength })}
                </TooltipContent>
              </Tooltip>
            );
          })}
          <div className={CELL}>
            <span className="text-label text-muted-foreground">
              {t("app.thac0")}
            </span>
            <span className="text-key tabular-nums" data-testid="thac0-value">
              {character.derived.thac0}
            </span>
          </div>
        </div>
      )}
      {character && character.derived.sheetDifferences.length > 0 && (
        <SheetDifferences differences={character.derived.sheetDifferences} />
      )}
    </div>
  );
}

/**
 * The values the imported sheet states differently from the server's rules (ADR 0030 step 24):
 * each one with the sheet's figure and the computed one, which the app uses. Rare (a data or rules
 * error), so it shows only when there is one; the server sends codes and numbers, worded here.
 */
function SheetDifferences({
  differences,
}: {
  differences: Character["derived"]["sheetDifferences"];
}) {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      data-testid="sheet-differences"
      className="flex flex-col gap-0.5 rounded-md bg-raised px-2.5 py-1.5 text-body"
    >
      <p className="font-semibold text-warning">{t("app.sheetDiff.title")}</p>
      <ul className="flex flex-col">
        {differences.map((d) => (
          <li key={d.value} className="tabular-nums">
            {t("app.sheetDiff.item", {
              value: t(`app.sheetDiff.values.${d.value}`),
              sheet: d.sheet,
              computed: d.computed,
            })}
          </li>
        ))}
      </ul>
      <p className="text-muted-foreground">{t("app.sheetDiff.hint")}</p>
    </div>
  );
}
