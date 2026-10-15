# ADR 0020: Visual identity: Critical! palettes, roles and brand mark

**Status**: accepted, 2026-10-15 (critical-a02; extends [ADR 0009](0009-dashboard-layout-and-theme-system.md)
step 4 and amends its step 5; builds on [ADR 0017](0017-type-scale-and-density.md))

## Context

User, 2026-10-15: _"the UI lacks contrast a lot"_, _"primary palette for main elements, secondary for
accents and highlights, maybe a tertiary"_, _"green and red must translate correctly within the
theme palette, play their role and stand out"_, _"This is Dungeons and Dragons!! Not your
accountant's spreadsheet"_, and the app name, `Critical!`, appeared nowhere. ADR 0009 gave one
accent ramp per ambiance at low chroma, pale surfaces, muted text at 64% and state colours derived
by a lightness flip; the result was a low-contrast lavender with no brand.

## Decision

1. **Three palettes per ambiance.** An ambiance (`web/src/lib/theme.ts` `AMBIANCES`) names three
   hues with a peak chroma: **primary** (chrome, selection, actions: buttons, rings, tabs, the rail,
   bars), **secondary** (highlights: the brand "!", the number a roll is about, a maximum die, a
   crit, XP and bonuses) and **tertiary** (magic: spell levels, slots, spell dice). Each gives a
   nine-step ramp (`amb-*` primary, `sec-*`, `ter-*`; 100 lightest in dark mode, reversed in light
   mode as in ADR 0009) from one lightness curve `[.96 .91 .84 .76 .68 .58 .47 .36 .26]` and one
   chroma curve `[.2 .4 .65 .9 1 1 .9 .75 .55]` x peak chroma. The neutral ramp `neu-*` is a faint
   tint of the primary. Arcane, the default and the brand: primary 285 / 0.17 (violet-indigo),
   secondary 88 / 0.15 (gold), tertiary 200 / 0.12 (teal). Sylve 152 / 70 / 300, Sang 15 / 85 /
   250, Ocean 240 / 75 / 330, Ambre 72 / 300 / 200, Obsidienne 265 (0.014) / 88 / 200 (0.07).
2. **Roles, not ramps, in components.** Components use the role tokens and the shadcn names mapped
   onto them; a ramp step is for surfaces and soft fills (`bg-amb-900` a selected row, `bg-neu-900`
   a raised step), never for text that carries meaning.

   | Role                                      | Token                                  | Dark (Arcane)          | Light (Arcane)         |
   | ----------------------------------------- | -------------------------------------- | ---------------------- | ---------------------- |
   | Page, panel, raised step                  | `ground`, `surface`, `neu-900`         | L .15 / .205 / .27     | L .93 / .985 / .90     |
   | Text, muted text                          | `ink`, `ink-muted` (ink at 70%)        | L .95                  | L .20                  |
   | Primary accent (`--primary`, `--ring`)    | `tint`                                 | L .72, C ≤ .14         | L .46, C ≤ .20         |
   | Secondary accent as text                  | `highlight`                            | L .82, C ≤ .15         | L .53, C ≤ .11         |
   | Secondary accent as a fill                | `highlight-fill` + `on-highlight`      | = highlight, dark text | L .80 gold, dark text  |
   | Tertiary accent                           | `magic`                                | L .78, C ≤ .12         | L .48, C ≤ .10         |
   | Text on an accent fill                    | `on-accent`                            | L .15 (dark ink)       | L .985 (near white)    |

   A fill is an accent with `on-accent` text (`bg-primary text-primary-foreground`,
   `bg-success text-on-accent`); a soft fill is the accent at an opacity (`bg-danger/15`).
3. **Semantic colours, the same hues in every ambiance.** `danger` 25, `success` 150 (was `heal`;
   the token is renamed), `warning` 55, each with a lightness per mode (dark .72 / .78 / .78, light
   .52 / .50 / .54) and a chroma above the ramps', so red and green stand out from any primary.
   Secondary gold sits at hue 88 and warning at 55 so a crit is never read as a warning. Sang's
   crimson primary is the accepted exception: red next to red; its danger still differs by chroma
   and lightness.
4. **Contrast, enforced by a unit test** (`web/src/lib/theme.test.ts`, "contrast"), for every
   ambiance in both modes: `ink` and `ink-muted` reach WCAG AA (4.5:1) on `ground`, `surface` and
   `neu-900`; every accent (`tint`, `highlight`, `magic`, `danger`, `success`, `warning`) reaches
   4.5:1 as text on `surface` and `neu-900`; `on-accent` reaches 4.5:1 on every accent and
   `on-highlight` on `highlight-fill`; every accent stands 3:1 off `surface` as a fill. Every token
   is inside the sRGB gamut: `themeVars` clamps chroma (never lightness) per step, so a hue that
   can't carry its peak chroma at a given lightness loses saturation, not contrast. The light-mode
   `highlight-fill` is exempt from the 3:1 fill rule: it is identified by its dark text.
5. **Brand.** The app is `Critical!`, exclamation mark included, in every language. The mark
   (`web/src/components/shell/BrandMark.tsx`) is a d20 face in `tint` with the "!" in `highlight`,
   the same drawing as `web/public/favicon.svg` (static Arcane colours there); the wordmark is set
   in Cinzel (`font-display`, `@fontsource-variable/cinzel`), `text-key`, with the "!" in
   `highlight`. It sits at the left of the header region, before the turn block, in the 112 px
   header budget of ADR 0017 step 6; the page title is `Critical!`. The display face is for the
   wordmark only: all other text stays Inter (ADR 0009 step 5) at the ADR 0017 roles.
6. **Defaults before the app runs.** `web/src/index.css` holds the Arcane light (`:root`) and dark
   (`.dark`) blocks exactly as `themeVars` computes them (the "index.css defaults" unit test);
   `--border` is ink at 18%, `--input` at 22%, `--ink-muted` at 70%.

## Consequences

- ADR 0009 step 4 (one accent ramp, lightness-flipped state colours, `--lA` / `--lB`) is replaced
  by steps 1-4 here; its ambiance ids, the light-mode ramp reversal and the near-neutral rule
  (Obsidienne highlights in grey, but keeps a gold secondary) stand. ADR 0009 step 5 gains the
  one exception to "one family": the Cinzel wordmark.
- `text-heal` / `bg-heal` became `text-success` / `bg-success`; `text-ground` on a fill became
  `text-on-accent`. New classes: `text-highlight`, `bg-highlight-fill text-on-highlight`,
  `text-magic`, `bg-magic`, `sec-*`, `ter-*`, `font-display`.
- A new ambiance is three hues in `AMBIANCES` and six i18n names; the contrast test tells whether
  it is viable before anyone looks at it.
- The e2e "theme" test (`web/e2e/layout.spec.ts`) screenshots the main screens (Act/Weapons after
  an attack, Spells, Inventory, Experience, with the journal) in dark and light, then every
  ambiance: `web/e2e-results/screens/layout/{dark,light}-{act-weapons,spells,inventory,experience}.png`
  and `{dark,light}-<ambiance>.png`.
- Not covered: a user-defined ambiance, high-contrast mode, print.
