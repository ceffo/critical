// The ability check entry (skillsvc abilityRecord, ADR 0028): a d20 under the score in effect; the
// payload in effect is the latest adjustment's.
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Barbell } from "@phosphor-icons/react";
import { useRollStore } from "@/stores/roll";
import { formatSigned } from "@/lib/format";
import { MAX_ADJUSTMENT, stepAdjustment } from "@/lib/saveAdjust";
import type { AbilityCheck } from "@/types/skills";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { AdjustCheck, CheckView } from "../parts";

/** The ability check's payload in effect (ADR 0028 step 5), with its entry id. */
export function decodeAbilityCheck(e: LogEntry): AbilityCheck | null {
  const p = (e.effectivePayload ?? e.payload) as Partial<AbilityCheck> | null;
  return p &&
    typeof p.ability === "string" &&
    typeof p.roll === "number" &&
    typeof p.target === "number" &&
    typeof p.adjustedTarget === "number" &&
    typeof p.success === "boolean"
    ? { ...(p as AbilityCheck), logEntryId: e.id }
    : null;
}

/**
 * "DEX: d20 14 vs 13 (15 −2 ledge), failure": the ability check as the journal words it (ADR 0028
 * step 5), its adjustment and reason when set. The verdict is the payload's.
 */
function abilityCheckText(c: AbilityCheck, t: TFunction): string {
  const name = t(`attr.${c.ability}`, { defaultValue: c.ability });
  if (c.adjustment) {
    return t(
      c.success
        ? "abilityCheck.log.successAdjusted"
        : "abilityCheck.log.failureAdjusted",
      {
        name,
        die: c.die,
        roll: c.roll,
        target: c.target,
        adjustedTarget: c.adjustedTarget,
        adjustment: formatSigned(c.adjustment),
        reason: c.adjustmentReason ? ` ${c.adjustmentReason}` : "",
      },
    );
  }
  return t(
    c.success ? "abilityCheck.log.success" : "abilityCheck.log.failure",
    { name, die: c.die, roll: c.roll, target: c.target },
  );
}

export const abilityCheckKind: KindView<AbilityCheck, AbilityCheck> = {
  decode: decodeAbilityCheck,
  icon: Barbell,
  text: (p, w) => abilityCheckText(p, w.t),
  value: (p) => ({ text: String(p.roll), tone: p.success ? "pass" : "fail" }),
  rolled: () => true,
  subtitle: (p, { t }) =>
    t(`abilityCheck.names.${p.ability}`, { defaultValue: p.ability }),
  short: (p, { t }, _e, value) => ({
    value,
    what: t(`attr.${p.ability}`, { defaultValue: p.ability }),
    verdict: p.success
      ? { text: t("lastShort.verdict.success"), tone: "pass" }
      : { text: t("lastShort.verdict.failure"), tone: "fail" },
  }),
  card: ({ payload, interactive }) => (
    <AbilityCard check={payload} interactive={interactive} />
  ),
  live: (r) => r,
};

/**
 * Ability check (ADR 0028): a d20 under the score in effect; the sheet's score beside it when an
 * effect changed it. The DM's adjustment is set here, after the roll (step 6).
 */
function AbilityCard({
  check: c,
  interactive,
}: {
  check: AbilityCheck;
  interactive: boolean;
}) {
  const { t } = useTranslation();
  const name = t(`attr.${c.ability}`, { defaultValue: c.ability });
  return (
    <CheckView
      testId="ability-result"
      die={c.die}
      roll={c.roll}
      target={c.target}
      adjustment={c.adjustment}
      adjustmentReason={c.adjustmentReason}
      adjustedTarget={c.adjustedTarget}
      success={c.success}
      heading={t(
        c.target === c.abilityScore
          ? "abilityCheck.heading"
          : "abilityCheck.headingEffect",
        { die: c.die, name, score: c.target, sheet: c.abilityScore },
      )}
      adjust={
        interactive &&
        c.logEntryId !== undefined && (
          // Keyed by entry: a new check starts its fold afresh (AdjustCheck).
          <AdjustAbility key={c.logEntryId} check={c} entryId={c.logEntryId} />
        )
      }
    />
  );
}

/**
 * The adjustment of the latest ability check (ADR 0028 steps 6 and 8): the shared check
 * adjustment, ±20 by 1. The server recomputes the verdict; the roll never changes.
 */
function AdjustAbility({
  check: c,
  entryId,
}: {
  check: AbilityCheck;
  entryId: number;
}) {
  const { t } = useTranslation();
  const adjustLastAbility = useRollStore((s) => s.adjustLastAbility);
  return (
    <AdjustCheck
      current={c}
      label={t("abilityCheck.adjustLabel")}
      entryId={entryId}
      valueTestId="ability-adjustment"
      min={-MAX_ADJUSTMENT}
      max={MAX_ADJUSTMENT}
      step={stepAdjustment}
      onApply={(value, reason) => adjustLastAbility(entryId, value, reason)}
    />
  );
}
