import { Button as ButtonPrimitive } from "@base-ui/react/button";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-lg border border-transparent bg-clip-padding text-body font-medium whitespace-nowrap transition-all outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:border-dashed disabled:border-input disabled:bg-transparent disabled:text-ink-muted disabled:shadow-none aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        // A button with nothing to submit is disabled and looks it, the same in every variant: no
        // fill, muted text, a dashed edge; never a faded main button.
        // The zone's one every-turn action (ADR 0027 step 3.4): filled with the tint.
        main: "bg-primary text-primary-foreground hover:bg-primary/85",
        // Every other button: no fill, the field edge (3:1 on the card, so it reads on the card, the
        // raised step and the selected tone alike), the raised tone on hover; a pressed toggle is
        // the chosen chip (ADR 0026 step 5.3).
        quiet:
          "border-input text-foreground hover:bg-raised-hover aria-expanded:bg-raised-hover aria-pressed:border-transparent aria-pressed:bg-selected aria-pressed:shadow-[inset_0_0_0_1.5px_var(--tint)]",
        // Icon-only buttons in a toolbar or a row: no fill until hovered (disabled: muted, no edge).
        ghost:
          "hover:bg-raised-hover hover:text-foreground aria-expanded:bg-raised-hover aria-expanded:text-foreground disabled:border-transparent",
        // A quiet button whose action removes something.
        destructive:
          "border-input text-danger hover:bg-raised-hover focus-visible:border-danger/40 focus-visible:ring-danger/30",
      },
      size: {
        default:
          "h-7 gap-1.5 px-2.5 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        xs: "h-6 gap-1 rounded-[min(var(--radius-md),10px)] px-2 text-label in-data-[slot=button-group]:rounded-lg has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-7 gap-1 rounded-[min(var(--radius-md),12px)] px-2 in-data-[slot=button-group]:rounded-lg has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-8 gap-1.5 px-3 has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2",
        icon: "size-7",
        "icon-xs":
          "size-6 rounded-[min(var(--radius-md),10px)] in-data-[slot=button-group]:rounded-lg [&_svg:not([class*='size-'])]:size-3",
        "icon-sm":
          "size-7 rounded-[min(var(--radius-md),12px)] in-data-[slot=button-group]:rounded-lg",
        "icon-lg": "size-8",
      },
    },
    defaultVariants: {
      variant: "quiet",
      size: "default",
    },
  },
);

function Button({
  className,
  variant = "quiet",
  size = "default",
  ...props
}: ButtonPrimitive.Props & VariantProps<typeof buttonVariants>) {
  return (
    <ButtonPrimitive
      data-slot="button"
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export { Button, buttonVariants };
