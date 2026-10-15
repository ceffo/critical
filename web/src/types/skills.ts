// Mirrors api/internal/rest/handler/skillhdl (SkillsDTO, CheckDTO). Targets and verdicts are the
// server's (ADR 0014); the client only words them.

export type Ability = "str" | "dex" | "con" | "int" | "wis" | "cha";

export type SkillKind = "proficiency" | "bardSkill";

/** The newest non-voided check of a skill. */
export interface LastCheck {
  entryId: number;
  roll: number;
  target: number;
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

export interface Skills {
  proficiencies: Proficiency[];
  bardSkills: BardSkill[];
}

/** One check as POST /api/roll/skill/{id} returns it, and the payload of a skill_check entry
 * (without logEntryId). ability, abilityScore and modifier are set for a proficiency only. */
export interface SkillCheck {
  id: string;
  kind: SkillKind;
  die: number;
  roll: number;
  target: number;
  success: boolean;
  ability?: Ability;
  abilityScore?: number;
  modifier?: number;
  logEntryId?: number;
}
