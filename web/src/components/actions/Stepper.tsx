import { useId, useState } from "react";
import { Minus, Plus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";

/** The server's bound on a situational modifier and the target AC (combatsvc MaxSituational). */
export const BOUND = 20;

/**
 * − / + around a whole number in ±BOUND (or min..max); typing is kept as text until it is a number. A null value
 * shows an empty field with the placeholder, − / + then start from emptyBase; clearing the field
 * calls onClear when given.
 */
export function Stepper({
  label,
  shortLabel,
  lower,
  raise,
  value: current,
  signed = false,
  placeholder,
  emptyBase = 0,
  max = BOUND,
  min = -BOUND,
  step = 1,
  onChange,
  onClear,
}: {
  label: string;
  /** Shown beside the field instead of `label`, which stays the field's accessible name. */
  shortLabel?: string;
  lower: string;
  raise: string;
  value: number | null;
  signed?: boolean;
  placeholder?: string;
  emptyBase?: number;
  /** Upper bound, when below BOUND (the target AC stops at the highest real AC). */
  max?: number;
  /** Lower bound, when not -BOUND. */
  min?: number;
  /** What − and + move the value by. */
  step?: number;
  onChange: (v: number) => void;
  onClear?: () => void;
}) {
  const id = useId();
  const [draft, setDraft] = useState<string | null>(null);
  const value = current ?? emptyBase;
  const shown =
    draft ??
    (current === null
      ? ""
      : signed && current > 0
        ? `+${current}`
        : String(current));
  const set = (v: number) => {
    setDraft(null);
    onChange(Math.max(min, Math.min(max, v)));
  };
  return (
    <div className="flex items-center gap-2">
      <label
        htmlFor={id}
        className="text-body whitespace-nowrap text-muted-foreground"
      >
        {shortLabel ?? label}
      </label>
      <span className="flex items-center gap-0.5">
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={lower}
          disabled={value <= min}
          onClick={() => set(value - step)}
        >
          <Minus />
        </Button>
        <input
          id={id}
          aria-label={label}
          className={cn(fieldClass, "w-12 text-center")}
          inputMode="numeric"
          value={shown}
          placeholder={placeholder}
          onChange={(e) => {
            const text = e.target.value;
            setDraft(text);
            const n = Number(text.trim());
            if (/^[+-]?\d+$/.test(text.trim()) && n >= min && n <= max)
              onChange(n);
            else if (text.trim() === "" && onClear) onClear();
          }}
          onBlur={() => setDraft(null)}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={raise}
          disabled={value >= max}
          onClick={() => set(value + step)}
        >
          <Plus />
        </Button>
      </span>
    </div>
  );
}
