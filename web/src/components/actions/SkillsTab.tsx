import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useTableStore } from "@/stores/table";
import { useRollStore } from "@/stores/roll";
import { useSkillsStore } from "@/stores/skills";
import {
  ProficiencyNameButton,
  type ProficiencyFacts,
} from "@/components/proficiencies/ProficiencyDialog";
import { SkillTerms } from "./SkillBreakdown";
import { useCharacterStore } from "@/stores/character";
import { skillName } from "@/journal/words";
import { formatSigned } from "@/lib/format";
import { localized } from "@/i18n/localized";

/**
 * Skills tab of the Act panel (ADR 0014; layout ADR 0027 step 12): the non-weapon proficiencies
 * (d20 at or under the target) and the bard or thief skills (d100 at or under the percentage,
 * computed by the server from the entered points, ADR 0030 steps 17 and 18, with a budget warning,
 * the skill armour the server read from the body armour worn (ADR 0033 step 14: changed by wearing
 * armour in the inventory, never here)), side by side
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
  const slots = useCharacterStore((s) => s.character?.derived.proficiencySlots);
  // Read once shown; then the action module reads them again when a write names what they follow
  // (an effect, an item, the points, the level).
  useEffect(() => {
    void refresh(["skills"]);
  }, [refresh]);

  if (!skills) return null;
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

  // Nothing to roll (a class with no proficiency picked and no skill set): say so, and where the
  // slots are spent (ADR 0022 step 2.5, amended).
  if (skills.proficiencies.length === 0 && skills.skillSets.length === 0)
    return (
      <p
        className="px-2.5 text-body break-words text-ink-muted"
        data-testid="skills-empty"
      >
        {t("skills.empty")}
      </p>
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
      )}
      {skills.skillSets.map((skillSet) => {
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
          </div>
        );
      })}
    </div>
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
