import type { CreationOptions } from "@/types/character";

/**
 * PHB Table 4's number of additional languages for an Intelligence score, from the server's bands
 * (ADR 0032 step 4.2); undefined when the score is unknown or the server serves no bands (a
 * release that predates them). The rule and the warning on the card are the server's: this only
 * reads the served table for the form's live count.
 */
export function languageLimit(
  options: Pick<CreationOptions, "languageLimits">,
  score: number | null,
): number | undefined {
  if (score === null || Number.isNaN(score)) return undefined;
  return options.languageLimits?.find(
    (b) => score >= b.intMin && score <= b.intMax,
  )?.limit;
}

/** A race's own language and its PHB list ("" and none for an unknown race or no catalogue). */
export function raceLanguages(
  options: Pick<CreationOptions, "raceLanguages">,
  race: string,
): { native: string; list: string[] } {
  const r = options.raceLanguages?.find((x) => x.race === race);
  return { native: r?.native ?? "", list: r?.languages ?? [] };
}

/** The additional languages picked: the native one is not counted. */
export function additionalLanguages(
  picked: string[],
  native: string,
): string[] {
  return picked.filter((l) => l !== native);
}
