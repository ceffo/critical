// The skill check entry (skillsvc checkRecord, ADR 0014): a proficiency against its ability and
// modifier, a bard or thief skill against its percentage; the payload in effect is the latest
// adjustment's.
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { MusicNotes } from "@phosphor-icons/react";
import { useRollStore } from "@/stores/roll";
import { formatSigned } from "@/lib/format";
import {
  checkAdjustedTarget,
  maxSkillAdjustment,
  stepSkillAdjustment,
} from "@/lib/skillAdjust";
import type { SkillCheck } from "@/types/skills";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { buffName, englishSpellName } from "@/i18n/spellName";
import { abilityText, counters, skillName } from "../words";
import { AdjustCheck, CheckView } from "../parts";

/** The skill check's payload in effect, with its entry id. */
export function decodeSkillCheck(e: LogEntry): SkillCheck | null {
  const p = (e.effectivePayload ?? e.payload) as Partial<SkillCheck> | null;
  return p &&
    typeof p.id === "string" &&
    typeof p.roll === "number" &&
    typeof p.target === "number" &&
    typeof p.success === "boolean" &&
    (p.kind === "proficiency" ||
      p.kind === "bardSkill" ||
      p.kind === "thiefSkill")
    ? { ...(p as SkillCheck), logEntryId: e.id }
    : null;
}

/**
 * The check's line, with the buff its verdict in effect put on (ADR 0037 step 20): "Tumbling: d20
 * 12 vs 17, success; Tumbling on: AC −4, 1 rd". An entry logged before has no buff.
 */
function skillText(c: SkillCheck, t: TFunction): string {
  const check = checkText(c, t);
  const b = c.buff;
  if (!b) return check;
  const effects = [
    b.effects.ac !== undefined
      ? t("log.skill.buffAc", { value: formatSigned(b.effects.ac) })
      : null,
    counters(t, b.rounds),
  ]
    .filter((p): p is string => p !== null)
    .join(", ");
  return t(c.renewed ? "log.skill.buffRenewed" : "log.skill.buffOn", {
    check,
    buff: buffName(englishSpellName, b, t),
    effects,
  });
}

function checkText(c: SkillCheck, t: TFunction): string {
  if (c.adjustment) {
    return t(
      c.success ? "log.skill.successAdjusted" : "log.skill.failureAdjusted",
      {
        name: skillName(t, c.id),
        die: c.die,
        roll: c.roll,
        target: c.target,
        adjustedTarget: checkAdjustedTarget(c),
        adjustment: formatSigned(c.adjustment),
        reason: c.adjustmentReason ? ` ${c.adjustmentReason}` : "",
      },
    );
  }
  return t(c.success ? "log.skill.success" : "log.skill.failure", {
    name: skillName(t, c.id),
    die: c.die,
    roll: c.roll,
    target: c.target,
  });
}

export const skillCheckKind: KindView<SkillCheck, SkillCheck> = {
  decode: decodeSkillCheck,
  icon: MusicNotes,
  text: (p, w) => skillText(p, w.t),
  value: (p) => ({ text: String(p.roll), tone: p.success ? "pass" : "fail" }),
  rolled: () => true,
  subtitle: (p, w) => skillName(w.t, p.id),
  short: (p, { t }, _e, value) => ({
    value,
    what: skillName(t, p.id),
    verdict: p.success
      ? { text: t("lastShort.verdict.success"), tone: "pass" }
      : { text: t("lastShort.verdict.failure"), tone: "fail" },
  }),
  card: ({ payload, interactive }) => (
    <SkillCard check={payload} interactive={interactive} />
  ),
  live: (r) => r,
};

/** Skill check (ADR 0014): a proficiency against its ability and modifier, a bard skill against its percentage. */
function SkillCard({
  check: c,
  interactive,
}: {
  check: SkillCheck;
  interactive: boolean;
}) {
  const { t } = useTranslation();
  const proficiency = c.kind === "proficiency";
  return (
    <CheckView
      testId="skill-result"
      die={c.die}
      roll={c.roll}
      target={c.target}
      adjustment={c.adjustment ?? 0}
      adjustmentReason={c.adjustmentReason}
      adjustedTarget={checkAdjustedTarget(c)}
      success={c.success}
      percent={!proficiency}
      heading={
        proficiency
          ? t("result.skill.proficiency", {
              die: c.die,
              stat: abilityText(
                t,
                c.ability ?? "",
                c.abilityScore ?? 0,
                c.modifier ?? 0,
              ),
            })
          : t(
              c.kind === "thiefSkill"
                ? "result.skill.thiefSkill"
                : "result.skill.bardSkill",
              { die: c.die, percent: c.target },
            )
      }
      adjust={
        interactive &&
        c.logEntryId !== undefined && (
          // Keyed by entry: a new check starts its fold afresh (AdjustCheck).
          <AdjustSkill key={c.logEntryId} check={c} entryId={c.logEntryId} />
        )
      }
    />
  );
}

/**
 * The adjustment of the latest skill check (ADR 0014 step 6): the shared check adjustment, by 1 on
 * a proficiency, 5 points on a bard skill. The server recomputes the verdict; the roll never changes.
 */
function AdjustSkill({
  check: c,
  entryId,
}: {
  check: SkillCheck;
  entryId: number;
}) {
  const { t } = useTranslation();
  const adjustLastSkill = useRollStore((s) => s.adjustLastSkill);
  const max = maxSkillAdjustment(c.kind);
  return (
    <AdjustCheck
      current={{
        adjustment: c.adjustment ?? 0,
        adjustmentReason: c.adjustmentReason,
      }}
      label={t("result.skill.adjustLabel")}
      entryId={entryId}
      valueTestId="skill-adjustment"
      min={-max}
      max={max}
      step={(v, by) => stepSkillAdjustment(c.kind, v, by)}
      onApply={(value, reason) => adjustLastSkill(entryId, value, reason)}
    />
  );
}
