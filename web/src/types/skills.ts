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

export interface BardSkill {
  id: string;
  percent: number;
  last: LastCheck | null;
}

/** A thieving skill (ADR 0030 step 17): percent = base + points + race + dexterity + armor,
 * capped at 95 and floored at 0, computed by the server. */
export interface ThiefSkill {
  id: string;
  base: number;
  points: number;
  race: number;
  dexterity: number;
  armor: number;
  percent: number;
  last: LastCheck | null;
}

export interface Skills {
  proficiencies: Proficiency[];
  bardSkills: BardSkill[];
  thiefSkills: ThiefSkill[];
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
