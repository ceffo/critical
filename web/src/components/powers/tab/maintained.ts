// The rows of the "Maintained" card, as a rule on the buff data fields (never on a power id): a row
// keeps something up when it carries a power or a mind. What each pays at the next Next Round is the
// server's preview (ADR 0043 step 14); nothing here adds or compares PSPs.
import type { BuffEffect, RowCharge } from "@/types/buffs";

/** How a row's cost reads: paid every period, free for good, or a contact free while a power uses it. */
export type RowKind = "paid" | "standing" | "contact-free";

export interface MaintainedRow {
  id: string;
  kind: RowKind;
  powerId?: string;
  target?: string;
  /** A tangent row holds a mind with a count; a contact row has none. */
  tangent: boolean;
  cost: number;
  period: number;
  /** What the next Next Round charges the row (from the server). */
  charge: number;
  ends: boolean;
  /** The buff row's stored name, the fallback label. */
  name: string;
}

/** Whether a buff row keeps a power, a tangent or a contact up. */
export const keepsUp = (b: BuffEffect) => !!b.powerId || !!b.target;

/** The card's rows: every row that keeps something up, in the buff list's order, with its share of the preview. */
export function maintainedRows(
  buffs: BuffEffect[],
  charges: RowCharge[],
): MaintainedRow[] {
  return buffs.filter(keepsUp).map((b) => {
    const share = charges.find((c) => c.buffId === b.id);
    const cost = b.maintenanceCost ?? 0;
    const charge = share?.charge ?? 0;
    const tangent = !!b.target && !!b.tangents;
    const kind: RowKind =
      cost === 0 ? "standing" : share?.free ? "contact-free" : "paid";
    return {
      id: b.id,
      kind,
      powerId: b.powerId,
      target: b.target,
      tangent,
      cost,
      period: Math.max(b.maintenancePeriod ?? 1, 1),
      charge,
      ends: !!share?.ends,
      name: b.name,
    };
  });
}
