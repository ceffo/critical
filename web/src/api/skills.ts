import type {
  Ability,
  AbilityCheck,
  SkillAdjustment,
  SkillCheck,
  Skills,
} from "@/types/skills";
import { post, request } from "./http";

export function fetchSkills(): Promise<Skills> {
  return request<Skills>("/api/skills");
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
