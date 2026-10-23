import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useTableStore } from "@/stores/table";
import { useRollStore } from "@/stores/roll";
import { useSkillsStore } from "@/stores/skills";
import { Button } from "@/components/ui/button";
import { ThiefPointsEditor } from "./ThiefPointsEditor";
import { useSkillsEditor } from "@/stores/skillsEditor";
import { PickEditor } from "@/components/proficiencies/PickEditor";
import {
  ProficiencyNameButton,
  type ProficiencyFacts,
} from "@/components/proficiencies/ProficiencyDialog";
import { SkillBudgetSummary, SkillTerms } from "./SkillBreakdown";
import { useCharacterStore } from "@/stores/character";
import { skillName } from "@/journal/words";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { isSpecialization } from "@/components/proficiencies/weapons";
import type {
  NonweaponPool,
  WeaponPickFigures,
  WeaponPool,
} from "@/types/character";
import { localized } from "@/i18n/localized";
import { primaryEntry } from "@/lib/classEntries";

/**
 * Skills tab of the Act panel (ADR 0014; layout ADR 0027 step 12): the non-weapon proficiencies
 * (d20 at or under the target) and the bard or thief skills (d100 at or under the percentage,
 * computed by the server from the entered points, ADR 0030 steps 17 and 18, with a budget warning,
 * the skill armour the server read from the body armour worn (ADR 0033 step 14: changed by wearing
 * armour in the inventory, never here) and the "Spend" editor), side by side
 * when the panel holds two 15rem columns, stacked otherwise. Each row is one button that rolls the
 * check on the server; a proficiency shows its ability with the modifier as a pill, a bard skill its
 * percentage. The target, the verdict and the DM's adjustment are on the result card, after the
 * roll (ADR 0027 step 8.3, ADR 0014 step 6).
 */
