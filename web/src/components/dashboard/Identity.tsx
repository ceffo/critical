import { useTranslation } from "react-i18next";
import { ArrowsLeftRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { formatStrength } from "@/lib/format";
import { navigate } from "@/lib/navigation";
import { ROSTER_PATH } from "@/lib/session";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { useRollStore } from "@/stores/roll";
import { useTableStore } from "@/stores/table";
import { useLookupStore } from "@/stores/lookup";
import { useSkillsEditor, type SkillsEditor } from "@/stores/skillsEditor";
import type { FreeSpell } from "@/types/character";
import { useXpStore } from "@/stores/xp";
import { CharacterDetails } from "./CharacterDetails";
import { LevelUpPrompt } from "./LevelUpPrompt";
import { ToSpendTag } from "./ToSpendTag";

const ABILITIES = ["str", "dex", "con", "int", "wis", "cha"] as const;
const CELL = "flex min-w-0 flex-col items-center rounded-md bg-raised py-1";

/**
 * Who: the name, the class under it (muted), the level right-aligned on the name's line (a button
 * opening the Expérience workspace, underlined with the server's progress to the next level, ADR
 * 0015) and right of it the way into the roster, "Vos personnages" (ADR 0034 step 7.1), then the
 * abilities in one row of seven cells and THAC0 last.
 * Each ability tile rolls its check on the server (ADR 0028): the result card shows it.
 * STR is the server's effective value; changed by an effect, it shows only that value, in the
 * success colour, and the sheet value is in a tooltip on hover and keyboard focus, never under it
 * (ADR 0027 step 5.4: a click on a stat is kept for the ability check). The level and THAC0 are the
 * server's, derived from class and XP (ADR 0030 steps 3 and 5). A level whose hit points are owed
 * shows its prompt there too (step 7, LevelUpPrompt), and what a level grants or what is left to
 * spend, with the ways into the Skills tab's editors (ADR 0037 step 15).
 */
export function Identity({
  onOpenXp,
  onOpenSkills,
  onOpenSpells,
}: {
  onOpenXp: () => void;
  /** Focuses the Act activity, whose Skills tab holds the proficiency and skill point editors. */
  onOpenSkills: () => void;
  /** Focuses the Spells activity, the spell lookup (a free spell's link, ADR 0030 step 20.8). */
  onOpenSpells: () => void;
}) {
  const { t, i18n } = useTranslation();
  const xp = useXpStore((s) => s.ledger);
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  const strBuffed =
    strength !== null &&
    (strength.effective !== strength.base ||
      strength.effectiveExceptional !== strength.baseExceptional);
  // The score in effect and the sheet's, an 18/xx as 18/50 (ADR 0030 step 9).
  const strText = strength && {
    effective: formatStrength(
      strength.effective,
      strength.effectiveExceptional,
    ),
    base: formatStrength(strength.base, strength.baseExceptional),
  };
  const rollAbility = useRollStore((s) => s.rollAbility);
  const rollLevelUp = useCharacterStore((s) => s.rollLevelUp);
  const levelUpBusy = useTableStore((s) => s.busy);
  const openEditor = useSkillsEditor((s) => s.open);
  // ADR 0037 step 15's links: the editor, in the Act activity's Skills tab.
  const openSkillsEditor = (editor: SkillsEditor) => {
    openEditor(editor);
    onOpenSkills();
  };
  // ADR 0030 step 20.8's link: the lookup on the free spell's spell level, Ajouter claiming it.
  const openFreeSpellLookup = useLookupStore((s) => s.openFreeSpell);
  const openFreeSpell = (fs: FreeSpell) => {
    openFreeSpellLookup(fs);
    onOpenSpells();
  };

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
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <p
                className="text-body text-muted-foreground"
                data-testid="class"
              >
                {character.classes
                  .map((e) =>
                    i18n.exists(`details.classes.${e.class}`)
                      ? t(`details.classes.${e.class}`)
                      : e.class,
                  )
                  .join(" / ")}
              </p>
              <ToSpendTag
                toSpend={character.derived.levelUp.toSpend}
                onOpenEditor={openSkillsEditor}
                onOpenFreeSpell={openFreeSpell}
              />
            </div>
          )}
        </div>
        {character && (
          <div className="flex shrink-0 items-center gap-2">
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
              {t("app.levelValue", { level: character.derived.realizedLevel })}
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
            <Button
              variant="ghost"
              size="icon-sm"
              className="shrink-0 shadow-[inset_0_0_0_1.5px_var(--tint)]"
              aria-label={t("roster.open")}
              title={t("roster.open")}
              data-testid="roster-open"
              onClick={() => navigate(ROSTER_PATH)}
            >
              <ArrowsLeftRight aria-hidden />
            </Button>
          </div>
        )}
      </div>
      {character && (
        <div
          role="group"
          className={cn(
            "grid gap-1",
            // An 18/xx Strength takes a wider first tile (ADR 0030 step 9), never a cut one.
            strength && strength.effectiveExceptional > 0
              ? "grid-cols-[minmax(0,1.5fr)_repeat(5,minmax(0,1fr))_minmax(0,1.35fr)]"
              : "grid-cols-[repeat(6,minmax(0,1fr))_minmax(0,1.35fr)]",
          )}
          aria-label={t("app.abilities")}
        >
          {ABILITIES.map((k) => {
            const isStr = k === "str" && strength !== null;
            const buffed = isStr && strBuffed;
            const score = isStr && strText ? strText.effective : character[k];
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
                    <span data-testid="str-value">
                      {/* An 18/xx: the percentile smaller, so the tile fits it (ADR 0030 step 9). */}
                      {strength.effective}
                      {strength.effectiveExceptional > 0 && (
                        <span className="text-label">
                          {strText?.effective.slice(
                            String(strength.effective).length,
                          )}
                        </span>
                      )}
                    </span>
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
                  {t("app.strTitle", { ...strText })}
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
      {character && (
        <LevelUpPrompt
          levelUp={character.derived.levelUp}
          realizedLevel={character.derived.realizedLevel}
          targetLevel={character.derived.targetLevel}
          busy={levelUpBusy}
          onRoll={rollLevelUp}
          onOpenEditor={openSkillsEditor}
          onOpenFreeSpell={openFreeSpell}
        />
      )}
    </div>
  );
}
