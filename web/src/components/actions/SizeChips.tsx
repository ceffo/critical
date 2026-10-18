import { cn } from "@/lib/utils";
import type { AttackForm } from "@/types/combat";

type Size = AttackForm["targetSize"];

const SIZES: Size[] = ["sm", "l"];

/**
 * The target size chips (S/M · L), always on the target line (ADR 0027 step 1.5): the most
 * important attack setting, one click to change. Text comes from the caller (i18n).
 */
export function SizeChips({
  value,
  label,
  names,
  shorts,
  onChange,
}: {
  value: Size;
  label: string;
  names: Record<Size, string>;
  shorts: Record<Size, string>;
  onChange: (size: Size) => void;
}) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className="flex rounded-lg bg-surface p-0.5"
    >
      {SIZES.map((s) => (
        <button
          key={s}
          type="button"
          role="radio"
          aria-checked={value === s}
          aria-label={names[s]}
          className={cn(
            "h-6 rounded-md px-3 text-body whitespace-nowrap text-muted-foreground",
            value === s && "chip-chosen",
          )}
          onClick={() => onChange(s)}
        >
          {shorts[s]}
        </button>
      ))}
    </div>
  );
}