export function SkillsTab() {
  const { t, i18n } = useTranslation();
  const skills = useSkillsStore((s) => s.skills);
  const refresh = useTableStore((s) => s.refresh);
  const { rollSkill, rolling } = useRollStore();
  const derivedSets = useCharacterStore((s) => s.character?.derived.skillSets);
  const cls = useCharacterStore(
    (s) => primaryEntry(s.character?.classes)?.class ?? "",
  );
  const slots = useCharacterStore((s) => s.character?.derived.proficiencySlots);
  const race = useCharacterStore((s) => s.character?.raceCode);
  const languages = useCharacterStore((s) => s.character?.languages);
  // The open editor lives in a store: the character card's level-up grants open one too (ADR 0037
  // step 15). Leaving the tab closes it (ActionArea), as it always did.
  const editor = useSkillsEditor((s) => s.editor);
  const openEditor = useSkillsEditor((s) => s.open);
  const closeEditor = useSkillsEditor((s) => s.close);
  const editingSet = useSkillsEditor((s) => s.set);
  const editingPoints = editor === "skillPoints";
  const editingPicks = editor === "nonWeapon";
  const editingWeapons = editor === "weapon";
  // Read once shown; then the action module reads them again when a write names what they follow
  // (an effect, an item, the points, the level).
  useEffect(() => {
    void refresh(["skills"]);
  }, [refresh]);

  if (!skills) return null;
  // The server sends one skill set per rogue_skills feature the class grants and none for another
  // class: no set, no section (ADR 0030 steps 17 and 18, ADR 0040 step 3.5). The points editor
  // opened from the level-up's grants names no set: the first with points left to spend.
  const editedSet =
    editingSet ??
    (
      skills.skillSets.find((x) => (x.budget?.left ?? 0) > 0) ??
      skills.skillSets[0]
    )?.set;
  const rows = (
    heading: string,
    skillSet: string | undefined,
    list: {
      id: string;
      value: ReactNode;
      terms?: ReactNode;
      unusable?: boolean;
      /** The worn body armour blocks the skill (ADR 0042 step 16.3). */
      blocked?: boolean;
      /** A proficiency: its name opens the description, apart from the roll. */
      facts?: ProficiencyFacts;
    }[],
    after?: ReactNode,
  ) => {
    const label = heading;
    const id = `skills-${skillSet ?? "proficiency"}`;
    return (
      <section aria-labelledby={id} className="flex min-w-0 flex-col gap-2">
        <h3 id={id} className="group-heading section-label px-2.5">
          {label}
        </h3>
        <ul aria-label={label} className="flex flex-col gap-1">
          {list.map(({ id: skill, value, terms, unusable, blocked, facts }) => {
            const name = skillName(t, skill);
            // ADR 0030 step 17 (PHB p.39): a thieving skill under 1% waits for points.
            const label = blocked
              ? t("skills.blocked", { name })
              : unusable
                ? t("skills.unusable", { name })
                : t("skills.rollLabel", { name });
            if (facts)
              return (
                <li key={skill}>
                  {/* Two targets in one raised stripe: the name opens the description, the ability
                      and target part rolls the check. */}
                  <div
                    className="flex min-h-7 w-full items-stretch gap-1 rounded-md bg-raised text-body"
                    data-testid={`skill-row-${skill}`}
                  >
                    <ProficiencyNameButton
                      facts={facts}
                      className="min-w-0 flex-1 rounded-md px-2.5 py-0.5 text-left transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50"
                    />
                    <button
                      type="button"
                      className="flex items-center rounded-md px-2.5 py-0.5 transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted"
                      disabled={rolling || unusable}
                      aria-label={label}
                      title={label}
                      data-testid={`skill-${skill}`}
                      onClick={() => void rollSkill(skill, undefined, skillSet)}
                    >
                      {value}
                    </button>
                  </div>
                </li>
              );
            return (
              <li key={skill}>
                {/* The shared clickable row (the saves' rows): a raised stripe, hover and focus. */}
                <button
                  type="button"
                  className="grid min-h-7 w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md bg-raised px-2.5 py-0.5 text-left text-body transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted"
                  disabled={rolling || unusable}
                  aria-label={label}
                  title={label}
                  data-testid={`skill-${skill}`}
                  onClick={() => void rollSkill(skill, undefined, skillSet)}
                >
                  <span className="flex min-w-0 flex-col">
                    <span className="break-words">{name}</span>
                    {terms}
                    {blocked && (
                      <span
                        className="text-label text-warning"
                        data-testid={`skill-blocked-${skill}`}
                      >
                        {t("skills.blockedTag")}
                      </span>
                    )}
                  </span>
                  {value}
                </button>
              </li>
            );
          })}
        </ul>
        {after}
      </section>
    );
  };

  // ADR 0037 step 12: the picks are edited in place of the lists, the catalogue needing the room.
  if (editingPicks && slots && cls)
    return (
      <PickEditor
        pool="nonWeapon"
        cls={cls}
        race={race ?? ""}
        languages={languages ?? []}
        slots={slots}
        onDone={closeEditor}
      />
    );

  if (editingWeapons && slots && cls)
    return (
      <PickEditor
        pool="weapon"
        cls={cls}
        race={race ?? ""}
        languages={languages ?? []}
        slots={slots}
        onDone={closeEditor}
      />
    );

  return (
    <div
      className="grid grid-cols-[repeat(auto-fit,minmax(15rem,1fr))] items-start gap-x-6 gap-y-4"
      data-testid="skills-blocks"
    >
      {rows(
        t("skills.proficiencies"),
        undefined,
        skills.proficiencies.map((p) => {
          const pick = slots?.picks.find((f) => f.id === p.id);
          return {
            id: p.id,
            facts: {
              id: p.id,
              ability: p.ability,
              modifier: p.modifier,
              slots: pick?.slots,
              extraSlots: pick?.extraSlots,
            },
            value: (
              <Ability
                ability={p.ability}
                score={p.abilityScore}
                modifier={p.modifier}
              />
            ),
          };
        }),
        slots && (
          <>
            <ProficiencySlotsLine pool={slots.nonWeapon} />
            <Button
              type="button"
              size="sm"
              className="mx-2.5 w-fit"
              title={t("proficiencies.editHint")}
              onClick={() => openEditor("nonWeapon")}
              data-testid="proficiency-edit"
            >
              {t("proficiencies.edit")}
            </Button>
          </>
        ),
      )}
      {slots?.weapon && (
        <WeaponProficienciesBlock
          pool={slots.weapon}
          picks={slots.weaponPicks ?? []}
          onEdit={() => openEditor("weapon")}
        />
      )}
      {skills.skillSets.map((skillSet) => {
        const warnings =
          derivedSets?.find((d) => d.set === skillSet.set)?.warnings ?? [];
        const editing = editingPoints && editedSet === skillSet.set;
        return (
          <div
            key={skillSet.set}
            className="flex min-w-0 flex-col gap-2"
            data-testid={`skill-set-${skillSet.set}`}
          >
            {rows(
              localized(i18n.language, skillSet.name) || skillSet.set,
              skillSet.set,
              // ADR 0030 steps 17 and 18: the server's percentage; its terms in the tooltip. The
              // server says which skill cannot be rolled yet.
              skillSet.skills.map((s) => ({
                id: s.id,
                unusable: !s.usable,
                blocked: s.blocked,
                terms: (
                  <SkillTerms skill={s} cap={skillSet.budget?.perSkillCap} />
                ),
                value: (
                  <span
                    className="text-right font-semibold whitespace-nowrap tabular-nums"
                    title={t("skills.thiefTerms", {
                      base: s.base,
                      points: s.points,
                      race: formatSigned(s.race),
                      dexterity: formatSigned(s.dexterity),
                      armor: formatSigned(s.armor),
                      max: s.maxPercent,
                    })}
                    data-testid={`${skillSet.set}-percent-${s.id}`}
                  >
                    {t("skills.percent", { value: s.percent })}
                  </span>
                ),
              })),
            )}
            {skillSet.budget && !editing && (
              <SkillBudgetSummary budget={skillSet.budget} />
            )}
            {warnings.length > 0 && (
              <ul
                className="flex flex-col gap-1 px-2.5 text-body text-warning"
                data-testid="skill-budget"
              >
                {warnings.map((w) => (
                  <li key={w.code + (w.skill ?? "")} className="break-words">
                    {t(`skills.budget.${w.code}`, {
                      name: w.skill ? skillName(t, w.skill) : "",
                      points: w.points,
                      limit: w.limit,
                    })}
                  </li>
                ))}
              </ul>
            )}
            {skillSet.skillArmor && (
              <p
                className="px-2.5 text-body break-words text-muted-foreground"
                title={t("skills.armorHint")}
                data-testid="skill-armor"
              >
                {t("skills.armor", {
                  armor: t(`characters.armors.${skillSet.skillArmor}`, {
                    defaultValue: skillSet.skillArmor,
                  }),
                })}
              </p>
            )}
            {editing ? (
              <ThiefPointsEditor
                set={skillSet.set}
                skills={skillSet.skills}
                budget={skillSet.budget}
                onDone={closeEditor}
              />
            ) : (
              <Button
                type="button"
                size="sm"
                className="mx-2.5 w-fit"
                title={t("skills.spendPointsHint")}
                onClick={() => openEditor("skillPoints", skillSet.set)}
                data-testid="thief-points-edit"
              >
                {t("skills.spendPoints")}
              </Button>
            )}
          </div>
        );
      })}
    </div>
  );
}

