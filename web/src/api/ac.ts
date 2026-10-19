import type { ArmourClass } from "@/types/ac";
import { del, put, request } from "./http";

/** The character's AC, its breakdown and its override (ADR 0033 steps 10 to 13). */
export function fetchAc(): Promise<ArmourClass> {
  return request<ArmourClass>("/api/ac");
}

/** Sets the override (logged, undone like any move); answers the AC. */
export function setAcOverride(
  value: number,
  label: string,
): Promise<ArmourClass> {
  return put<ArmourClass>("/api/ac/override", { value, label });
}

/** Clears the override (logged); answers the AC. */
export function clearAcOverride(): Promise<ArmourClass> {
  return del<ArmourClass>("/api/ac/override");
}
