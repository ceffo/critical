import type { CreationOptions } from "@/types/character";

/** A race's own language and its PHB list ("" and none for an unknown race or no catalogue). */
export function raceLanguages(
  options: Pick<CreationOptions, "raceLanguages">,
  race: string,
): { native: string; list: string[] } {
  const r = options.raceLanguages?.find((x) => x.race === race);
  return { native: r?.native ?? "", list: r?.languages ?? [] };
}
