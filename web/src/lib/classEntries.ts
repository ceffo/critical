import type { ClassEntry } from "@/types/character";

/** The code of the first active class entry: the class the single-class screens read (ADR 0042 step 1). */
export function primaryClass(entries: ClassEntry[] | undefined): string {
  return (entries?.find((e) => e.state === "active") ?? entries?.[0])?.class ?? "";
}

/** The level of the first active class entry. */
export function primaryLevel(entries: ClassEntry[] | undefined): number {
  return (entries?.find((e) => e.state === "active") ?? entries?.[0])?.level ?? 0;
}
