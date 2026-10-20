import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useTableStore } from "@/stores/table";
import { useRollStore } from "@/stores/roll";
import { useSkillsStore } from "@/stores/skills";
import { Button } from "@/components/ui/button";
import { ThiefPointsEditor } from "./ThiefPointsEditor";
import { useSkillsEditor } from "@/stores/skillsEditor";
import { ProficiencyEditor } from "@/components/proficiencies/ProficiencyEditor";
import { SkillBudgetSummary, SkillTerms } from "./SkillBreakdown";
import { useCharacterStore } from "@/stores/character";
import { skillName } from "@/journal/words";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { WeaponProficiencyEditor } from "@/components/proficiencies/WeaponProficiencyEditor";
import { isSpecialization } from "@/components/proficiencies/weapons";
import type {
  NonweaponPool,
  WeaponPickFigures,
  WeaponPool,
} from "@/types/character";
import type { SkillKind } from "@/types/skills";

const HEADINGS: Record<SkillKind, string> = {
  proficiency: "skills.proficiencies",
  bardSkill: "skills.bardSkills",
  thiefSkill: "skills.thiefSkills",
};

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
  const { t } = useTranslation();
  const skills = useSkillsStore((s) => s.skills);
  const refresh = useTableStore((s) => s.refresh);
  const { rollSkill, rolling } = useRollStore();
  const budget = useCharacterStore((s) => s.character?.derived.skillBudget);
  const cls = useCharacterStore((s) => s.character?.class);
  const slots = useCharacterStore((s) => s.character?.derived.proficiencySlots);
  // The open editor lives in a store: the character card's level-up grants open one too (ADR 0037
  // step 15). Leaving the tab closes it (ActionArea), as it always did.
  const editor = useSkillsEditor((s) => s.editor);
  const openEditor = useSkillsEditor((s) => s.open);
  const closeEditor = useSkillsEditor((s) => s.close);
  const editingPoints = editor === "skillPoints";
  const editingPicks = editor === "nonWeapon";
  const editingWeapons = editor === "weapon";
  // Read once shown; then the action module reads them again when a write names what they follow
  // (an effect, an item, the points, the level).
  useEffect(() => {
    void refresh(["skills"]);
  }, [refresh]);

  if (!skills) return null;
  // The server sends the class skills of a thief or a bard and none for another class: no list, no
  // section (ADR 0030 steps 17 and 18).
  const classKind: SkillKind =
    skills.thiefSkills.length > 0 ? "thiefSkill" : "bardSkill";
  const classSkills =
    classKind === "thiefSkill" ? skills.thiefSkills : skills.bardSkills;
  const rows = (
    kind: SkillKind,
    list: {
      id: string;
      value: ReactNode;
      terms?: ReactNode;
      unusable?: boolean;
    }[],
    after?: ReactNode,
  ) => {
    const label = t(HEADINGS[kind]);
    const id = `skills-${kind}`;
    return (
      <section aria-labelledby={id} className="flex min-w-0 flex-col gap-2">
        <h3 id={id} className="group-heading section-label px-2.5">
          {label}
        </h3>
        <ul aria-label={label} className="flex flex-col gap-1">
          {list.map(({ id: skill, value, terms, unusable }) => {
            const name = skillName(t, skill);
            // ADR 0030 step 17 (PHB p.39): a thieving skill under 1% waits for points.
            const label = unusable
              ? t("skills.unusable", { name })
              : t("skills.rollLabel", { name });
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
                  onClick={() => void rollSkill(skill)}
                >
                  <span className="flex min-w-0 flex-col">
                    <span className="break-words">{name}</span>
                    {terms}
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
    return <ProficiencyEditor cls={cls} slots={slots} onDone={closeEditor} />;

  if (editingWeapons && slots && cls)
    return (
      <WeaponProficiencyEditor cls={cls} slots={slots} onDone={closeEditor} />
    );

  return (
    <div
      className="grid grid-cols-[repeat(auto-fit,minmax(15rem,1fr))] items-start gap-x-6 gap-y-4"
      data-testid="skills-blocks"
    >
      {rows(
        "proficiency",
        skills.proficiencies.map((p) => ({
          id: p.id,
          value: (
            <Ability
              ability={p.ability}
              score={p.abilityScore}
              modifier={p.modifier}
            />
          ),
        })),
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
      {classSkills.length > 0 && (
        <div className="flex min-w-0 flex-col gap-2">
          {rows(
            classKind,
            // ADR 0030 steps 17 and 18: the server's percentage; its terms in the tooltip. The
            // server says which skill cannot be rolled yet.
            classSkills.map((s) => ({
              id: s.id,
              unusable: !s.usable,
              terms: <SkillTerms skill={s} cap={skills.budget?.perSkillCap} />,
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
                  data-testid={`${classKind === "thiefSkill" ? "thief" : "bard"}-percent-${s.id}`}
                >
                  {t("skills.percent", { value: s.percent })}
                </span>
              ),
            })),
          )}
          {skills.budget && !editingPoints && (
            <SkillBudgetSummary budget={skills.budget} />
          )}
          {budget && budget.length > 0 && (
            <ul
              className="flex flex-col gap-1 px-2.5 text-body text-warning"
              data-testid="skill-budget"
            >
              {budget.map((w) => (
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
          {skills.skillArmor && (
            <p
              className="px-2.5 text-body break-words text-muted-foreground"
              title={t("skills.armorHint")}
              data-testid="skill-armor"
            >
              {t("skills.armor", {
                armor: t(`characters.armors.${skills.skillArmor}`, {
                  defaultValue: skills.skillArmor,
                }),
              })}
            </p>
          )}
          {editingPoints ? (
            <ThiefPointsEditor
              skills={classSkills}
              budget={skills.budget}
              onDone={closeEditor}
            />
          ) : (
            <Button
              type="button"
              size="sm"
              className="mx-2.5 w-fit"
              title={t("skills.spendPointsHint")}
              onClick={() => openEditor("skillPoints")}
              data-testid="thief-points-edit"
            >
              {t("skills.spendPoints")}
            </Button>
          )}
        </div>
      )}
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
