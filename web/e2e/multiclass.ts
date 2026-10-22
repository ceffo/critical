import type { Page, Route } from "@playwright/test";

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
  /** One pending level per class. */
  pending?: boolean;
  /** A long first class name, to prove wrapping: the code, as the web has no wording for it. */
  longClass?: string;
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

/** The class codes sharing an award: the active entries. */
export function sharing(opts: MultiClassOptions): string[] {
  return opts.dual ? ["mage"] : [opts.longClass ?? "fighter", "thief"];
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
            c.derived.classes = entries.map((e) => ({
              class: e.class,
              state: e.state,
              xp: e.state === "frozen" ? e.startingXp : 64000,
              targetLevel: e.state === "frozen" ? 0 : e.level,
              level: e.level,
              nextLevelXp: e.state === "frozen" ? null : 125000,
            }));
            c.warnings = [
              ...(c.warnings ?? []),
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
      patched(route, (l) =>
        (l.entries ?? []).forEach(
          (e: Json) => (e.split = split(e.total, classes)),
        ),
      ),
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