/**
 * The weapon proficiencies (ADR 0037 steps 7 and 9): the weapon types picked, the specialization
 * tagged, a kept type the class may not pick marked as such, the pool's line, and Choisir, which
 * opens the editor (a rare control one click away, ADR 0027 step 1).
 */
function WeaponProficienciesBlock({
  pool,
  picks,
  onEdit,
}: {
  pool: WeaponPool;
  picks: WeaponPickFigures[];
  onEdit: () => void;
}) {
  const { t } = useTranslation();
  const label = t("weaponProficiencies.section");
  return (
    <section
      aria-labelledby="skills-weapons"
      className="flex min-w-0 flex-col gap-2"
      data-testid="weapon-proficiencies"
    >
      <h3 id="skills-weapons" className="group-heading section-label px-2.5">
        {label}
      </h3>
      {picks.length === 0 ? (
        <p className="px-2.5 text-body break-words text-ink-muted">
          {t("weaponProficiencies.none")}
        </p>
      ) : (
        <ul aria-label={label} className="flex flex-col gap-1">
          {picks.map((p) => (
            <li
              key={p.type}
              className="flex min-h-7 flex-wrap items-center gap-x-2 gap-y-0.5 rounded-md bg-raised px-2.5 py-0.5 text-body"
              data-testid={`weapon-pick-${p.type}`}
            >
              <span className="min-w-0 break-words">
                {t(`weapons.types.${p.type}`, { defaultValue: p.type })}
              </span>
              {isSpecialization(p) && (
                <span
                  className="chip-chosen shrink-0 px-1.5 text-label font-medium"
                  data-testid="weapon-pick-specialized"
                >
                  {t("weaponProficiencies.specialized")}
                </span>
              )}
              {!p.allowed && (
                <span
                  className="min-w-0 text-label break-words text-warning"
                  data-testid="weapon-pick-not-allowed"
                >
                  {t("weaponProficiencies.notAllowed")}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
      <SlotsLine pool={pool} testId="weapon-slots-line" />
      <Button
        type="button"
        size="sm"
        className="mx-2.5 w-fit"
        title={t("weaponProficiencies.editHint")}
        onClick={onEdit}
        data-testid="weapon-edit"
      >
        {t("weaponProficiencies.edit")}
      </Button>
    </section>
  );
}

/** The slots spent of the available ones (ADR 0037 step 3) and, as the server's pool says, the
 * slots left to spend or the overspend's warning (step 11). */
function ProficiencySlotsLine({ pool }: { pool: NonweaponPool }) {
  return <SlotsLine pool={pool} testId="proficiency-slots-line" />;
}

/** A pool's line: spent of available, then what is left or the overspend (either pool). */
function SlotsLine({
  pool,
  testId,
}: {
  pool: { spent: number; available: number; left: number };
  testId: string;
}) {
  const { t } = useTranslation();
  return (
    <p
      className={cn(
        "px-2.5 text-body break-words tabular-nums",
        pool.left < 0 ? "text-warning" : "text-muted-foreground",
      )}
      data-testid={testId}
    >
      {t("proficiencies.tabLine", {
        spent: pool.spent,
        available: pool.available,
      })}
      {pool.left > 0 &&
        ` · ${t("proficiencies.toSpend", { count: pool.left })}`}
      {pool.left < 0 && ` · ${t("proficiencies.overShort")}`}
    </p>
  );
}

/** "DEX 17" and, when the proficiency has one, its modifier as a pill ("DEX 17 [−1]"). */
function Ability({
  ability,
  score,
  modifier,
}: {
  ability: string;
  score: number;
  modifier: number;
}) {
  const { t } = useTranslation();
  return (
    <span className="flex items-center justify-end gap-1.5 whitespace-nowrap tabular-nums">
      <span className="text-muted-foreground">
        {t(`attr.${ability}`, { defaultValue: ability })} {score}
      </span>{" "}
      {modifier !== 0 && (
        <span
          className="chip min-w-7 px-1.5 text-center font-semibold"
          data-testid="skill-modifier"
        >
          {formatSigned(modifier)}
        </span>
      )}
    </span>
  );
}
