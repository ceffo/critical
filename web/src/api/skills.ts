import type { SkillCheck, Skills } from "@/types/skills";
import { post, request } from "./http";

export function fetchSkills(): Promise<Skills> {
  return request<Skills>("/api/skills");
}

/** Rolls one check of the skill on the server; it is logged (skill_check). */
export function rollSkill(id: string): Promise<SkillCheck> {
  return post<SkillCheck>(`/api/roll/skill/${encodeURIComponent(id)}`);
}
