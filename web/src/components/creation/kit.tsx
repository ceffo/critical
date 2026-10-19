import type { ReactNode } from "react";
import { Check, Prohibit, Warning } from "@phosphor-icons/react";
import { BrandIcon } from "@/components/shell/BrandMark";
import { labelClass } from "@/components/dashboard/fieldStyles";
import { APP_NAME } from "@/lib/brand";
import { cn } from "@/lib/utils";

/** The brand: the icon and the name, its last character highlighted. */
export function Wordmark({ size = "key" }: { size?: "key" | "hero" }) {
  return (
    <div
      className="flex shrink-0 items-center gap-2"
      role="img"
      aria-label={APP_NAME}
    >
      <BrandIcon className={size === "hero" ? "size-12" : "size-7"} />
      <span
        aria-hidden
        className={cn(
          "font-display leading-none font-bold tracking-wide select-none",
          size === "hero" ? "text-hero" : "text-key",
        )}
      >
        {APP_NAME.slice(0, -1)}
        <span className="text-highlight">{APP_NAME.slice(-1)}</span>
      </span>
    </div>
  );
}

/** A note under a control: a warning (amber, never blocks), a refusal (red, blocks) or a hint. */
export function Note({
  kind,
  children,
  testId,
}: {
  kind: "warning" | "refusal" | "info";
  children: ReactNode;
  testId?: string;
}) {
  const Icon =
    kind === "refusal" ? Prohibit : kind === "warning" ? Warning : Check;
  return (
    <p
      role={kind === "refusal" ? "alert" : undefined}
      data-testid={testId}
      className={cn(
        "flex items-start gap-2 rounded-md px-3 py-2 text-body",
        kind === "refusal" &&
          "bg-raised text-danger shadow-[inset_3px_0_0_var(--danger)]",
        kind === "warning" &&
          "bg-raised text-warning shadow-[inset_3px_0_0_var(--warning)]",
        kind === "info" && "bg-raised text-ink-muted",
      )}
    >
      <Icon aria-hidden weight="bold" className="mt-0.5 size-4 shrink-0" />
      <span className="min-w-0 break-words">{children}</span>
    </p>
  );
}

/** A segmented choice: one of a few options, as radios. */
export function Seg<T extends string>({
  label,
  options,
  value,
  onChange,
  testId,
}: {
  label: string;
  options: { value: T; content: ReactNode; disabled?: boolean }[];
  value: T;
  onChange: (v: T) => void;
  testId?: string;
}) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className="flex flex-wrap gap-1"
      data-testid={testId}
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={o.value === value}
          disabled={o.disabled}
          onClick={() => onChange(o.value)}
          className={cn(
            "flex min-h-7 items-center justify-center gap-1 rounded-md px-3 py-1 text-body outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:text-ink-muted [&_svg]:size-4",
            o.value === value ? "chip-chosen" : "chip hover:bg-raised-hover",
          )}
        >
          {o.content}
        </button>
      ))}
    </div>
  );
}

/** One die of a roll's breakdown; a dropped die is struck through. */
export function Die({ v, dropped }: { v: number; dropped?: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex h-6 min-w-6 items-center justify-center rounded-md px-1 text-body tabular-nums",
        dropped
          ? "text-ink-muted line-through decoration-2"
          : "bg-raised-hover font-medium",
      )}
    >
      {v}
    </span>
  );
}

/** A card head: the section label on the band. */
export function CardHead({
  children,
  right,
}: {
  children: ReactNode;
  right?: ReactNode;
}) {
  return (
    <div className="card-band flex min-h-9 shrink-0 items-center justify-between gap-3 px-4 py-2">
      <h2 className="section-label">{children}</h2>
      {right}
    </div>
  );
}

/** A group heading inside a step card, with an optional control on its right. */
export function Group({
  title,
  right,
  children,
  className,
  testId,
}: {
  title: string;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
  testId?: string;
}) {
  return (
    <section
      className={cn("flex min-w-0 flex-col gap-3", className)}
      data-testid={testId}
    >
      <div className="group-heading flex min-h-9 items-center justify-between gap-3 px-3">
        <h3 className="section-label">{title}</h3>
        {right}
      </div>
      {children}
    </section>
  );
}

/** A pickable card (class, race): the chosen tone with the tint outline; a refused one dimmed. */
export function Choice({
  on,
  off,
  onPick,
  label,
  children,
  testId,
}: {
  on: boolean;
  off?: boolean;
  onPick: () => void;
  label: string;
  children: ReactNode;
  testId?: string;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      aria-label={label}
      aria-disabled={off || undefined}
      data-testid={testId}
      onClick={() => !off && onPick()}
      className={cn(
        "flex min-w-0 flex-col gap-1 rounded-lg p-3 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
        on ? "chip-chosen" : "bg-raised",
        !on && !off && "hover:bg-raised-hover",
        off && "cursor-not-allowed opacity-60",
      )}
    >
      {children}
    </button>
  );
}

/** One fact of a class or race card: the label left, the value right of it. */
export function Row({ k, v }: { k: string; v: ReactNode }) {
  return (
    <span className="grid grid-cols-[7.5rem_minmax(0,1fr)] items-baseline gap-2">
      <span className={cn(labelClass, "break-words")}>{k}</span>
      <span className="text-body break-words">{v}</span>
    </span>
  );
}

/** A done step's check, the current step's number in the tint, a later one's muted number. */
export function StepDot({
  n,
  done,
  on,
}: {
  n: number;
  done: boolean;
  on: boolean;
}) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-6 shrink-0 items-center justify-center rounded-full text-label font-semibold",
        done && !on && "bg-tint text-on-accent",
        on && "text-tint shadow-[inset_0_0_0_1.5px_var(--tint)]",
        !done && !on && "text-ink-muted shadow-[inset_0_0_0_1px_var(--input)]",
      )}
    >
      {done && !on ? <Check weight="bold" className="size-3.5" /> : n}
    </span>
  );
}
