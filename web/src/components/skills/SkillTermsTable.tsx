import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";

/** The terms of one skill's percentage, as the server derives them (ADR 0030 steps 17 and 18). */
export interface TermsRow {
  id: string;
  base: number;
  points: number;
  race: number;
  dexterity: number;
  armor: number;
  /** The sum before the cap and the floor. */
  rawPercent: number;
  percent: number;
  blocked?: boolean;
}

/** The word under a percentage the 95 cap or the 0 floor moved: the figure it was brought to. */
export function LimitNote({ row }: { row: TermsRow }) {
  const { t } = useTranslation();
  const note =
    row.rawPercent > row.percent
      ? t("skills.capped", { max: row.percent })
      : row.rawPercent < row.percent
        ? t("skills.floored", { min: row.percent })
        : null;
  return note ? (
    <span
      className="block text-label font-normal text-ink-muted"
      data-testid={`skill-limit-${row.id}`}
    >
      {note}
    </span>
  ) : null;
}

const TERMS = [
  "base",
  "points",
  "race",
  "dexterity",
  "armor",
  "total",
] as const;

/**
 * The breakdown of a skill set, a table: one column per term (base, points, race, Dexterity,
 * armour), the total last with the cap or the floor named under it when it applies. `name` draws the
 * skill's cell, `points` the points cell (the slider in the editor; the figure when absent). Every
 * figure is the server's.
 */
export function SkillTermsTable({
  setId,
  rows,
  name,
  points,
  onRowFocus,
  className,
}: {
  setId: string;
  rows: TermsRow[];
  name: (row: TermsRow) => ReactNode;
  points?: (row: TermsRow) => ReactNode;
  /** Pointing at a row or focusing into it (the ability in play). */
  onRowFocus?: (on: boolean) => void;
  className?: string;
}) {
  const { t } = useTranslation();
  const th = "px-2 py-1 text-right text-label font-medium text-ink-muted";
  const td = "px-2 py-1.5 text-right tabular-nums";
  return (
    <div className={cn("overflow-x-auto", className)}>
      <table
        className="w-full border-separate border-spacing-y-1 text-body"
        data-testid={`skills-points-${setId}-table`}
      >
        <thead>
          <tr>
            <th className={cn(th, "text-left")}>
              {t("skillsActivity.talent")}
            </th>
            {TERMS.map((k) => (
              <th
                key={k}
                className={cn(th, points && k === "points" && "text-center")}
              >
                {t(`skillsActivity.terms.${k}`)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr
              key={r.id}
              className={cn("bg-sub-band", r.blocked && "text-warning")}
              data-testid={`skill-set-read-${setId}-${r.id}`}
              onMouseEnter={() => onRowFocus?.(true)}
              onMouseLeave={() => onRowFocus?.(false)}
              onFocus={() => onRowFocus?.(true)}
              onBlur={() => onRowFocus?.(false)}
            >
              <td className="rounded-l-md px-2.5 py-1.5 text-left align-top">
                {name(r)}
              </td>
              <td className={td}>{r.base}</td>
              <td className={cn(td, points && "w-[45%] min-w-48")}>
                {points ? points(r) : r.points}
              </td>
              <td className={td}>{formatSigned(r.race)}</td>
              <td className={td}>{formatSigned(r.dexterity)}</td>
              <td className={td}>{formatSigned(r.armor)}</td>
              <td
                className={cn(
                  td,
                  "rounded-r-md font-semibold whitespace-nowrap",
                )}
                data-testid={`${setId}-read-percent-${r.id}`}
              >
                {t("skills.percent", { value: r.percent })}
                <LimitNote row={r} />
                {r.blocked && (
                  <span className="block text-label font-normal">
                    {t("skills.blockedTag")}
                  </span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
