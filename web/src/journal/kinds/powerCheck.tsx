// A psionicist's power check (psionicsvc, ADR 0043 steps 10 and 11): the d20 against the power's
// target and the PSPs it spent, as the entry's PSP delta. The power is named by the name the entry
// recorded, in the player's language (its id for an entry with none).
import { Brain } from "@phosphor-icons/react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { useInventoryName } from "@/i18n/inventoryName";
import { formatSigned } from "@/lib/format";
import type { LocalizedText } from "@/types/localized";
import type { LogEntry } from "@/types/log";
import type { KindView, ShortVerdict } from "../kindView";
import { Fact, HexDie, ResultLine, Verdict } from "../parts";

/** The check's breakdown and outcome, as logged (psionicsvc CheckRecord). */
export interface PowerCheckPayload {
  power: string;
  /** The power's catalogue name when logged, in every language. */
  name?: LocalizedText;
  ability?: string;
  abilityScore?: number;
  powerModifier?: number;
  enteredScore?: number;
  relearns?: number;
  modifiers?: { value: number; reason?: string }[];
  target: number;
  d20: number;
  outcome: "success" | "failure" | "power_score" | "natural_20";
  success: boolean;
  cost: number;
  spent: number;
  /** A power kept up: what each charge costs and how many rounds apart (logged on a success only). */
  maintenanceCost?: number;
  maintenancePeriod?: number;
  /** Warnings the check raised, by code (second_power_this_round). */
  warnings?: string[];
  /** The mind the check was on: the contact power's target, or the contacted mind of a power paid on contact. */
  opponent?: string;
  /** The contact power's target's level (Table 29's row). */
  level?: number;
  /** The contact power's score lines (Tables 30 and 31, resisting), by code. */
  contactModifiers?: { kind: string; code: string; modifier: number }[];
  /** The contact power's success made (or renewed) the opponent's contact row. */
  contact?: boolean;
}

function verdict(p: PowerCheckPayload, t: TFunction): ShortVerdict {
  switch (p.outcome) {
    case "power_score":
      return { text: t("lastShort.verdict.powerScore"), tone: "crit" };
    case "natural_20":
      return { text: t("lastShort.verdict.natural20"), tone: "fail" };
    case "success":
      return { text: t("lastShort.verdict.success"), tone: "pass" };
    default:
      return { text: t("lastShort.verdict.failure"), tone: "fail" };
  }
}

const spent = (e: LogEntry): number => e.pspDelta ?? 0;

/**
 * How the target came about, as recorded: the ability and the power's modifier (or the score the
 * player entered), the relearns, each labelled modifier, then the cost paid when it is not the
 * full one, the maintenance a success set up and the warnings.
 */
export function breakdown(p: PowerCheckPayload, t: TFunction): string[] {
  const parts: string[] = [];
  if (p.enteredScore !== undefined) {
    parts.push(t("log.powerParts.entered", { score: p.enteredScore }));
  } else if (p.ability) {
    parts.push(
      t("log.powerParts.ability", {
        ability: t(`attr.${p.ability}`, { defaultValue: p.ability }),
        score: p.abilityScore ?? 0,
        modifier: formatSigned(p.powerModifier ?? 0),
      }),
    );
  }
  if (p.relearns)
    parts.push(t("log.powerParts.relearns", { count: p.relearns }));
  if (p.opponent) {
    parts.push(
      p.level
        ? `${t("log.powerParts.opponent", { target: p.opponent })}, ${t("log.powerParts.targetLevel", { level: p.level })}`
        : t("log.powerParts.opponent", { target: p.opponent }),
    );
  }
  for (const c of p.contactModifiers ?? []) {
    parts.push(
      t("log.powerParts.contactLine", {
        what: t(`log.contactWords.${c.kind}.${c.code}`, {
          defaultValue: c.code,
        }),
        modifier: formatSigned(c.modifier),
      }),
    );
  }
  for (const m of p.modifiers ?? []) {
    parts.push(
      m.reason
        ? t("log.powerParts.modifierWhy", {
            value: formatSigned(m.value),
            reason: m.reason,
          })
        : formatSigned(m.value),
    );
  }
  if (p.spent !== p.cost) {
    parts.push(t("log.powerParts.halfCost", { spent: p.spent, cost: p.cost }));
  }
  if (p.success && p.maintenanceCost) {
    parts.push(
      t("log.powerParts.maintained", {
        cost: p.maintenanceCost,
        count: p.maintenancePeriod ?? 1,
      }),
    );
  }
  if (p.contact) parts.push(t("log.powerParts.contactMade"));
  for (const w of p.warnings ?? []) {
    parts.push(t(`log.powerParts.warning.${w}`, { defaultValue: w }));
  }
  return parts;
}

