// What casting one slot asks of the player, from the spellbook's cast fields (the spell effects
// data: slot_splits, consume_item_type, attack_roll, modes, cast_target). No game rule is decided here: the server
// validates every cast; this only picks which inputs to show and what to send.
import type { CastOptions } from "@/api/spells";
import type {
  CastOn,
  CastTarget,
  SpellbookEntry,
  SpellMode,
  SpellSlot,
  SpendableItem,
} from "@/types/spells";

/** The player's inputs for one slot, as typed. */
export interface CastForm {
  /** Blank extra slot: the spell picked. */
  pick?: string;
  /** "1,2": the chosen split of the extra slots granted. */
  split?: string;
  acText?: string;
  itemPick?: string;
  /** The key of the cast-time mode picked. */
  mode?: string;
}

export interface CastPlan {
  /** The spellbook entry of the spell cast (the pick for a blank slot). */
  needs?: SpellbookEntry;
  slotSplits: number[][];
  split?: string;
  targetAc?: number;
  /** Items the spell can spend; a select is shown when there are several. */
  spendable: SpendableItem[];
  itemId?: string;
  /** Cast-time modes to pick from (the first is the default) and the one picked. */
  modes: SpellMode[];
  mode?: SpellMode;
  /** The target choice the server offers for the spell; absent: the cast button has no arrow. */
  target?: CastTarget;
  /** Cast is disabled: slot used, no spell picked for a blank slot, or no target AC when needed. */
  blocked: boolean;
  options: Omit<CastOptions, "onSelf">;
}

/** The cast fields a spell's inputs are read from: a spellbook entry's, or the asks of an item's spell effect. */
export type CastFields = Pick<
  SpellbookEntry,
  | "slot_splits"
  | "consume_item_type"
  | "attack_roll"
  | "modes"
  | "cast_target"
  | "items"
>;

/** The answers as typed, for the fields of one spell. */
export type AskForm = Pick<CastForm, "split" | "acText" | "itemPick" | "mode">;

/** What one spell asks, and the choices the form makes of it (nothing decided here: the server validates). */
export interface AskPlan {
  slotSplits: number[][];
  split?: string;
  targetAc?: number;
  /** Items the spell can spend; a select is shown when there are several. */
  spendable: SpendableItem[];
  itemId?: string;
  /** Cast-time modes to pick from (the first is the default) and the one picked. */
  modes: SpellMode[];
  mode?: SpellMode;
  /** The target choice the server offers; absent: none. */
  target?: CastTarget;
  /** A to-hit is rolled and no target AC was typed. */
  missingAc: boolean;
  /** The answers to send, by the cast request's names. */
  answer: Omit<CastOptions, "onSelf" | "spellId">;
}

/** The inputs one spell's cast fields offer, and what the form answers (shared by a slot's cast and an item's spell effect). */
export function planAsks(
  needs: CastFields | undefined,
  form: AskForm,
): AskPlan {
  const slotSplits = needs?.slot_splits ?? [];
  const split = form.split ?? slotSplits[0]?.join(",");
  const acText = (form.acText ?? "").trim();
  const targetAc = /^-?\d+$/.test(acText) ? Number(acText) : undefined;
  const spendable = needs?.items ?? [];
  const itemId = form.itemPick ?? spendable[0]?.id;
  const modes = needs?.modes ?? [];
  const mode = modes.find((m) => m.key === form.mode) ?? modes[0];
  return {
    slotSplits,
    split,
    targetAc,
    spendable,
    itemId,
    modes,
    mode,
    target: needs?.cast_target,
    missingAc: needs?.attack_roll === true && targetAc === undefined,
    answer: {
      extraSlots:
        slotSplits.length > 0 && split
          ? split.split(",").map(Number)
          : undefined,
      targetAc: needs?.attack_roll ? targetAc : undefined,
      // One item left: the server takes it; none: the server refuses with its message.
      itemId: spendable.length > 1 ? itemId : undefined,
      mode: mode?.key,
    },
  };
}

/**
 * Whether a spell's cast fields leave the player something to answer: a to-hit's target AC, one of
 * several items, splits or modes, a target choice. Nothing to choose: the use takes the defaults and
 * opens no dialog.
 */
export function asksAnything(needs: CastFields | undefined): boolean {
  if (!needs) return false;
  return (
    needs.attack_roll === true ||
    (needs.items?.length ?? 0) > 1 ||
    (needs.slot_splits?.length ?? 0) > 1 ||
    (needs.modes?.length ?? 0) > 1 ||
    castMenu(needs.cast_target).length > 1
  );
}

export function planCast(
  slot: SpellSlot,
  blank: boolean,
  spellbook: SpellbookEntry[],
  form: CastForm,
): CastPlan {
  const castId = blank ? form.pick : slot.prepared_spell_id;
  const needs = castId ? spellbook.find((s) => s.id === castId) : undefined;
  const p = planAsks(needs, form);
  const blocked = slot.used || (blank && !form.pick) || p.missingAc;
  return {
    needs,
    slotSplits: p.slotSplits,
    split: p.split,
    targetAc: p.targetAc,
    spendable: p.spendable,
    itemId: p.itemId,
    modes: p.modes,
    mode: p.mode,
    target: p.target,
    blocked,
    options: {
      spellId: blank ? form.pick : undefined,
      ...p.answer,
    },
  };
}

/**
 * The targets the arrow's menu lists, from the served `cast_target` only: where the main part lands
 * (`default`) first, then the other one. A menu that would hold a single entry is no menu: the
 * button has no arrow then.
 */
export function castMenu(target?: CastTarget): CastOn[] {
  if (!target) return [];
  const entries = [target.default, target.alternate];
  return entries.filter((on, i) => entries.indexOf(on) === i);
}
