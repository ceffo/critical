import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Check, X } from "@phosphor-icons/react";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { useSkillsStore } from "@/stores/skills";
import { abilityText, skillName } from "@/lib/logSummary";
import { cn } from "@/lib/utils";
import type { LastCheck } from "@/types/skills";

/**
 * Compétences tab of the Agir workspace (ADR 0014): the non-weapon proficiencies (d20 at or under
 * the target) and the bard skills (d100 at or under the percentage). Each row is one button that
 * rolls the check on the server; targets, verdicts and the last check are the server's.
 */
export function SkillsTab() {
  const { t } = useTranslation();
  const skills = useSkillsStore((s) => s.skills);
  const fetchSkills = useSkillsStore((s) => s.fetchSkills);
  const { rollSkill, rolling } = useRollStore();
  const logEntries = useLogStore((s) => s.entries);
  // A check, an undo or a void (any new log state) can change the last checks.
  useEffect(() => {
    void fetchSkills();
  }, [fetchSkills, logEntries]);

  if (!skills) return null;
  const rowClass =
    "grid w-full grid-cols-[minmax(0,1fr)_7rem_4rem_4.5rem] items-center gap-3 rounded-lg px-3 py-2 text-left text-body hover:bg-neu-900 disabled:opacity-60";

  return (
    <div className="flex flex-col gap-3">
      <section aria-labelledby="skills-proficiencies">
        <div
          className={cn(
            rowClass,
            "py-1 text-label text-muted-foreground hover:bg-transparent",
          )}
        >
          <h3 id="skills-proficiencies" className="text-title text-foreground">
            {t("skills.proficiencies")}
          </h3>
          <span>{t("skills.columns.ability")}</span>
          <span className="text-right">{t("skills.columns.target")}</span>
          <span className="text-right">{t("skills.columns.last")}</span>
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
                  onClick={() => void rollSkill(p.id)}
                >
                  <span className="truncate">{name}</span>
                  <span className="text-muted-foreground tabular-nums">
                    {abilityText(t, p.ability, p.abilityScore, p.modifier)}
                  </span>
                  <span className="text-right font-semibold tabular-nums">
                    {p.target}
                  </span>
                  <Last last={p.last} />
                </button>
              </li>
            );
          })}
        </ul>
      </section>
      <section aria-labelledby="skills-bard">
        <div
          className={cn(
            rowClass,
            "py-1 text-label text-muted-foreground hover:bg-transparent",
          )}
        >
          <h3 id="skills-bard" className="text-title text-foreground">
            {t("skills.bardSkills")}
          </h3>
          <span />
          <span className="text-right">{t("skills.columns.percent")}</span>
          <span className="text-right">{t("skills.columns.last")}</span>
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
                  onClick={() => void rollSkill(b.id)}
                >
                  <span className="truncate">{name}</span>
                  <span />
                  <span className="text-right font-semibold tabular-nums">
                    {t("skills.percent", { value: b.percent })}
                  </span>
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
        last.success ? "text-heal" : "text-danger",
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
