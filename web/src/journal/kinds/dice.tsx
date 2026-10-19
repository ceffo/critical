// The free dice roll entry (dicesvc rollRecord): not a roll kind, but the header's last result
// shows it (ADR 0022 step 2.2) and its journal row offers Reroll (step 5.3).
import { useTranslation } from "react-i18next";
import { DiceFive } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useDiceStore } from "@/stores/dice";
import { formatSigned } from "@/lib/format";
import { dieName } from "@/lib/diceFormula";
import { cn } from "@/lib/utils";
import type { DiceRoll } from "@/types/dice";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { DieChip, Total, labelClass } from "../parts";

/** The free roll as recorded, with its entry id. */
export function decodeDice(e: LogEntry): DiceRoll | null {
  const p = (e.effectivePayload ?? e.payload) as Partial<DiceRoll> | null;
  return p &&
    typeof p.formula === "string" &&
    typeof p.total === "number" &&
    typeof p.modifier === "number" &&
    Array.isArray(p.groups)
    ? { ...(p as DiceRoll), logEntryId: e.id }
    : null;
}

export const diceKind: KindView<DiceRoll> = {
  decode: decodeDice,
  icon: DiceFive,
  text: (p, { t }) => t("log.dice", { formula: p.formula, total: p.total }),
  value: (p) => ({ text: String(p.total), tone: "ink" }),
  rolled: () => true,
  subtitle: (p) => p.formula,
  short: (p, _w, _e, value) => ({ value, what: p.formula, verdict: null }),
  card: ({ payload }) => <DiceCard roll={payload} />,
  reroll: (p) => p.formula,
  lastResult: true,
};

/** A free dice roll: the formula, the dice per kind as chips with their subtotal, the constant and the big total. */
function DiceCard({ roll: r }: { roll: DiceRoll }) {
  const { t } = useTranslation();
  const reroll = useDiceStore((s) => s.roll);
  const rolling = useDiceStore((s) => s.rolling);
  return (
    <div className="flex flex-col gap-3" data-testid="dice-result">
      <div className="flex items-end justify-between gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <span
            className="text-body font-medium tabular-nums"
            data-testid="dice-formula"
          >
            {r.formula}
          </span>
          {r.groups.map((g, i) => (
            <div
              key={i}
              className="flex flex-wrap items-center gap-2"
              data-testid="dice-group"
            >
              <span className={cn(labelClass, "tabular-nums")}>
                {g.negative ? "−" : ""}
                {dieName(g.sides)}
              </span>
              <ul className="flex flex-wrap gap-2">
                {g.rolls.map((v, j) => (
                  <DieChip
                    key={j}
                    value={v}
                    faces={g.negative ? null : g.sides}
                  />
                ))}
              </ul>
              <span
                className="text-body font-semibold tabular-nums"
                data-testid="dice-subtotal"
              >
                = {formatSigned(g.subtotal)}
              </span>
            </div>
          ))}
          {r.modifier !== 0 && (
            <div className="text-body tabular-nums" data-testid="dice-modifier">
              <span className={labelClass}>{t("dice.modifier")} </span>
              {formatSigned(r.modifier)}
            </div>
          )}
        </div>
        <Total label={t("dice.total")} value={r.total} testId="dice-total" />
      </div>
      <div>
        <Button
          size="xs"
          variant="ghost"
          disabled={rolling}
          title={t("dice.rerollTitle", { formula: r.formula })}
          onClick={() => void reroll(r.formula)}
        >
          {t("dice.reroll")}
        </Button>
      </div>
    </div>
  );
}
