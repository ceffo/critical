// The initiative entry (buffsvc initiativePayload, ADR 0013): the round's d10, logged by Next round
// or a reroll. Not a roll kind: it leaves the result card alone.
import { useTranslation } from "react-i18next";
import { Lightning } from "@phosphor-icons/react";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { roundLabel } from "../rounds";
import { DieChip, Total, labelClass } from "../parts";

export interface InitiativePayload {
  round: number;
  d10: number;
}

export function decodeInitiative(e: LogEntry): InitiativePayload | null {
  const p = e.payload as Partial<InitiativePayload> | null;
  return p && typeof p.round === "number" && typeof p.d10 === "number"
    ? { round: p.round, d10: p.d10 }
    : null;
}

export const initiativeKind: KindView<InitiativePayload> = {
  decode: decodeInitiative,
  icon: Lightning,
  text: (p, { t }) => t("log.initiative", { value: p.d10, round: p.round }),
  value: (p) => ({ text: String(p.d10), tone: "ink" }),
  rolled: () => true,
  subtitle: (p, { t }) => roundLabel(t, p.round),
  short: (_p, { t }, _e, value) => ({
    value,
    what: t("lastShort.initiative"),
    verdict: null,
  }),
  card: ({ payload }) => <InitiativeCard d10={payload.d10} />,
};

function InitiativeCard({ d10 }: { d10: number }) {
  const { t } = useTranslation();
  return (
    <div
      className="flex items-end justify-between gap-3"
      data-testid="initiative-result"
    >
      <div className="flex min-w-0 flex-col gap-2">
        <span className={labelClass}>{t("round.initiativeHint")}</span>
        <ul className="flex gap-2">
          <DieChip value={d10} faces={10} />
        </ul>
      </div>
      <Total
        label={t("round.initiative")}
        value={d10}
        testId="initiative-total"
      />
    </div>
  );
}
