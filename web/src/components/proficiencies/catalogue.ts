import type {
  CreationOptions,
  PicksInput,
  ProficiencyOption,
  ProficiencySlots,
} from "@/types/character";

/** One pick as the editor holds and sends it: the catalogue id and the slots spent. */
export type PickInput = NonNullable<PicksInput["nonweapon"]>[number];

/** A catalogue row as the editor lists it: the option, its name in the UI language and the
 * server's cost for the class. */
export interface CatalogueRow {
  option: ProficiencyOption;
  name: string;
  cost: number;
  /** The check's target the server serves for a new pick at 0 extra slots (ADR 0037 step 23.4);
   * undefined before it serves one, null for no check. */
  target?: number | null;
}

/** What the picker lists, as the server serves it for the character: the catalogue, the character's
 * Table 38 groups (every active class entry's) and each proficiency's cost (what a preview and a
 * save charge; 0 for one a class grants free). */
export interface ServedCatalogue {
  proficiencies: ProficiencyOption[];
  groups: string[];
  costs: Record<string, number>;
  /** The served target of each proficiency's check for a new pick (step 23.4); empty before the
   * server serves the catalogue. */
  targets?: Record<string, number | null>;
}

/**
 * The catalogue the editor lists for a character (ADR 0042 step 15): the creation options' catalogue
 * with the groups and costs the server serves for this character (`proficiencySlots.groups` and
 * `.catalogue`), read as they are. A server that serves none (before the first answer, an older
 * release) leaves the creation options' figures for the class `cls`.
 */
export function servedCatalogue(
  options: Pick<
    CreationOptions,
    "proficiencies" | "proficiencyGroups" | "proficiencyCosts"
  >,
  cls: string,
  figures: Pick<ProficiencySlots, "groups" | "catalogue"> | null | undefined,
): ServedCatalogue {
  const costs =
    figures?.catalogue !== undefined
      ? Object.fromEntries(figures.catalogue.map((c) => [c.id, c.cost]))
      : (options.proficiencyCosts?.[cls] ?? {});
  return {
    proficiencies: options.proficiencies ?? [],
    groups: figures?.groups ?? options.proficiencyGroups?.[cls] ?? [],
    costs,
    targets: Object.fromEntries(
      (figures?.catalogue ?? []).map((c) => [c.id, c.target]),
    ),
  };
}

/** Folds a text for the search: lower case, accents off ("Équitation" finds "equi"). */
export function fold(s: string): string {
  return s.normalize("NFD").replace(/\p{M}/gu, "").toLocaleLowerCase();
}

/**
 * The catalogue rows by name, filtered by the search (on the name, accents ignored). The cost is the
 * server's, never derived here; a proficiency the server gives no cost is not offered (it leaves out
 * what is never picked on its own, ADR 0037 step 6).
 */
export function catalogueRows(
  catalogue: ServedCatalogue,
  nameOf: (id: string) => string,
  query: string,
  locale: string,
): CatalogueRow[] {
  const q = fold(query.trim());
  return catalogue.proficiencies
    .flatMap((p) => {
      const cost = catalogue.costs[p.id];
      return cost === undefined
        ? []
        : [
            {
              option: p,
              name: nameOf(p.id),
              cost,
              target: catalogue.targets?.[p.id],
            },
          ];
    })
    .filter((r) => q === "" || fold(r.name).includes(q))
    .sort((a, b) => a.name.localeCompare(b.name, locale));
}

/** One of the book's groups with its rows, and whether it is one of the character's (ADR 0037 step
 * 4: the character's groups cost normally, the others one slot more). */
export interface GroupBand {
  group: string;
  own: boolean;
  rows: CatalogueRow[];
}

/**
 * The rows under their groups (ADR 0037 steps 2 and 4): the character's groups first, in the order
 * the server gives them, then the others by name. A proficiency of several groups is listed under
 * each. A group with no row (the search) is left out.
 */
export function groupBands(
  rows: CatalogueRow[],
  groups: string[],
  groupName: (group: string) => string,
  locale: string,
): GroupBand[] {
  const seen = new Set(rows.flatMap((r) => r.option.groups));
  const others = [...seen]
    .filter((g) => !groups.includes(g))
    .sort((a, b) => groupName(a).localeCompare(groupName(b), locale));
  return [...groups, ...others]
    .map((group) => ({
      group,
      own: groups.includes(group),
      rows: rows.filter((r) => r.option.groups.includes(group)),
    }))
    .filter((b) => b.rows.length > 0);
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
