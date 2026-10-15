import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

/** The type scale by role (ADR 0017 step 2), the only text sizes. */
export const TYPE_ROLES = ["label", "body", "title", "key", "headline"] as const;

// tailwind-merge only knows Tailwind's default sizes: without this, `text-body` would be taken
// for a colour and dropped next to `text-muted-foreground`.
const twMerge = extendTailwindMerge({
  extend: { classGroups: { "font-size": [{ text: [...TYPE_ROLES] }] } },
});

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
