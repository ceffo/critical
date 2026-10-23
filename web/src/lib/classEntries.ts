import type { ClassEntry } from "@/types/character";

/**
 * The first active class entry, else the first entry: the one the single-class screens read
 * (ADR 0045 step 5). The server's rule is `View.Primary`; the case table in `classEntries.test.ts`
 * is the same as the Go test's (`TestView_Step3_PrimaryIsTheFirstActive`).
 */
export function primaryEntry(
  entries: ClassEntry[] | undefined,
): ClassEntry | undefined {
  return entries?.find((e) => e.state === "active") ?? entries?.[0];
}
