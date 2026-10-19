// The exposure entry (castsvc.ExposureBreakdown, ADR 0008 step 16): one round of damage to whoever
// stands in a spell's area; on the Bard it carries an HP delta.
import type { TFunction } from "i18next";
import { CloudFog } from "@phosphor-icons/react";
import type { SpellNamer } from "@/i18n/spellName";
import { formatSigned } from "@/lib/format";
import { modeName } from "@/lib/spellMode";
import { ExposureResultView } from "@/components/dashboard/CastResultView";
import type { ExposureBreakdown, ExposureResult } from "@/types/spells";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";

/** Payload of exposure entries (castsvc.ExposureBreakdown, ADR 0008 step 16). */
export interface ExposureLogPayload extends ExposureBreakdown {
  /** Set by the server when the HP floor cut an exposure on the Bard: what was asked and applied. */
  hpRequested?: number;
  hpApplied?: number;
}

/** An exposure as the card shows it: the endpoint's answer shape, from the entry or the answer. */
export type ExposurePayload = ExposureResult & ExposureLogPayload;

/**
 * An exposure entry's payload as the exposure endpoint's result, so one view shows both; the HP
 * applied is the entry's (a voided one keeps its original delta, as an HP change shows it).
 */
export function decodeExposure(e: LogEntry): ExposurePayload | null {
  const p = e.payload as Partial<ExposureLogPayload> | null;
  if (!p || typeof p.spellId !== "string" || typeof p.round !== "number")
    return null;
  if (!p.damage) return null;
  return {
    ...(p as ExposureLogPayload),
    logEntryId: e.id,
    hpDelta: e.status === "voided" ? e.hpDelta : e.effectiveHpDelta,
  };
}

/** "Cloudkill exposure, round 2 (Within 10 ft): 7 fire damage, on me". */
function exposureText(
  p: ExposureLogPayload,
  t: TFunction,
  spellName: SpellNamer,
): string {
  const spell = spellName(p.spellId, p.spell);
  const text = t(p.onSelf ? "log.exposure.onMe" : "log.exposure.forDm", {
    spell,
    round: p.round,
    total: p.damage.total,
    type: p.damage.type
      ? t(`spellDice.types.${p.damage.type}`, { defaultValue: p.damage.type })
      : "",
  });
  const parts = [text];
  if (p.mode) parts.push(modeName(t, p.spellId, p.mode));
  // Cut by the HP floor (ADR 0011): the server merged what was asked and applied into the payload.
  if (
    typeof p.hpRequested === "number" &&
    typeof p.hpApplied === "number" &&
    p.hpRequested !== p.hpApplied
  )
    parts.push(t("log.floored", { requested: formatSigned(p.hpRequested) }));
  return parts.join(" · ");
}

export const exposureKind: KindView<ExposurePayload, ExposureResult> = {
  decode: decodeExposure,
  icon: CloudFog,
  text: (p, w) => exposureText(p, w.t, w.spellName),
  value: (p) =>
    p.onSelf
      ? // On the Bard: the HP the server applied (the floor may cut it), as an HP change shows it.
        { text: formatSigned(p.hpDelta), tone: "damage" }
      : { text: String(p.damage.total), tone: "ink" },
  rolled: () => true,
  subtitle: (p, w) => w.spellName(p.spellId, p.spell),
  short: (p, w, _e, value) => ({
    value,
    what: w.spellName(p.spellId, p.spell),
    verdict: null,
  }),
  card: ({ payload, interactive }) => (
    <ExposureResultView result={payload} interactive={interactive} />
  ),
  live: (r) => r,
};
