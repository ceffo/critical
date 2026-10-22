// Mirrors api/internal/rest/handler/skillhdl (SkillsDTO, CheckDTO). Targets and verdicts are the
// server's (ADR 0014); the client only words them. A DM adjustment moves the target (+ easier,
// - harder): `target` stays the unadjusted one, `adjustedTarget` is the one the roll is judged
// against (steps 5 to 8).

export type Ability = "str" | "dex" | "con" | "int" | "wis" | "cha";

export type SkillKind = "proficiency" | "bardSkill" | "thiefSkill";

/** The newest non-voided check of a skill, with the adjustment and verdict in effect. */
export interface LastCheck {
  entryId: number;
  roll: number;
  target: number;
  adjustment: number;
  /** Set when the adjustment in effect has a reason. */
  adjustmentReason?: string;
  adjustedTarget: number;
  success: boolean;
}

export interface Proficiency {
  id: string;
  ability: Ability;
  abilityScore: number;
  modifier: number;
  /** abilityScore + modifier, computed by the server. */
  target: number;
  last: LastCheck | null;
}

/** A thieving skill (ADR 0030 step 17) or a bard skill (step 18): percent = base + points + race +
 * dexterity + armor, capped at 95 and floored at 0, computed by the server. */
export interface RogueSkill {
  id: string;
  base: number;
  points: number;
  race: number;
  dexterity: number;
  armor: number;
  percent: number;
  /** The sum of the terms before the cap and the floor; it differs from `percent` when one
   * applied (more: capped at maxPercent, less: floored at 0). */
  rawPercent: number;
  /** The cap on the percentage, and the most points one skill takes (ADR 0030 steps 17, 18). */
  maxPercent: number;
  maxPoints: number;
  /** False while the server says the skill cannot be rolled. */
  usable: boolean;
  /** The worn body armour blocks the skill (ADR 0042 step 16.3): its roll is refused with
   * `skill_blocked_by_armour`. Absent when it does not. */
  blocked?: boolean;
  last: LastCheck | null;
}

export type ThiefSkill = RogueSkill;
export type BardSkill = RogueSkill;

/** One skill set of the character (ADR 0040 step 3.5): one per rogue_skills feature its classes
 * grant, each with its own budget and points. `set` is the code the points are spent under, `kind`
 * the check kind its rolls are logged as (a thief's set logs thiefSkill, ADR 0040 step 3.2). */
export interface SkillSet {
  set: string;
  kind: Exclude<SkillKind, "proficiency">;
  /** ADR 0033 step 14: the skill armour the set reads, from the body armour worn (none, leather,
   * padded_studded_leather, elven_chain, metal). */
  skillArmor: string;
  /** The skill point budget with the terms of its formula; null before the level its points
   * start. */
  budget: SkillBudget | null;
  skills: RogueSkill[];
}

export interface Skills {
  proficiencies: Proficiency[];
  /** One set per skill set the class grants; empty for a class without. */
  skillSets: SkillSet[];
}

/** ADR 0030 steps 17 and 18: available = first + perLevel x (level - 1) points in all, spent
 * entered, left = available - spent (negative when over). perSkillCap = perSkillFirst +
 * perSkillPerLevel x (level - 1) is the most one skill holds (a thief); all 0 for no such limit
 * (a bard). Computed by the server. */
export interface SkillBudget {
  level: number;
  first: number;
  perLevel: number;
  available: number;
  spent: number;
  left: number;
  perSkillFirst: number;
  perSkillPerLevel: number;
  perSkillCap: number;
}

/** One check as POST /api/roll/skill/{id} and POST /api/roll/skill/{id}/adjust return it, and the
 * payload in effect of a skill_check entry (without logEntryId). success is the verdict in effect,
 * rolledSuccess the verdict as rolled (with the pre-roll adjustment). adjustment, adjustedTarget
 * and rolledSuccess are always in a response; a payload logged before adjustments lacks them (no
 * adjustment, the verdict as rolled). previousSuccess and previousAdjustment are set in the
 * payload of a post-roll adjustment only. ability, abilityScore and modifier are set for a
 * proficiency only. */
export interface SkillCheck {
  id: string;
  kind: SkillKind;
  die: number;
  roll: number;
  target: number;
  adjustment?: number;
  adjustmentReason?: string;
  adjustedTarget?: number;
  success: boolean;
  rolledSuccess?: boolean;
  previousSuccess?: boolean;
  previousAdjustment?: number;
  ability?: Ability;
  abilityScore?: number;
  modifier?: number;
  logEntryId?: number;
  /** The buff the verdict in effect put on (ADR 0037 step 20); absent when none, and in entries
   * logged before. `renewed`: it replaced an active one. */
  buff?: SkillCheckBuff;
  renewed?: boolean;
}

/** The buff a successful check put on (skillhdl CheckBuffDTO, the skill_check payload's `buff`):
 * its row id, stored name, the proficiency it is named from, effects keyed like the buff API, and
 * rounds. */
export interface SkillCheckBuff {
  id: string;
  name: string;
  proficiencyId: string;
  effects: Record<string, number>;
  rounds: number;
}

/** Body of POST /api/roll/skill/{id} (optional) and POST /api/roll/skill/{id}/adjust: proficiency
 * -20..+20, bard skill -100..+100 (percentage points); reason at most 80 characters. */
export interface SkillAdjustment {
  adjustment: number;
  reason?: string;
}

/** One ability check as POST /api/roll/ability/{ability} and POST /api/roll/ability/{id}/adjust
 * return it (skillhdl AbilityCheckDTO), and the payload in effect of an ability_check entry
 * (without logEntryId; ADR 0028 step 5). abilityScore is the sheet's score, target the score in
 * effect when it was rolled, adjustedTarget = target + adjustment the one the roll was judged
 * against; success is the verdict in effect, rolledSuccess the verdict as rolled. previousSuccess
 * and previousAdjustment are set in the payload of an adjustment only. */
export interface AbilityCheck {
  ability: Ability;
  die: number;
  roll: number;
  abilityScore: number;
  target: number;
  adjustment: number;
  adjustmentReason?: string;
  adjustedTarget: number;
  success: boolean;
  rolledSuccess: boolean;
  previousSuccess?: boolean;
  previousAdjustment?: number;
  logEntryId?: number;
}