export const powerCheckKind: KindView<PowerCheckPayload> = {
  decode(e) {
    const p = e.payload as Partial<PowerCheckPayload> | null;
    return p &&
      typeof p.power === "string" &&
      typeof p.d20 === "number" &&
      typeof p.outcome === "string"
      ? (p as PowerCheckPayload)
      : null;
  },
  icon: Brain,
  // "Power check (Mind Thrust): d20 9 vs 13, success, PSPs −7 (INT 18 −4, +1 relearned, −2 the DM)".
  text: (p, { t, names }, e) => {
    const parts = breakdown(p, t);
    const line = t("log.powerCheck", {
      power: names.text(p.name, p.power),
      d20: p.d20,
      target: p.target,
      verdict: verdict(p, t).text,
      spent: formatSigned(spent(e)),
    });
    return parts.length > 0
      ? t("log.powerCheckWith", { text: line, parts: parts.join(", ") })
      : line;
  },
  // "Mind Thrust: success, PSPs −7".
  summary: (p, { t, names }, e) =>
    t("log.summary.powerCheck", {
      power: names.text(p.name, p.power),
      verdict: verdict(p, t).text,
      spent: formatSigned(spent(e)),
    }),
  detail: (p, { t }, e) => [
    t("log.summary.powerRoll", {
      d20: p.d20,
      target: p.target,
      verdict: verdict(p, t).text,
      spent: formatSigned(spent(e)),
    }),
    ...breakdown(p, t),
  ],
  value: (p) => ({ text: String(p.d20), tone: p.success ? "pass" : "fail" }),
  rolled: () => true,
  subtitle: (p, { names }) => names.text(p.name, p.power),
  short: (p, { t }, _e, value) => ({
    value,
    what: t("log.kinds.power_check"),
    verdict: verdict(p, t),
  }),
  card: ({ payload, entry }) => (
    <PowerCheckCard p={payload} spent={entry ? spent(entry) : undefined} />
  ),
};

/**
 * A power check as a card (ADR 0043 steps 10 and 11): the d20 in its hexagon, the target it was
 * rolled against and the verdict beside it, the PSPs spent as a tile, then how the target came
 * about. Every figure is the check's record; the PSPs spent are the entry's PSP delta, or the
 * payload's `spent` while the entry is not in the log yet.
 */
function PowerCheckCard({
  p,
  spent: delta,
}: {
  p: PowerCheckPayload;
  spent?: number;
}) {
  const { t } = useTranslation();
  const names = useInventoryName();
  const paid = delta !== undefined ? Math.abs(delta) : p.spent;
  const word = t(`powers.use.outcome.${p.outcome}`);
  const lines = breakdown(p, t);
  return (
    <div className="flex flex-col gap-3" data-testid="power-check-result">
      <div className="flex items-center gap-3">
        <HexDie value={p.d20} label={t("journal.d20", { value: p.d20 })} />
        <div className="min-w-0 flex-1">
          <ResultLine verdict={<Verdict ok={p.success} yes={word} no={word} />}>
            <span
              className="text-key tabular-nums"
              data-testid="power-check-target"
            >
              {t("result.psi.target", { target: p.target })}
            </span>
          </ResultLine>
          <div className="text-body text-muted-foreground">
            {names.text(p.name, p.power)}
          </div>
        </div>
      </div>
      <dl
        className="grid grid-cols-[repeat(auto-fit,minmax(5rem,1fr))] gap-2"
        data-testid="power-check-tiles"
      >
        <Fact label={t("result.d20")}>{p.d20}</Fact>
        <Fact label={t("result.target")}>{p.target}</Fact>
        <Fact label={t("result.psi.cost")}>
          <span data-testid="power-check-cost">
            {paid === p.cost
              ? paid
              : t("result.psi.paidOf", { spent: paid, cost: p.cost })}
          </span>
        </Fact>
      </dl>
      {lines.length > 0 && (
        <ul
          className="flex flex-col gap-0.5 text-label text-muted-foreground"
          data-testid="power-check-parts"
        >
          {lines.map((l) => (
            <li key={l} className="break-words">
              {l}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
