// The registry: one view per entry kind. The type checker refuses a LogKind without its view, so a
// new kind is added here, in its kind file and in the i18n, and nowhere else.
import type { LogKind } from "@/types/log";
import type { AnyKindView } from "./kindView";
import { abilityCheckKind } from "./kinds/abilityCheck";
import { attackKind } from "./kinds/attack";
import {
  buffChangeKind,
  roundAdvanceKind,
  roundResetKind,
  roundSetKind,
  timePassedKind,
} from "./kinds/buff";
import { castKind } from "./kinds/cast";
import {
  acOverrideKind,
  levelUpKind,
  longRestKind,
  skillArmorKind,
  skillPointsKind,
  spellLearnKind,
  spellPrepKind,
  xpAddKind,
  xpEditKind,
  xpRemoveKind,
} from "./kinds/character";
import { correctionKind } from "./kinds/correction";
import { diceKind } from "./kinds/dice";
import { exposureKind } from "./kinds/exposure";
import { hpChangeKind } from "./kinds/hpChange";
import { initiativeKind } from "./kinds/initiative";
import {
  equipmentStateKind,
  itemAddKind,
  itemEditKind,
  itemRemoveKind,
  itemRestockKind,
  weaponAddKind,
  weaponEditKind,
  weaponRemoveKind,
} from "./kinds/inventory";
import { itemUseKind } from "./kinds/itemUse";
import { saveKind } from "./kinds/save";
import { skillCheckKind } from "./kinds/skillCheck";
import { turnUndeadKind } from "./kinds/turnUndead";

export const KIND_VIEWS: Record<LogKind, AnyKindView> = {
  hp_change: hpChangeKind,
  cast: castKind,
  exposure: exposureKind,
  buff_change: buffChangeKind,
  round_advance: roundAdvanceKind,
  round_reset: roundResetKind,
  round_set: roundSetKind,
  time_passed: timePassedKind,
  initiative: initiativeKind,
  attack: attackKind,
  item_use: itemUseKind,
  item_restock: itemRestockKind,
  item_add: itemAddKind,
  item_edit: itemEditKind,
  item_remove: itemRemoveKind,
  weapon_add: weaponAddKind,
  weapon_edit: weaponEditKind,
  weapon_remove: weaponRemoveKind,
  equipment_state: equipmentStateKind,
  xp_add: xpAddKind,
  xp_edit: xpEditKind,
  xp_remove: xpRemoveKind,
  level_up: levelUpKind,
  spell_learn: spellLearnKind,
  turn_undead: turnUndeadKind,
  skill_points: skillPointsKind,
  skill_armor: skillArmorKind,
  ac_override: acOverrideKind,
  spell_prep: spellPrepKind,
  long_rest: longRestKind,
  save: saveKind,
  skill_check: skillCheckKind,
  ability_check: abilityCheckKind,
  dice: diceKind,
  correction: correctionKind,
};

/** The view of a kind; null for a kind this web does not know (a newer server's). */
export function viewOf(kind: string): AnyKindView | null {
  return Object.hasOwn(KIND_VIEWS, kind) ? KIND_VIEWS[kind as LogKind] : null;
}
