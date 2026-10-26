import type { PspPool, PspRecoveryRow } from "@/types/character";
import type { BuffEffect } from "@/types/buffs";

/** The hour presets the rest popover offers (ADR 0047 item 11). */
export const REST_PRESETS = [1, 4, 8] as const;

/** Hours the stepper starts on. */
export const DEFAULT_REST_HOURS = 8;

/**
 * Table 6's rows the character may rest with (ADR 0047 items 5 and 11): a row that names a
 * proficiency shows only when the character holds it, hidden rather than disabled. The rows, their
 * order and the gate are the server's; `held` is the ids of the proficiencies the character knows.
 */
export function visibleRecovery(
  rows: PspRecoveryRow[] | undefined,
  held: ReadonlySet<string>,
): PspRecoveryRow[] {
  return (rows ?? []).filter(
    (r) => !r.requiresProficiency || held.has(r.requiresProficiency),
  );
}

/** The row a rest opens on: the fastest one on offer (the first of equals, in the table's order). */
export function defaultRow(rows: PspRecoveryRow[]): PspRecoveryRow | undefined {
  return rows.reduce<PspRecoveryRow | undefined>(
    (best, r) => (!best || r.perHour > best.perHour ? r : best),
    undefined,
  );
}

/**
 * The PSPs after resting `hours` at `perHour`, as the preview shows them (the server's figure:
 * hours times the rate rounded down, the pool capped at its maximum). Shown, never decided: the
 * answer of the recovery is what the card then reads.
 */
export function previewRest(
  pool: Pick<PspPool, "current" | "max">,
  perHour: number,
  hours: number,
): { gain: number; after: number } {
  if (!(hours > 0)) return { gain: 0, after: pool.current };
  const asked = Math.floor(hours * perHour + 1e-9);
  const after = Math.min(pool.max, pool.current + asked);
  return { gain: Math.max(0, after - pool.current), after };
}

/** A row that keeps a power or a mind up (a maintained power, a tangent, a contact). */
const keepsUp = (b: BuffEffect) => !!(b.powerId || b.target);

/** The rows that pay PSPs each period (what the server's stop-and-recover ends). */
export function payingRows(buffs: BuffEffect[]): BuffEffect[] {
  return buffs.filter((b) => keepsUp(b) && (b.maintenanceCost ?? 0) > 0);
}

/** The rows kept up at no cost, which stay up through a rest. */
export function freeRows(buffs: BuffEffect[]): BuffEffect[] {
  return buffs.filter((b) => keepsUp(b) && !((b.maintenanceCost ?? 0) > 0));
}
