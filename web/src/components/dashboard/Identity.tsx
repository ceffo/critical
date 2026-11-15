import { useTranslation } from "react-i18next";
import { useState } from "react";
import { ArrowsLeftRight, CaretDown } from "@phosphor-icons/react";
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
import { useSkillsEditor } from "@/stores/skillsEditor";
import type { Character, FreeSpell } from "@/types/character";
import { useXpStore } from "@/stores/xp";
import { schoolOf, switchableEntries } from "@/lib/classSwitch";
import { useClassName } from "@/lib/useClassName";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { SwitchClassDialog } from "./SwitchClassDialog";
import { CharacterDetails } from "./CharacterDetails";
import { ClassEntries } from "./ClassEntries";
import { LevelUpPrompt } from "./LevelUpPrompt";
import { SpendDialog } from "./SpendDialog";
import { ToSpendTag, type SpendPools } from "./ToSpendTag";

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
  onOpenSpells,
}: {
  onOpenXp: () => void;
  /** Focuses the Spells activity, the spell lookup (a free spell's link, ADR 0030 step 20.8). */
  onOpenSpells: () => void;
}) {
  const { t, i18n } = useTranslation();
  const className = useClassName();
  const { options } = useCreationOptions();
  // The class entry whose school picker is open (ADR 0050 step 15.7): from the class name's link
  // or the details popover's "School".
  const [schoolEntry, setSchoolEntry] = useState<number | null>(null);
  const xp = useXpStore((s) => s.ledger);
  const character = useCharacterStore((s) => s.character);
  const strength = useBuffsStore((s) => s.strength);
  // A multi-class or dual-class character (ADR 0042): one line per class entry in place of the
  // single class and level.
  const entries = (character?.derived.classes?.length ?? 0) > 1;
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
  // ADR 0037 step 15's links: the editor, in the dialog the card hosts.
  const openSkillsEditor = useSkillsEditor((s) => s.open);
  // ADR 0030 step 20.8's link: the lookup on the free spell's spell level, Ajouter claiming it.
  const openFreeSpellLookup = useLookupStore((s) => s.openFreeSpell);
  const openFreeSpell = (fs: FreeSpell) => {
    openFreeSpellLookup(fs);
    onOpenSpells();
  };

  // The pools the character has: the tag stays while one has slots left or is over
  // budget, and goes once all are spent (ADR 0037 step 15, amended).
  const slots = character?.derived.proficiencySlots;
  const pools: SpendPools = {
    weaponSlots: slots?.weapon ?? undefined,
    nonWeaponSlots: slots?.nonWeapon,
    skillPoints:
      (character?.derived.skillSets.length ?? 0) > 0 ? {} : undefined,
  };

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <SpendDialog />
      {character && (
        <SwitchClassDialog
          character={character}
          entry={schoolEntry ?? 0}
          open={schoolEntry !== null}
          onOpenChange={(next) => {
            if (!next) setSchoolEntry(null);
          }}
        />
      )}
      {/* The identity's sub-band at the card's top, in place of its top padding (ADR 0026 6.3, 6.5). */}
      <div
        className={cn(
          "group-band bleed-x-4 flex min-w-0 items-baseline justify-between gap-2 py-2",
        )}
      >
        <div className="flex min-w-0 flex-1 flex-col">
          <h1 className="min-w-0">
            <CharacterDetails
              character={character}
              onOpenSchool={setSchoolEntry}
            />
          </h1>
          {character && (
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              {!entries && (
                <ClassLine
                  character={character}
                  onOpenSchool={setSchoolEntry}
                  switchable={switchableEntries(character, options)}
                  school={(cls) => {
                    const code = schoolOf(options, cls);
                    return code === ""
                      ? ""
                      : t(`lookup.schoolNames.${code}`, { defaultValue: code });
                  }}
                  name={className}
                />
              )}
              <ToSpendTag
                toSpend={character.derived.levelUp.toSpend}
                pools={pools}
                onOpenEditor={openSkillsEditor}
                onOpenFreeSpell={openFreeSpell}
              />
            </div>
          )}
        </div>
        {character && (
          <div className="flex shrink-0 items-center gap-2">
            {!entries && (
              <button
                type="button"
                onClick={onOpenXp}
                title={
                  xp?.nextLevel
                    ? t("xp.levelLinkTitle", {
                        xp: new Intl.NumberFormat(i18n.language).format(
                          xp.total,
                        ),
                        percent: xp.progressPercent,
                        level: xp.nextLevel,
                      })
                    : undefined
                }
                data-testid="level-open"
                className="hit-24 shrink-0 pb-1 text-body text-muted-foreground tabular-nums hover:text-foreground"
              >
                {t("app.levelValue", {
                  level: character.derived.realizedLevel,
                })}
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
      {character && entries && (
        <ClassEntries
          character={character}
          onOpenXp={onOpenXp}
          onOpenSchool={setSchoolEntry}
        />
      )}
      {character && (
        <div
          role="group"
          className={cn(
            "grid gap-1",
            // An 18/xx Strength takes a wider first tile (ADR 0030 step 9), never a cut one.
            strength && strength.effectiveExceptional > 0
              ? "grid-cols-[minmax(0,1.5fr)_repeat(5,minmax(0,1fr))_auto]"
              : "grid-cols-[repeat(6,minmax(0,1fr))_auto]",
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
                className={cn(CELL, "cursor-pointer hover:bg-input")}
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

/**
 * The class under the name. When the entry may switch (a mage, or a specialist, ADR 0050 step 15)
 * the class name is a link, dotted underline and caret like the level beside it, opening the school
 * picker: it reads the specialist and its school once switched ("Transmuter · Alteration") and still
 * opens the picker. Any other class is plain text.
 */
function ClassLine({
  character,
  switchable,
  school,
  name,
  onOpenSchool,
}: {
  character: Character;
  switchable: number[];
  school: (cls: string) => string;
  name: (cls: string) => string;
  onOpenSchool: (entry: number) => void;
}) {
  const { t } = useTranslation();
  const text = character.classes
    .map((e) => {
      const s = school(e.class);
      return s === "" ? name(e.class) : `${name(e.class)} · ${s}`;
    })
    .join(" / ");
  const entry = switchable[0];
  if (entry === undefined)
    return (
      <p className="text-body text-muted-foreground" data-testid="class">
        {text}
      </p>
    );
  const cls = character.classes[entry]?.class ?? "";
  return (
    <button
      type="button"
      onClick={() => onOpenSchool(entry)}
      title={
        school(cls) === ""
          ? t("switchClass.linkTitleGeneralist")
          : t("switchClass.linkTitleSpecialist", { school: school(cls) })
      }
      data-testid="class"
      className="hit-24-up inline-flex min-w-0 cursor-pointer items-center gap-0.5 rounded-sm text-left text-body text-foreground underline decoration-tint decoration-dotted underline-offset-4 hover:text-amb-300"
    >
      <span className="min-w-0 break-words">{text}</span>
      <CaretDown aria-hidden className="size-3 shrink-0" />
    </button>
  );
}
