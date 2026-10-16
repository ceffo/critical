import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Check, X } from "@phosphor-icons/react";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { useSkillsStore } from "@/stores/skills";
import { abilityText, skillName } from "@/lib/logSummary";
import { formatSigned } from "@/lib/format";
import {
  maxSkillAdjustment,
  previewTarget,
  stepSkillAdjustment,
} from "@/lib/skillAdjust";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import type { LastCheck, SkillKind } from "@/types/skills";

/**
 * Skills tab of the Act panel (ADR 0014; layout ADR 0022 step 2.5): the non-weapon proficiencies (d20 at or under
 * the target) and the bard skills (d100 at or under the percentage). Each row is one button that
 * rolls the check on the server; targets, verdicts and the last check are the server's. Each
 * section's header has a stepper for the DM's adjustment of its next check (ADR 0014 steps 5 and 8):
 * the rows preview the adjusted target, the roll sends it and the stepper resets.
 */
export function SkillsTab() {
  const { t } = useTranslation();
  const skills = useSkillsStore((s) => s.skills);
  const fetchSkills = useSkillsStore((s) => s.fetchSkills);
  const { rollSkill, rolling } = useRollStore();
  const pending = useSkillsStore((s) => s.pending);
  const setPending = useSkillsStore((s) => s.setPending);
  const logEntries = useLogStore((s) => s.entries);
  // A check, an undo or a void (any new log state) can change the last checks.
  useEffect(() => {
    void fetchSkills();
  }, [fetchSkills, logEntries]);

  if (!skills) return null;
  const rowClass =
    "grid w-full grid-cols-[minmax(0,1fr)_7rem_4rem_4.5rem] items-center gap-3 rounded-lg px-3 py-1.5 text-left text-body hover:bg-neu-800 disabled:opacity-60";
  // Column headers are section labels (ADR 0022 step 4.2).
  const headClass = cn(rowClass, "py-1 hover:bg-transparent");
  const roll = async (kind: SkillKind, id: string) => {
    if (await rollSkill(id, pending[kind])) setPending(kind, 0);
  };

  return (
    <div className="flex flex-col gap-6">
      <section
        aria-labelledby="skills-proficiencies"
        className="rounded-lg bg-neu-900 p-2"
      >
        <div className="flex items-center justify-between gap-3 px-3 py-1">
          <h3 id="skills-proficiencies" className="section-label">
            {t("skills.proficiencies")}
          </h3>
          <AdjustStepper kind="proficiency" />
        </div>
        <div className={headClass}>
          <span />
          <span className="section-label">{t("skills.columns.ability")}</span>
          <span className="section-label text-right">
            {t("skills.columns.target")}
          </span>
          <span className="section-label text-right">
            {t("skills.columns.last")}
          </span>
        </div>
        <ul
          aria-label={t("skills.proficiencies")}
          className="flex flex-col gap-1"
        >
          {skills.proficiencies.map((p) => {
            const name = skillName(t, p.id);
            return (
              <li key={p.id}>
                <button
                  type="button"
                  className={rowClass}
                  disabled={rolling}
                  aria-label={t("skills.rollLabel", { name })}
                  data-testid={`skill-${p.id}`}
                  onClick={() => void roll("proficiency", p.id)}
                >
                  <span className="break-words">{name}</span>
                  <span className="text-muted-foreground tabular-nums">
                    {abilityText(t, p.ability, p.abilityScore, p.modifier)}
                  </span>
                  <Target value={p.target} adjustment={pending.proficiency} />
                  <Last last={p.last} />
                </button>
              </li>
            );
          })}
        </ul>
      </section>
      <section
        aria-labelledby="skills-bard"
        className="rounded-lg bg-neu-900 p-2"
      >
        <div className="flex items-center justify-between gap-3 px-3 py-1">
          <h3 id="skills-bard" className="section-label">
            {t("skills.bardSkills")}
          </h3>
          <AdjustStepper kind="bardSkill" />
        </div>
        <div className={headClass}>
          <span />
          <span />
          <span className="section-label text-right">
            {t("skills.columns.percent")}
          </span>
          <span className="section-label text-right">
            {t("skills.columns.last")}
          </span>
        </div>
        <ul aria-label={t("skills.bardSkills")} className="flex flex-col gap-1">
          {skills.bardSkills.map((b) => {
            const name = skillName(t, b.id);
            return (
              <li key={b.id}>
                <button
                  type="button"
                  className={rowClass}
                  disabled={rolling}
                  aria-label={t("skills.rollLabel", { name })}
                  data-testid={`skill-${b.id}`}
                  onClick={() => void roll("bardSkill", b.id)}
                >
                  <span className="break-words">{name}</span>
                  <span />
                  <Target
                    value={b.percent}
                    adjustment={pending.bardSkill}
                    percent
                  />
                  <Last last={b.last} />
                </button>
              </li>
            );
          })}
        </ul>
      </section>
    </div>
  );
}

