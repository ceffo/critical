// The initiative entries (buffsvc, ADR 0013 as amended by ADR 0035 item 6). `initiative`: the round's
// d10 and the modifiers added to it (the weapon in hand's speed, the initiative effects), logged by
// Next round or a reroll; entries from before the modifiers carry the d10 alone, their total. And
// `initiative_correct`: the round's initiative corrected by a spell cast or an item used, the same
// d10, no die rolled. Neither is a roll kind: they leave the result card alone.
import { useTranslation } from "react-i18next";
import { Lightning, Timer } from "@phosphor-icons/react";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import { initiativeBreakdown, initiativeTermText } from "@/lib/initiative";
import type { InitiativeModifier } from "@/types/buffs";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { roundLabel } from "../rounds";
import { DieChip, Total, labelClass } from "../parts";

export interface InitiativePayload {
  round: number;
  d10: number;
  /** The modifiers added to the die (absent: none, and every entry from before them). */
  modifiers: InitiativeModifier[];
  /** The initiative: the die plus the modifiers. */
  total: number;
}

export function decodeInitiative(e: LogEntry): InitiativePayload | null {
  const p = e.payload as Partial<InitiativePayload> | null;
  if (!p || typeof p.round !== "number" || typeof p.d10 !== "number")
    return null;
  return {
    round: p.round,
    d10: p.d10,
    modifiers: Array.isArray(p.modifiers) ? p.modifiers : [],
    total: typeof p.total === "number" ? p.total : p.d10,
  };
}

export const initiativeKind: KindView<InitiativePayload> = {
  decode: decodeInitiative,
  icon: Lightning,
  text: (p, { t, spellName, names }) =>
    p.modifiers.length === 0
      ? t("log.initiative", { value: p.total, round: p.round })
      : t("log.initiativeWith", {
          value: p.total,
          round: p.round,
          breakdown: initiativeBreakdown(
            p.d10,
            p.modifiers,
            t,
            spellName,
            names,
          ),
        }),
  value: (p) => ({ text: String(p.total), tone: "ink" }),
  rolled: () => true,
  subtitle: (p, { t }) => roundLabel(t, p.round),
  short: (_p, { t }, _e, value) => ({
    value,
    what: t("lastShort.initiative"),
    verdict: null,
  }),
  card: ({ payload }) => (
    <InitiativeCard
      d10={payload.d10}
      modifiers={payload.modifiers}
      total={payload.total}
    />
  ),
};

/** The payload of an initiative correction (buffsvc initiativeCorrectPayload). */
export interface InitiativeCorrectPayload {
  round: number;
  d10: number;
  before: number;
  after: number;
  modifiers: InitiativeModifier[];
  /** The spell cast or the item used, one of the modifiers. */
  cause: InitiativeModifier;
}

function decodeCorrect(e: LogEntry): InitiativeCorrectPayload | null {
  const p = e.payload as Partial<InitiativeCorrectPayload> | null;
  return p &&
    typeof p.round === "number" &&
    typeof p.d10 === "number" &&
    typeof p.before === "number" &&
    typeof p.after === "number" &&
    Array.isArray(p.modifiers) &&
    p.cause &&
    typeof p.cause.source === "string"
    ? (p as InitiativeCorrectPayload)
    : null;
}

export const initiativeCorrectKind: KindView<InitiativeCorrectPayload> = {
  decode: decodeCorrect,
  icon: Timer,
  text: (p, { t, spellName, names }) =>
    t("log.initiativeCorrect", {
      round: p.round,
      before: p.before,
      after: p.after,
      cause: initiativeTermText(p.cause, t, spellName, names),
    }),
  value: (p) => ({ text: String(p.after), tone: "ink" }),
  subtitle: (p, { t }) => roundLabel(t, p.round),
  card: ({ payload }) => (
    <InitiativeCard
      d10={payload.d10}
      modifiers={payload.modifiers}
      total={payload.after}
    />
  ),
};

/** The die, each modifier on its own line, and the total (ADR 0035 item 6.3). */
function InitiativeCard({
  d10,
  modifiers,
  total,
}: {
  d10: number;
  modifiers: readonly InitiativeModifier[];
  total: number;
}) {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
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
        {modifiers.length > 0 && (
          <ul
            className="flex flex-col text-body"
            data-testid="initiative-terms"
          >
            {modifiers.map((m, i) => (
              <li key={i}>{initiativeTermText(m, t, spellName, names)}</li>
            ))}
          </ul>
        )}
      </div>
      <Total
        label={t("round.initiative")}
        value={total}
        testId="initiative-total"
      />
    </div>
  );
}
