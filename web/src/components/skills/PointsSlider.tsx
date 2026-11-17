import type { KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

/** The thumb's width in px: the native thumb travels the track less one thumb, so the fills end on
 * its centre. */
const THUMB = 16;
const at = (ratio: number) =>
  `calc(${ratio} * (100% - ${THUMB}px) + ${THUMB / 2}px)`;

/**
 * A skill's points as a slider with a typed box beside it (ADR 0027 step 1: balancing is done every
 * level, so it gets a control to drag). The solid part is the skill's points, the light part how far
 * the points left take it, "/ N" the skill's limit. A native range input, so the arrows move it by 1,
 * Home and End to the ends, and PageUp and PageDown by 5; its name and its value ("20 of 30") are read
 * aloud. A move past what is left stops at the reach. The box takes any text, the server judges it.
 */
export function PointsSlider({
  id,
  scope = "thief",
  name,
  value,
  limit,
  reach,
  invalid,
  describedBy,
  onChange,
}: {
  id: string;
  /** The skill set, so two sets sharing a skill keep distinct element ids. */
  scope?: string;
  /** The skill's name, for the accessible name. */
  name: string;
  /** What the box holds, as typed. */
  value: string;
  /** The skill's limit: the end of the slider. */
  limit: number;
  /** The most the slider goes to with the points left (the skill's points plus what is left). */
  reach: number;
  invalid?: boolean;
  describedBy?: string;
  onChange: (value: string) => void;
}) {
  const { t } = useTranslation();
  const typed = Number(value.trim() === "" ? 0 : value);
  const points = Number.isInteger(typed) && typed >= 0 ? typed : 0;
  const scale = Math.max(limit, 1);
  const set = (n: number) =>
    onChange(String(Math.max(0, Math.min(n, limit, Math.max(reach, 0)))));
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== "PageUp" && e.key !== "PageDown") return;
    e.preventDefault();
    set(points + (e.key === "PageUp" ? 5 : -5));
  };
  return (
    <span className="flex min-w-0 items-center gap-2">
      <span
        className="relative block h-7 min-w-28 flex-1 rounded-md has-[input:focus-visible]:ring-2 has-[input:focus-visible]:ring-ring"
        data-testid={`points-slider-${id}`}
      >
        <span className="absolute inset-x-0 top-3 h-1.5 rounded-full bg-input" />
        <span
          className="absolute top-3 left-0 h-1.5 rounded-full bg-tint/35"
          style={{ width: at(Math.min(reach, limit) / scale) }}
        />
        <span
          className="absolute top-3 left-0 h-1.5 rounded-full bg-tint"
          style={{ width: at(Math.min(points, limit) / scale) }}
        />
        <input
          type="range"
          min={0}
          max={limit}
          step={1}
          value={Math.min(points, limit)}
          disabled={limit === 0}
          aria-label={t("skills.pointsSlider", { name })}
          aria-valuetext={t("skills.pointsOf", { points, cap: limit })}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          data-testid={`thief-points-slider-${id}`}
          onChange={(e) => set(Number(e.target.value))}
          onKeyDown={onKey}
          className={cn(
            "absolute inset-0 m-0 h-7 w-full cursor-pointer appearance-none bg-transparent focus:outline-none disabled:cursor-default",
            "[&::-webkit-slider-runnable-track]:h-7 [&::-webkit-slider-runnable-track]:bg-transparent",
            "[&::-webkit-slider-thumb]:mt-1.5 [&::-webkit-slider-thumb]:size-4 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:border-2 [&::-webkit-slider-thumb]:border-tint [&::-webkit-slider-thumb]:bg-surface",
            "[&::-moz-range-track]:bg-transparent [&::-moz-range-thumb]:size-3 [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:border-2 [&::-moz-range-thumb]:border-tint [&::-moz-range-thumb]:bg-surface",
          )}
        />
      </span>
      <input
        id={`${scope}-points-${id}`}
        inputMode="numeric"
        autoComplete="off"
        className={cn(
          "h-7 w-12 shrink-0 rounded-md border bg-transparent text-center text-body tabular-nums",
          invalid ? "border-danger" : "border-input",
        )}
        aria-label={t("skills.pointsLabel", { name })}
        aria-invalid={invalid || undefined}
        aria-describedby={describedBy}
        data-testid={`thief-points-${id}`}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <span className="w-10 shrink-0 text-label text-ink-muted tabular-nums">
        / {limit}
      </span>
    </span>
  );
}