/**
 * The section's pending adjustment: − / + around a signed value (proficiencies by 1, bard skills
 * by 5 points; + easier, − harder). It applies to the section's next check only.
 */
function AdjustStepper({ kind }: { kind: SkillKind }) {
  const { t } = useTranslation();
  const value = useSkillsStore((s) => s.pending[kind]);
  const setPending = useSkillsStore((s) => s.setPending);
  const section = t(
    kind === "proficiency" ? "skills.proficiencies" : "skills.bardSkills",
  );
  return (
    <div
      role="group"
      aria-label={t("skills.adjust.label", { section })}
      className="flex items-center gap-1"
      data-testid={`skills-adjust-${kind}`}
    >
      <Button
        type="button"
        size="xs"
        variant="outline"
        aria-label={t("skills.adjust.decrease", { section })}
        disabled={value <= -maxSkillAdjustment(kind)}
        onClick={() => setPending(kind, stepSkillAdjustment(kind, value, -1))}
      >
        −
      </Button>
      <output
        className={cn(
          "w-12 text-center text-body font-semibold tabular-nums",
          value === 0 && "text-muted-foreground",
        )}
        aria-label={t("skills.adjust.value", { section })}
        data-testid={`skills-adjust-${kind}-value`}
      >
        {formatSigned(value)}
      </output>
      <Button
        type="button"
        size="xs"
        variant="outline"
        aria-label={t("skills.adjust.increase", { section })}
        disabled={value >= maxSkillAdjustment(kind)}
        onClick={() => setPending(kind, stepSkillAdjustment(kind, value, 1))}
      >
        +
      </Button>
    </div>
  );
}

/** A row's target: the sheet's, or while the section has a pending adjustment the adjusted one with a signed tag. */
function Target({
  value,
  adjustment,
  percent = false,
}: {
  value: number;
  adjustment: number;
  percent?: boolean;
}) {
  const { t } = useTranslation();
  const shown = previewTarget(value, adjustment);
  const text = percent ? t("skills.percent", { value: shown }) : shown;
  if (adjustment === 0)
    return (
      <span className="text-right font-semibold tabular-nums">{text}</span>
    );
  return (
    <span
      className="flex flex-wrap items-baseline justify-end gap-x-1 text-right font-semibold tabular-nums"
      data-testid="skill-adjusted"
    >
      {text}
      <span className="text-label font-medium text-amb-300">
        {formatSigned(adjustment)}
      </span>
    </span>
  );
}

/** The last check: its roll and ✓ / ✗, or a dash before the first one. */
function Last({ last }: { last: LastCheck | null }) {
  const { t } = useTranslation();
  if (!last) {
    return (
      <span
        className="text-right text-muted-foreground"
        data-testid="skill-last"
      >
        –
      </span>
    );
  }
  return (
    <span
      className={cn(
        "flex items-center justify-end gap-1 font-semibold tabular-nums",
        last.success ? "text-success" : "text-danger",
      )}
      data-testid="skill-last"
      data-success={last.success}
      title={t(last.success ? "skills.lastSuccess" : "skills.lastFailure", {
        roll: last.roll,
      })}
    >
      {last.roll}
      {last.success ? (
        <Check aria-hidden weight="bold" />
      ) : (
        <X aria-hidden weight="bold" />
      )}
    </span>
  );
}
