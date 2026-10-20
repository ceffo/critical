import type {
  CreationOptions,
  PicksInput,
  ProficiencyOption,
} from "@/types/character";

/** One pick as the editor holds and sends it: the catalogue id and the slots spent. */
export type PickInput = PicksInput["nonweapon"][number];

/** A catalogue row as the editor lists it: the option, its name in the UI language and the
 * server's cost for the class. */
export interface CatalogueRow {
  option: ProficiencyOption;
  name: string;
  cost: number;
}

/** Folds a text for the search: lower case, accents off ("Équitation" finds "equi"). */
export function fold(s: string): string {
  return s.normalize("NFD").replace(/\p{M}/gu, "").toLocaleLowerCase();
}

/**
 * The catalogue split as the triage asks (ADR 0037 steps 2 and 4): the proficiencies of one of the
 * class's Table 38 groups first, then the others, each by name, filtered by the search (on the
 * name, accents ignored). The cost shown is the server's (`proficiencyCosts`), never derived here;
 * a proficiency the server gives no cost for the class is not offered (the server leaves out what
 * is never picked on its own, step 6).
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
    .flatMap((p) => {
      const cost = costs[p.id];
      return cost === undefined
        ? []
        : [{ option: p, name: nameOf(p.id), cost }];
    })
    .filter((r) => q === "" || fold(r.name).includes(q))
    .sort((a, b) => a.name.localeCompare(b.name, locale));
  const own: CatalogueRow[] = [];
  const other: CatalogueRow[] = [];
  for (const r of rows)
    (r.option.groups.some((g) => groups.includes(g)) ? own : other).push(r);
  return { own, other };
}

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
