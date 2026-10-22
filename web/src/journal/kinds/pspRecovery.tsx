// PSPs recovered over hours of an activity (ADR 0043 step 6): hours times Table 6's hourly rate, as the
// ledger's delta; a gain cut by the maximum carries what was asked and applied.
import { Brain } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { formatSigned } from "@/lib/format";
import type { LogEntry, PspBreakdown } from "@/types/log";
import type { KindView } from "../kindView";
import { InfoBody } from "../parts";
import { cut } from "./pspChange";

/** A recovery's payload: the hours, Table 6's activity and its hourly rate, and the cut, if any. */
export type PspRecoveryPayload = {
  hours: number;
  activity: string;
  perHour: number;
} & Partial<PspBreakdown>;

const amount = (e: LogEntry): number => e.pspDelta ?? 0;

export const pspRecoveryKind: KindView<PspRecoveryPayload> = {
  decode: (e) => (e.effectivePayload ?? e.payload) as PspRecoveryPayload,
  icon: Brain,
  text: (p, { t }, e) =>
    t("log.pspRecovery", {
      hours: p.hours,
      activity: t(`psp.activity.${p.activity}`, { defaultValue: p.activity }),
      amount: amount(e),
    }),
  value(_p, e) {
    return { text: formatSigned(amount(e)), tone: "heal" };
  },
  short: (_p, { t }, _e, value) => ({
    value,
    what: t("lastShort.pspRecovered"),
    verdict: null,
  }),
  card: ({ payload, entry, text }) =>
    entry && (
      <PspRecoveryCard text={text} entry={entry} cutBy={cut(payload, entry)} />
    ),
};

function PspRecoveryCard({
  text,
  entry,
  cutBy,
}: {
  text: string;
  entry: LogEntry;
  cutBy: PspBreakdown | null;
}) {
  const { t } = useTranslation();
  return (
    <InfoBody
      text={text}
      value={{ amount: amount(entry), tone: "heal" }}
      note={
        cutBy &&
        t("log.pspCapped", { requested: formatSigned(cutBy.pspRequested) })
      }
    />
  );
}
