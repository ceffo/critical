import { useTranslation } from "react-i18next";
import { codeMessage } from "@/api/http";
import type { AttackScore } from "@/api/psionics";
import { formatSigned } from "@/lib/format";
import type { AttackScores } from "./useAttackScores";

/**
 * An attack row's score to beat (ADR 0047 item 1): the figure the server's preview gives for the
 * defence the Target band names, with Table 14's cell beside it when there is one. Nothing is
 * computed here.
 */
export function ScoreToBeat({
  scores,
  power,
}: {
  scores: AttackScores;
  power: string;
}) {
  const { t } = useTranslation();
  const row = scores.status === "ready" ? scores.byPower.get(power) : undefined;
  if (row?.refusal) return null;
  return (
    <span
      className="flex min-w-14 flex-col items-end"
      data-testid="attack-score"
    >
      <span className="text-label text-ink-muted">{t("powers.tab.score")}</span>
      <span className="text-body font-semibold whitespace-nowrap tabular-nums">
        {figure(scores, row, t("powers.tab.scoreUnknown"))}
      </span>
    </span>
  );
}

function figure(
  scores: AttackScores,
  row: AttackScore | undefined,
  dash: string,
) {
  if (scores.status === "loading") return "…";
  if (!row || row.target === undefined) return dash;
  const cell = row.attackVersusDefence;
  return (
    <span data-testid="attack-score-target" data-target={row.target}>
      {row.target}
      {cell !== undefined && cell !== 0 && row.score !== undefined && (
        <span className="font-normal text-ink-muted">
          {" "}
          ({row.score}
          {formatSigned(cell)})
        </span>
      )}
    </span>
  );
}

/** Why the attack would refuse this mode, in the player's language (the error code's own wording). */
export function ScoreRefusal({
  scores,
  power,
}: {
  scores: AttackScores;
  power: string;
}) {
  const refusal =
    scores.status === "ready" ? scores.byPower.get(power)?.refusal : undefined;
  if (!refusal) return null;
  return (
    <span
      role="note"
      className="basis-full text-label text-ink-muted"
      data-testid="attack-score-refusal"
      data-code={refusal.code}
    >
      {codeMessage(refusal.code) ?? refusal.error}
    </span>
  );
}
