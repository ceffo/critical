import type {
  Ability,
  AbilityCheck,
  SkillAdjustment,
  SkillCheck,
  Skills,
} from "@/types/skills";
import { post, put, request } from "./http";

export function fetchSkills(): Promise<Skills> {
  return request<Skills>("/api/skills");
}

/** Sets a thief's total points per skill (ADR 0030 step 17); logged (skill_points), so it is
 * undone like any move. Answers the skills. */
export function setSkillPoints(
  points: Record<string, number>,
): Promise<Skills> {
  return put<Skills>("/api/skills/points", { points });
}

/** The skills and budget the points would give, as the server derives them; writes and logs
 * nothing (ADR 0030 steps 17 and 18). */
export function previewSkillPoints(
  points: Record<string, number>,
): Promise<Skills> {
  return post<Skills>("/api/skills/points/preview", { points });
}

/** Rolls one check of the skill on the server; it is logged (skill_check). An adjustment moves its target. */
export function rollSkill(
  id: string,
  adjustment?: SkillAdjustment,
): Promise<SkillCheck> {
  return post<SkillCheck>(
    `/api/roll/skill/${encodeURIComponent(id)}`,
    adjustment,
  );
}

/** Sets the adjustment of the latest check (log entry id); the server recomputes the verdict. */
export function adjustSkill(
  entryId: number,
  adjustment: number,
  reason: string,
): Promise<SkillCheck> {
  return post<SkillCheck>(`/api/roll/skill/${entryId}/adjust`, {
    adjustment,
    reason,
  });
}

/** Rolls a check of the ability on the server against its score in effect; it is logged (ability_check). */
export function rollAbility(ability: Ability): Promise<AbilityCheck> {
  return post<AbilityCheck>(`/api/roll/ability/${ability}`);
}

/** Sets the adjustment of the latest ability check (log entry id); the server recomputes the verdict. */
export function adjustAbility(
  entryId: number,
  adjustment: number,
  reason: string,
): Promise<AbilityCheck> {
  return post<AbilityCheck>(`/api/roll/ability/${entryId}/adjust`, {
    adjustment,
    reason,
  });
}
