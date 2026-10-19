// The turning undead entry (turnsvc record, ADR 0030 step 16): a roll kind that changes no state.
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Skull } from "@phosphor-icons/react";
import type { TurnResult } from "@/types/character";
import type { LogEntry } from "@/types/log";
import type { KindView, ShortVerdict } from "../kindView";
import { undeadName } from "../words";
import { HexDie, ResultLine, Verdict } from "../parts";

export function decodeTurnUndead(e: LogEntry): TurnResult | null {
  const p = e.payload as Partial<TurnResult> | null;
  return p && typeof p.undead === "string" && typeof p.success === "boolean"
    ? (p as TurnResult)
    : null;
}

/** "Turning undead (Ghoul): 7 turned, d20 12 vs 10": the type, the result and the d20 if rolled. */
function turnUndeadText(p: TurnResult, t: TFunction): string {
  const undead = undeadName(t, p.undead);
  const result = !p.success
    ? t("log.turn.failed")
    : p.destroyed > 0 && p.turned > 0
      ? t("log.turn.destroyedTurned", {
          destroyed: p.destroyed,
          turned: p.turned,
        })
      : p.destroyed > 0
        ? t("log.turn.destroyed", { count: p.destroyed })
        : t("log.turn.turned", { count: p.turned });
  const roll =
    p.d20 !== undefined && p.target !== undefined
      ? t("log.turn.roll", { d20: p.d20, target: p.target })
      : t("log.turn.noRoll");
  return t("log.turn.text", { undead, result, roll });
}

function shortVerdict(p: TurnResult, t: TFunction): ShortVerdict {
  if (!p.success) return { text: t("lastShort.verdict.failure"), tone: "fail" };
  const key =
    p.destroyed > 0 && p.turned > 0
      ? "destroyedTurned"
      : p.destroyed > 0
        ? "destroyed"
        : "turned";
  return {
    text: t(`lastShort.verdict.${key}`, { turned: p.turned }),
    tone: "pass",
  };
}

export const turnUndeadKind: KindView<TurnResult> = {
  decode: decodeTurnUndead,
  icon: Skull,
  text: (p, w) => turnUndeadText(p, w.t),
  value: (p) =>
    p.d20 !== undefined
      ? { text: String(p.d20), tone: p.success ? "pass" : "fail" }
      : { text: String(p.destroyed || p.turned), tone: "ink" },
  rolled: () => true,
  subtitle: (p, { t }) => undeadName(t, p.undead),
  short: (p, { t }, _e, value) => ({
    value,
    what: t("lastShort.turn", { undead: undeadName(t, p.undead) }),
    verdict: shortVerdict(p, t),
  }),
  card: ({ payload }) => <TurnUndeadCard turn={payload} />,
};

/**
 * Turning undead (ADR 0030 step 16): the cleric's level, the d20 against the cell's target (or the
 * cell's outcome when no die is rolled), the verdict and the dice of how many, all as the server
 * recorded them.
 */
function TurnUndeadCard({ turn: r }: { turn: TurnResult }) {
  const { t } = useTranslation();
  const verdictText = r.success
    ? t(r.effect === "destroy" ? "result.turn.destroyed" : "result.turn.turned")
    : t("result.turn.failed");
  return (
    <div className="flex flex-col gap-3" data-testid="turn-result">
      <div className="text-body text-muted-foreground">
        {t("result.turn.heading", { level: r.clericLevel })}
      </div>
      {r.d20 !== undefined && r.target !== undefined ? (
        <div className="flex items-center gap-3">
          <HexDie value={r.d20} label={t("journal.d20", { value: r.d20 })} />
          <ResultLine
            verdict={
              <Verdict ok={r.success} yes={verdictText} no={verdictText} />
            }
          >
            <span className="text-body">
              {t("result.turn.target", { target: r.target })}
            </span>
          </ResultLine>
        </div>
      ) : (
        <ResultLine
          verdict={
            <Verdict ok={r.success} yes={verdictText} no={verdictText} />
          }
        >
          <span className="text-body">
            {t("result.turn.auto", {
              outcome: t(`turning.outcome.${r.outcome ?? "cannot"}`),
            })}
          </span>
        </ResultLine>
      )}
      {r.destroyed > 0 && (
        <p className="text-body font-semibold" data-testid="turn-destroyed">
          {t("result.turn.destroyedCount", {
            count: r.destroyed,
            dice: (r.affected ?? []).join(" + "),
          })}
        </p>
      )}
      {r.turned > 0 && (
        <p className="text-body font-semibold" data-testid="turn-turned">
          {t("result.turn.turnedCount", {
            count: r.turned,
            dice: (r.effect === "destroy"
              ? (r.more ?? [])
              : (r.affected ?? [])
            ).join(" + "),
          })}
        </p>
      )}
    </div>
  );
}
