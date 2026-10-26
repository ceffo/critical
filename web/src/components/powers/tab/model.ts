// What the Act > Powers tab shows, as rules on the fields of the powers the character knows (never
// on an id): the tags of a row, the groups, the defence chips and the minds held. Pure and
// store-free; the words come from the caller's `t`.
import type { TFunction } from "i18next";
import type { BuffEffect } from "@/types/buffs";
import type { PowerPick } from "@/types/character";
import type { Power, PowerCost } from "@/types/powers";

/** A cost in words: PSPs, or the word the book gives where it prints none. */
function costText(t: TFunction, c: PowerCost): string {
  return typeof c === "number"
    ? t("lookup.powers.psp", { count: c })
    : t(`lookup.powers.costWords.${c}`);
}

/** The row's tags, from `kind`, `initial_cost` and `maintenance`. */
export function rowTags(t: TFunction, p: Power): string[] {
  const tags: string[] = [];
  if (p.kind === "science" || p.kind === "devotion") {
    tags.push(t(`lookup.powers.kinds.${p.kind}`));
  }
  tags.push(costText(t, p.initial_cost));
  const m = p.maintenance;
  if (m) {
    tags.push(
      m.cost === 0
        ? t("powers.tab.standing")
        : t("lookup.powers.maintenanceValue", {
            cost:
              typeof m.cost === "number" ? `+${m.cost}` : costText(t, m.cost),
            count: m.period,
          }),
    );
  }
  return tags;
}

/** A defence mode that costs nothing to keep up is standing: it needs no round. */
export const isStanding = (p: Power) => p.maintenance?.cost === 0;

/** A power that takes an opponent's mind: an attack mode, or one paid on contact. */
export const takesMind = (p: Power) =>
  p.kind === "attack_mode" || p.initial_cost === "contact";

/** One group of rows: the attack modes first, then each discipline in the order first learned. */
export interface Group {
  key: string;
  /** The discipline id, or null for the attack modes. */
  discipline: string | null;
  powers: Power[];
}

/**
 * The known powers in learning order, minus the defence modes (the chips above show those), grouped:
 * attack modes first, then one group per `discipline` in the order its first power was learned.
 */
export function groupsOf(known: Power[]): Group[] {
  const attacks = known.filter((p) => p.kind === "attack_mode");
  const groups: Group[] = [];
  if (attacks.length > 0) {
    groups.push({ key: "attack", discipline: null, powers: attacks });
  }
  for (const p of known) {
    if (p.kind === "attack_mode" || p.kind === "defence_mode") continue;
    let g = groups.find((x) => x.discipline === p.discipline);
    if (!g) {
      g = { key: `d-${p.discipline}`, discipline: p.discipline, powers: [] };
      groups.push(g);
    }
    g.powers.push(p);
  }
  return groups;
}

/** The known powers, in the order of the picks (a relearn names a power already listed). */
export function inLearningOrder(
  picks: PowerPick[],
  byId: Record<string, Power | undefined>,
): Power[] {
  const seen = new Set<string>();
  const out: Power[] = [];
  for (const pick of picks) {
    if (pick.kind !== "power" || seen.has(pick.id)) continue;
    const p = byId[pick.id];
    if (!p) continue;
    seen.add(pick.id);
    out.push(p);
  }
  return out;
}

/** A mind held in a tangent (1 or 2) or contact row. */
export interface HeldMind {
  name: string;
  /** The tangents held; absent for contact. */
  tangents?: number;
}

/** The minds the rows hold, one entry per label, the first row's figures. */
export function heldMinds(buffs: BuffEffect[]): HeldMind[] {
  const out: HeldMind[] = [];
  for (const b of buffs) {
    if (!b.target || out.some((m) => m.name === b.target)) continue;
    out.push({ name: b.target, tangents: b.tangents });
  }
  return out;
}
