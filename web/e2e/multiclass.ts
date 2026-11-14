import type { Locator, Page, Route } from "@playwright/test";

/**
 * A multi-class (or dual-class) character for the specs. The API cannot create one yet, so the real
 * answers of the real API are overlaid with what the server serves for such a character
 * (`classes`, `derived.classes`, pending levels per class, the XP award's `split`). The overlay is
 * test data shaped like the DTOs (characterhdl, xphdl, xpsvc's log payload); the rules behind them
 * (the equal shares, the per-entry levels) are Go tests (xpsvc, charactersvc, levelupsvc).
 */
export interface MultiClassOptions {
  /** A frozen old class and one active (a dual-class), else two active classes. */
  dual?: boolean;
  /** Three active classes, at levels 5, 5 and 6, in place of two. */
  three?: boolean;
  /** One pending level per class. */
  pending?: boolean;
  /** A long first class name, to prove wrapping: the code, as the web has no wording for it. */
  longClass?: string;
  /** The first class is a specialist and the character's race is one the book does not list for it. */
  specialistRace?: boolean;
}

type Json = Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any

async function patched(route: Route, patch: (body: Json) => void) {
  const response = await route.fetch();
  const body = (await response.json()) as Json;
  patch(body);
  await route.fulfill({ response, json: body });
}

const split = (total: number, classes: string[]) =>
  classes.map((c) => ({
    class: c,
    amount: Math.floor(total / classes.length),
  }));

/** The prime requisites of the classes the specs use (catalogue data, served as `bonusClasses`). */
const PRIMES: Record<string, string[]> = {
  fighter: ["str"],
  thief: ["dex"],
  mage: ["int"],
};

/** The class codes sharing an award: the active entries. */
export function sharing(opts: MultiClassOptions): string[] {
  if (opts.dual) return ["mage"];
  const first = opts.longClass ?? "fighter";
  return opts.three ? [first, "mage", "thief"] : [first, "thief"];
}

export async function asMultiClass(page: Page, opts: MultiClassOptions = {}) {
  const first = opts.longClass ?? "fighter";
  const entries = opts.dual
    ? [
        {
          class: "fighter",
          level: 7,
          startingXp: 0,
          state: "frozen",
          leftAtLevel: 7,
        },
        {
          class: "mage",
          level: 5,
          startingXp: 0,
          state: "active",
          leftAtLevel: 0,
        },
      ]
    : opts.three
      ? [first, "mage", "thief"].map((c, i) => ({
          class: c,
          level: i === 2 ? 6 : 5,
          startingXp: 64000,
          state: "active",
          leftAtLevel: 0,
        }))
      : [
          {
            class: first,
            level: 7,
            startingXp: 64000,
            state: "active",
            leftAtLevel: 0,
          },
          {
            class: "thief",
            level: 9,
            startingXp: 160000,
            state: "active",
            leftAtLevel: 0,
          },
        ];
  const classes = sharing(opts);
  await page.route(
    (url) => url.pathname === "/api/character",
    (route) =>
      route.request().method() !== "GET"
        ? route.fallback()
        : patched(route, (c) => {
            c.classes = entries;
            c.derived.classes = entries.map((e, i) => ({
              class: e.class,
              state: e.state,
              // The first class sits on its floor; the others are part-way to the next level.
              xp: e.state === "frozen" ? e.startingXp : 64000 + i * 30000,
              targetLevel: e.state === "frozen" ? 0 : e.level,
              level: e.level,
              nextLevelXp: e.state === "frozen" ? null : 125000,
              currentFloor: e.state === "frozen" ? null : 64000,
            }));
            c.warnings = [
              ...(c.warnings ?? []),
              ...(opts.specialistRace
                ? [
                    {
                      code: "specialist_race",
                      field: "race",
                      class: first,
                      race: c.raceCode,
                    },
                  ]
                : []),
              ...(opts.dual
                ? []
                : [
                    {
                      code: "level_limit",
                      field: "level",
                      class: "thief",
                      value: 9,
                      max: 8,
                    },
                  ]),
            ];
            if (opts.pending)
              c.derived.levelUp.pending = entries
                .filter((e) => e.state === "active")
                .map((e) => ({
                  class: e.class,
                  classes: classes.length,
                  level: e.level + 1,
                  die: 8,
                  fixedGain: 0,
                  grants: {
                    weaponSlots: 0,
                    nonWeaponSlots: 0,
                    skillPoints: 0,
                    freeSpells: [],
                  },
                }));
          }),
  );
  if (opts.dual) return;
  await page.route(
    (url) => url.pathname === "/api/xp/preview",
    (route) => patched(route, (a) => (a.split = split(a.total, classes))),
  );
  await page.route(
    (url) => url.pathname === "/api/xp",
    (route) =>
      patched(route, (l) => {
        (l.entries ?? []).forEach(
          (e: Json) => (e.split = split(e.total, classes)),
        );
        // What the server serves for the active classes (xpsvc BonusClasses).
        l.bonusClasses = classes.map((c) => ({
          class: c,
          primes: PRIMES[c] ?? ["str"],
        }));
      }),
  );
  await page.route(
    (url) => url.pathname === "/api/log",
    (route) =>
      route.request().method() !== "GET"
        ? route.fallback()
        : patched(route, (l) =>
            (l.entries ?? []).forEach((e: Json) => {
              if (e.kind === "xp_add" && e.payload?.entry)
                e.payload.entry.split = split(e.payload.entry.total, classes);
            }),
          ),
  );
}

/**
 * Opens the class summary line's popover of the character card (by keyboard, as the line is a
 * button) and returns the popover, which holds one row per class entry. Escape closes it.
 */
export async function openClassDetail(page: Page): Promise<Locator> {
  const summary = page.getByTestId("class-summary");
  await summary.focus();
  await page.keyboard.press("Enter");
  const detail = page.getByTestId("class-entries");
  await detail.waitFor();
  return detail;
}
