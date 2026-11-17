import type { ReactNode } from "react";
import { PencilSimple } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * A section's Edit, or Act's Manage (ADR 0027 step 3.4, the pending state): a quiet button that,
 * while something is left to spend behind it, takes the danger edge and a red dot in place of its
 * pencil. The mark sits on what is to be clicked: no text pill, no second filled button. The dot is
 * decoration; `label` (the aria-label and tooltip) says what the button does, `pendingLabel` what
 * is left, read by screen readers beside it.
 */
export function PendingButton({
  pending,
  label,
  pendingLabel,
  onClick,
  testId,
  className,
  children,
}: {
  pending: boolean;
  label: string;
  pendingLabel: string;
  onClick: () => void;
  testId?: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <Button
      type="button"
      size="sm"
      variant={pending ? "pending" : "quiet"}
      aria-label={pending ? `${label} (${pendingLabel})` : label}
      title={pending ? `${label} (${pendingLabel})` : label}
      data-pending={pending ? "true" : undefined}
      data-testid={testId}
      className={cn(className)}
      onClick={onClick}
    >
      {pending ? (
        <span aria-hidden className="size-2 rounded-full bg-danger" />
      ) : (
        <PencilSimple aria-hidden />
      )}
      {children}
    </Button>
  );
}
