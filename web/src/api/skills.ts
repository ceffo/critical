import type { SkillAdjustment, SkillCheck, Skills } from "@/types/skills";
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
