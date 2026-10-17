// What casting one slot asks of the player, from the spellbook's cast fields (the spell effects
// data: slot_splits, consume_item_type, attack_roll, modes, cast_target). No game rule is decided here: the server
// validates every cast; this only picks which inputs to show and what to send.
import type { CastOptions } from "@/api/spells";
import type {
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

export function planCast(
  slot: SpellSlot,
  blank: boolean,
  spellbook: SpellbookEntry[],
  form: CastForm,
): CastPlan {
  const castId = blank ? form.pick : slot.prepared_spell_id;
  const needs = castId ? spellbook.find((s) => s.id === castId) : undefined;
  const slotSplits = needs?.slot_splits ?? [];
  const split = form.split ?? slotSplits[0]?.join(",");
  const acText = (form.acText ?? "").trim();
  const targetAc = /^-?\d+$/.test(acText) ? Number(acText) : undefined;
  const spendable = needs?.items ?? [];
  const itemId = form.itemPick ?? spendable[0]?.id;
  const modes = needs?.modes ?? [];
  const mode = modes.find((m) => m.key === form.mode) ?? modes[0];
  const blocked =
    slot.used ||
    (blank && !form.pick) ||
    (needs?.attack_roll === true && targetAc === undefined);
  return {
    needs,
    slotSplits,
    split,
    targetAc,
    spendable,
    itemId,
    modes,
    mode,
    target: needs?.cast_target,
    blocked,
    options: {
      spellId: blank ? form.pick : undefined,
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
