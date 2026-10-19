// The HP change entry (ADR 0007): the ledger's delta and the player's source; a heal cut by max HP
// or damage cut by the floor carries what was asked and applied (ADR 0011).
import { Heart } from "@phosphor-icons/react";
import { formatSigned } from "@/lib/format";
import type { HpBreakdown, LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { InfoBody } from "../parts";

/** An HP change's payload: what was asked and applied, when the server cut it. */
export type HpChangePayload = Partial<HpBreakdown>;

/** The server's breakdown when an HP change was cut (a heal by max HP, damage by the floor), else null. */
function cut(p: HpChangePayload, e: LogEntry): HpBreakdown | null {
  if (e.status === "voided") return null;
  if (typeof p.hpRequested !== "number" || typeof p.hpApplied !== "number")
    return null;
  return p.hpRequested !== p.hpApplied ? (p as HpBreakdown) : null;
}

/** The delta as the journal and the card show it: a voided change keeps its original delta. */
function amount(e: LogEntry): number {
  return e.status === "voided" ? e.hpDelta : e.effectiveHpDelta;
}

export const hpChangeKind: KindView<HpChangePayload> = {
  // The payload in effect; an HP change logged without one still reads (its delta is the entry's).
  decode: (e) => (e.effectivePayload ?? {}) as HpChangePayload,
  icon: Heart,
  // The player's source is the text; an HP change logged without one is worded from its delta.
  text: (_p, { t }, e) =>
    e.effectiveSummary ||
    t(e.hpDelta < 0 ? "log.hpDamage" : "log.hpHeal", {
      amount: Math.abs(e.hpDelta),
    }),
  value(_p, e) {
    const a = amount(e);
    return { text: formatSigned(a), tone: a < 0 ? "damage" : "heal" };
  },
  short: (_p, { t }, e, value) => ({
    value,
    what: t(e.hpDelta < 0 ? "lastShort.damage" : "lastShort.heal"),
    verdict: null,
  }),
  capped: cut,
  card: ({ payload, entry, text }) =>
    entry && (
      <InfoBody
        text={text}
        value={{
          amount: amount(entry),
          tone: amount(entry) < 0 ? "damage" : "heal",
        }}
        capped={cut(payload, entry)}
      />
    ),
};
