// The PSP change entered by hand (ADR 0043 step 5), the PSP pool's hp_change: the ledger's delta and
// the player's source; a gain cut by the maximum or a loss cut at 0 carries what was asked and applied.
import { Brain } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { formatSigned } from "@/lib/format";
import type { LogEntry, PspBreakdown } from "@/types/log";
import type { KindView } from "../kindView";
import { InfoBody } from "../parts";

/** A PSP change's payload: what was asked and applied, when the server cut it. */
export type PspChangePayload = Partial<PspBreakdown>;

/** The server's breakdown when a PSP change was cut (a gain by the maximum, a loss at 0), else null. */
function cut(p: PspChangePayload, e: LogEntry): PspBreakdown | null {
  if (e.status === "voided") return null;
  if (typeof p.pspRequested !== "number" || typeof p.pspApplied !== "number")
    return null;
  return p.pspRequested !== p.pspApplied ? (p as PspBreakdown) : null;
}

/** The delta as the journal and the card show it: entries are never amended, a voided one keeps its
 * original delta. */
const amount = (e: LogEntry): number => e.pspDelta ?? 0;

export const pspChangeKind: KindView<PspChangePayload> = {
  decode: (e) => (e.effectivePayload ?? {}) as PspChangePayload,
  icon: Brain,
  // The player's source is the text; a change logged without one is worded from its delta.
  text: (_p, { t }, e) =>
    e.effectiveSummary ||
    t(amount(e) < 0 ? "log.pspSpent" : "log.pspRecovered", {
      amount: Math.abs(amount(e)),
    }),
  value(_p, e) {
    const a = amount(e);
    return { text: formatSigned(a), tone: a < 0 ? "damage" : "heal" };
  },
  short: (_p, { t }, e, value) => ({
    value,
    what: t(amount(e) < 0 ? "lastShort.pspSpent" : "lastShort.pspRecovered"),
    verdict: null,
  }),
  card: ({ payload, entry, text }) =>
    entry && (
      <PspChangeCard text={text} entry={entry} cutBy={cut(payload, entry)} />
    ),
};

/** The card: the text, the delta and, when the server cut it, what was asked. */
function PspChangeCard({
  text,
  entry,
  cutBy,
}: {
  text: string;
  entry: LogEntry;
  cutBy: PspBreakdown | null;
}) {
  const { t } = useTranslation();
  const a = amount(entry);
  return (
    <InfoBody
      text={text}
      value={{ amount: a, tone: a < 0 ? "damage" : "heal" }}
      note={
        cutBy &&
        t(cutBy.pspRequested < 0 ? "log.pspFloored" : "log.pspCapped", {
          requested: formatSigned(cutBy.pspRequested),
        })
      }
    />
  );
}
