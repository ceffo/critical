import type {
  CreationOptions,
  PicksInput,
  ProficiencyOption,
} from "@/types/character";

/** One pick as the editor holds and sends it: the catalogue id and the slots spent. */
export type PickInput = PicksInput["nonweapon"][number];

/** A catalogue row as the editor lists it: the option, its name in the UI language and the
 * server's cost for the class (undefined from a server that does not send it: not offered). */
export interface CatalogueRow {
  option: ProficiencyOption;
  name: string;
  cost: number | undefined;
}

/** Folds a text for the search: lower case, accents off ("Équitation" finds "equi"). */
export function fold(s: string): string {
  return s.normalize("NFD").replace(/\p{M}/gu, "").toLocaleLowerCase();
}

/**
 * The catalogue split as the triage asks (ADR 0037 steps 2 and 4): the proficiencies of one of the
 * class's Table 38 groups first, then the others, each by name, filtered by the search (on the
 * name, accents ignored). The cost shown is the server's (`proficiencyCosts`), never derived here;
 * "Languages, Modern" is no pick of its own (step 6) and is left out.
 */
export function splitCatalogue(
  options: Pick<
    CreationOptions,
    "proficiencies" | "proficiencyGroups" | "proficiencyCosts"
  >,
  cls: string,
  nameOf: (id: string) => string,
  query: string,
  locale: string,
): { own: CatalogueRow[]; other: CatalogueRow[] } {
  const groups = options.proficiencyGroups?.[cls] ?? [];
  const costs = options.proficiencyCosts?.[cls] ?? {};
  const q = fold(query.trim());
  const rows = (options.proficiencies ?? [])
    .filter((p) => p.id !== LANGUAGES_MODERN)
    .map((p) => ({ option: p, name: nameOf(p.id), cost: costs[p.id] }))
    .filter((r) => q === "" || fold(r.name).includes(q))
    .sort((a, b) => a.name.localeCompare(b.name, locale));
  const own: CatalogueRow[] = [];
  const other: CatalogueRow[] = [];
  for (const r of rows)
    (r.option.groups.some((g) => groups.includes(g)) ? own : other).push(r);
  return { own, other };
}

/** "Languages, Modern": a language of the character's list, never picked on its own (ADR 0037
 * step 6, refused `not_allowed`). */
export const LANGUAGES_MODERN = "languages_modern";

/** The picks with one more: the catalogue id at the server's cost for the class (the fewest slots
 * the server takes, step 5). Already picked or with no cost: unchanged. */
export function addPick(
  picks: PickInput[],
  id: string,
  cost: number | undefined,
) {
  if (cost === undefined || picks.some((p) => p.id === id)) return picks;
  return [...picks, { id, slots: cost }];
}

/** The picks with one pick's slots set (the stepper's value); the server checks the range. */
export function setSlots(
  picks: PickInput[],
  id: string,
  slots: number,
): PickInput[] {
  return picks.map((p) => (p.id === id ? { ...p, slots } : p));
}

/** The picks without one. */
export function removePick(picks: PickInput[], id: string): PickInput[] {
  return picks.filter((p) => p.id !== id);
}
