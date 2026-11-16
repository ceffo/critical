import type { LocalizedText } from "@/types/localized";
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

/** What `GET /api/skill-sets/{set}/skills/{skill}/description` answers (ADR 0037 step 19.6). */
export interface SkillDescription {
  set: string;
  skill: string;
  description: LocalizedText;
}

const descriptions = new Map<string, Promise<SkillDescription>>();

/**
 * The description of a skill of a skill set in every language, read when its pop-up opens and kept
 * for the page's life: the books' text never changes under a running page. A set has its own text
 * for a skill, so the key is the pair. A failed read is not kept, so reopening the pop-up asks
 * again.
 */
export function getSkillDescription(
  set: string,
  skill: string,
): Promise<SkillDescription> {
  const key = `${set}/${skill}`;
  let held = descriptions.get(key);
  if (!held) {
    held = request<SkillDescription>(
      `/api/skill-sets/${encodeURIComponent(set)}/skills/${encodeURIComponent(skill)}/description`,
    );
    descriptions.set(key, held);
    held.catch(() => descriptions.delete(key));
  }
  return held;
}

/** Sets the total points per skill of the skill set `set` (ADR 0030 step 17, ADR 0040 step 3.5);
 * logged (skill_points), so it is undone like any move. Answers the skills. */
export function setSkillPoints(
  set: string,
  points: Record<string, number>,
): Promise<Skills> {
  return put<Skills>("/api/skills/points", { set, points });
}

/** The skills and budget the points would give, as the server derives them; writes and logs
 * nothing (ADR 0030 steps 17 and 18). */
export function previewSkillPoints(
  set: string,
  points: Record<string, number>,
): Promise<Skills> {
  return post<Skills>("/api/skills/points/preview", { set, points });
}

/** Rolls one check of the skill on the server; it is logged (skill_check). An adjustment moves its
 * target. `set` names the skill set when the character has two that share the skill. */
export function rollSkill(
  id: string,
  adjustment?: SkillAdjustment,
  set?: string,
): Promise<SkillCheck> {
  const query = set ? `?set=${encodeURIComponent(set)}` : "";
  return post<SkillCheck>(
    `/api/roll/skill/${encodeURIComponent(id)}${query}`,
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
