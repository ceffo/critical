import { useEffect, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { PendingButton } from "@/components/skills/PendingButton";
import { SkillNameButton } from "@/components/skills/SkillDialog";
import { SkillTermsTable } from "@/components/skills/SkillTermsTable";
import { ProficiencyNameButton } from "@/components/proficiencies/ProficiencyDialog";
import { isSpecialization } from "@/components/proficiencies/weapons";
import { localized } from "@/i18n/localized";
import { formatSigned } from "@/lib/format";
import { formula } from "@/lib/skillBudget";
import { cn } from "@/lib/utils";
import { weaponName } from "@/lib/weaponNames";
import { useEquipmentCatalogueStore } from "@/stores/equipmentCatalogue";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { raceLanguages } from "@/lib/languages";
import type { ProficiencySlots } from "@/types/character";
import type { Proficiency, SkillSet } from "@/types/skills";

/** A section of the read view: the title, the class tag of a multi-class character, the one Edit,
 * then the budget line and the rows. */
function Section({
  id,
  title,
  tag,
  pending,
  onModify,
  className,
  children,
}: {
  id: string;
  title: string;
  tag?: string;
  pending: boolean;
  onModify: () => void;
  className?: string;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <section
      aria-labelledby={`${id}-title`}
      className={cn(
        "flex min-w-0 flex-col gap-2 rounded-lg bg-raised p-3",
        className,
      )}
      data-testid={`${id}-read`}
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <h3 id={`${id}-title`} className="section-label min-w-0">
          {title}
        </h3>
        {tag && (
          <span
            className="chip px-2 py-0.5 text-label text-ink-muted"
            data-testid={`${id}-tag`}
          >
            {tag}
          </span>
        )}
        <span className="flex-1" />
        <PendingButton
          pending={pending}
          label={t("skillsActivity.modifyLabel", { section: title })}
          pendingLabel={t("skillsActivity.pending")}
          onClick={onModify}
          testId={`skills-modify-${id}`}
        >
          {t("skillsActivity.modify")}
        </PendingButton>
      </div>
      {children}
    </section>
  );
}

/** The budget line, "6 / 7 slots" with the server's formula after it; over budget in the warning
 * tone. */
function Budget({
  spent,
  available,
  over,
  detail,
  unit,
  testId,
}: {
  spent: number;
  available: number;
  over: boolean;
  detail?: string;
  unit: "slots" | "points";
  testId: string;
}) {
  const { t } = useTranslation();
  return (
    <p
      className="text-body break-words text-ink-muted tabular-nums"
      data-testid={testId}
      data-over={over ? "true" : undefined}
    >
      <b className={cn("font-semibold", over ? "text-warning" : "text-ink")}>
        {t(`skillsActivity.${unit === "slots" ? "slotsLine" : "pointsLine"}`, {
          spent,
          available,
          count: available,
        })}
      </b>
      {detail && ` · ${detail}`}
    </p>
  );
}

const rowClass =
  "flex min-w-0 items-center justify-between gap-3 rounded-md bg-sub-band px-2.5 py-1 text-body";

/**
 * The non-weapon proficiencies and the languages (ADR 0037 steps 3, 6 and 18), read: the pool as the
 * server derives it, each pick with its ability, the score in effect, its modifier, the target the
 * check rolls under and its slots, the languages after them. Every figure is the server's
 * (`derived.proficiencySlots`, `GET /api/skills`); a name opens its description (step 19).
 */
export function ProficienciesRead({
  slots,
  proficiencies,
  languages,
  race,
  tag,
  pending,
  onModify,
}: {
  slots: ProficiencySlots;
  proficiencies: Proficiency[];
  languages: string[];
  race: string;
  tag?: string;
  pending: boolean;
  onModify: () => void;
}) {
  const { t } = useTranslation();
  const { options } = useCreationOptions();
  const language = useLanguageNames(options);
  const native = options ? raceLanguages(options, race).native : undefined;
  const pool = slots.nonWeapon;
  return (
    <Section
      id="skills-nonWeapon"
      title={t("skillsActivity.sections.nonWeapon")}
      tag={tag}
      pending={pending}
      onModify={onModify}
      className="min-[1400px]:row-span-2"
    >
      <Budget
        spent={pool.spent}
        available={pool.available}
        over={pool.left < 0}
        unit="slots"
        testId="skills-nonWeapon-budget"
        detail={
          pool.languages > 0
            ? t("proficiencies.languagesLine", { count: pool.languages })
            : undefined
        }
      />
      {slots.picks.length === 0 ? (
        <p className="text-body text-ink-muted">
          {t("skillsActivity.noPicks")}
        </p>
      ) : (
        <ul
          className="flex flex-col gap-1"
          data-testid="skills-proficiency-rows"
        >
          {slots.picks.map((pick) => {
            const served = proficiencies.find((p) => p.id === pick.id);
            const ability = served?.ability ?? pick.ability;
            return (
              <li
                key={pick.id}
                className={cn(rowClass, "flex-wrap")}
                data-testid={`skill-read-${pick.id}`}
              >
                <ProficiencyNameButton
                  facts={{
                    id: pick.id,
                    ability,
                    modifier: served?.modifier ?? pick.modifier,
                    slots: pick.slots,
                    extraSlots: pick.extraSlots,
                    cost: pick.cost,
                  }}
                  className="min-w-0 rounded-md text-left underline-offset-2 hover:underline"
                />
                <span className="flex flex-wrap items-center justify-end gap-x-3 text-right">
                  {pick.target === null || !ability ? (
                    <span className="text-label text-ink-muted">
                      {t("proficiencies.noCheck")}
                    </span>
                  ) : (
                    <span
                      className="whitespace-nowrap tabular-nums"
                      data-testid={`skill-target-${pick.id}`}
                    >
                      <span className="text-ink-muted">
                        {t(`attr.${ability}`, { defaultValue: ability })}{" "}
                        {served?.abilityScore ?? ""}
                        {served && served.modifier !== 0
                          ? ` ${formatSigned(served.modifier)}`
                          : ""}{" "}
                        →{" "}
                      </span>
                      <b className="font-semibold">{pick.target}</b>
                    </span>
                  )}
                  <span className="text-label whitespace-nowrap text-ink-muted">
                    {pick.free
                      ? t("proficiencies.free")
                      : t("skillsActivity.slotCount", { count: pick.slots })}
                    {pick.extraSlots > 0 &&
                      ` · ${t("proficiencies.extra", { count: pick.extraSlots })}`}
                  </span>
                </span>
              </li>
            );
          })}
        </ul>
      )}
      <div className="flex flex-col gap-1 pt-1" data-testid="skills-languages">
        <span className="text-label text-ink-muted">
          {t("details.fields.languages")}
        </span>
        {languages.length === 0 ? (
          <p className="text-body text-ink-muted">
            {t("skillsActivity.noLanguages")}
          </p>
        ) : (
          <div className="flex flex-wrap gap-1.5">
            {languages.map((code) => (
              <span key={code} className="chip px-2 py-0.5 text-body">
                {language(code)}
                {code === native && (
                  <span className="ml-1.5 text-label text-ink-muted">
                    {t("skillsActivity.native")}
                  </span>
                )}
              </span>
            ))}
          </div>
        )}
      </div>
    </Section>
  );
}

/**
 * The weapon proficiencies (ADR 0037 steps 3 and 21), read: the pool, each held weapon with the
 * slots it costs and the specialization marked. The server's figures (`weaponPicks`).
 */
export function WeaponsRead({
  slots,
  tag,
  pending,
  onModify,
}: {
  slots: ProficiencySlots;
  tag?: string;
  pending: boolean;
  onModify: () => void;
}) {
  const { t, i18n } = useTranslation();
  const catalogue = useEquipmentCatalogueStore((s) => s.catalogue);
  const load = useEquipmentCatalogueStore((s) => s.load);
  useEffect(() => {
    void load().catch(() => undefined); // the codes name the rows meanwhile
  }, [load]);
  const pool = slots.weapon;
  if (!pool) return null;
  const picks = slots.weaponPicks ?? [];
  return (
    <Section
      id="skills-weapon"
      title={t("skillsActivity.sections.weapon")}
      tag={tag}
      pending={pending}
      onModify={onModify}
    >
      <Budget
        spent={pool.spent}
        available={pool.available}
        over={pool.left < 0}
        unit="slots"
        testId="skills-weapon-budget"
      />
      {picks.length === 0 ? (
        <p className="text-body text-ink-muted">
          {t("skillsActivity.noPicks")}
        </p>
      ) : (
        <ul className="flex flex-col gap-1" data-testid="skills-weapon-rows">
          {picks.map((p) => (
            <li
              key={p.code}
              className={rowClass}
              data-testid={`weapon-read-${p.code}`}
            >
              <span className="min-w-0 break-words">
                {weaponName(t, p.code, catalogue, i18n.language)}
              </span>
              <span className="flex shrink-0 items-center gap-2 text-label text-ink-muted">
                {isSpecialization(p) && (
                  <span
                    className="chip px-2 py-0.5 font-medium text-ink"
                    data-testid={`weapon-specialized-${p.code}`}
                  >
                    {t("weaponProficiencies.specialized")}
                  </span>
                )}
                {t("skillsActivity.slotCount", { count: p.slots })}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Section>
  );
}

/**
 * One skill set (ADR 0030 steps 17 and 18, ADR 0040 step 3.5), read: the points against the
 * budget and a row per skill with the terms of its percentage, the total last. The server's
 * figures (`GET /api/skills`); a name opens the set's description of the skill (ADR 0037 step
 * 19.6).
 */
export function SkillSetRead({
  set,
  pending,
  onModify,
}: {
  set: SkillSet;
  pending: boolean;
  onModify: () => void;
}) {
  const { t, i18n } = useTranslation();
  const b = set.budget;
  const title = localized(i18n.language, set.name) || set.set;
  return (
    <Section
      id={`skills-points-${set.set}`}
      title={title}
      pending={pending}
      onModify={onModify}
    >
      {b && (
        <Budget
          spent={b.spent}
          available={b.available}
          over={b.left < 0}
          unit="points"
          testId={`skills-points-${set.set}-budget`}
          detail={formula(t, b.first, b.perLevel, b.level, b.available)}
        />
      )}
      <SkillTermsTable
        setId={set.set}
        rows={set.skills}
        name={(r) => (
          <SkillNameButton
            set={set.set}
            skill={set.skills.find((s) => s.id === r.id) ?? set.skills[0]}
            className="rounded-md text-left break-words underline-offset-2 hover:underline"
          />
        )}
      />
      {set.skillArmor && (
        <p
          className="text-label break-words text-ink-muted"
          title={t("skills.armorHint")}
        >
          {t("skills.armor", {
            armor: t(`characters.armors.${set.skillArmor}`, {
              defaultValue: set.skillArmor,
            }),
          })}
          {b && b.perSkillCap > 0 && (
            <>
              {" · "}
              {t("skills.perSkillCap", {
                cap: b.perSkillCap,
                formula: formula(
                  t,
                  b.perSkillFirst,
                  b.perSkillPerLevel,
                  b.level,
                  b.perSkillCap,
                ),
              })}
            </>
          )}
        </p>
      )}
    </Section>
  );
}
