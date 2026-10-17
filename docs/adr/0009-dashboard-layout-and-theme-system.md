# ADR 0009: Dashboard layout and theme system

**Status**: accepted, 2026-10-15; steps 3 and 5 amended by [ADR 0017](0017-type-scale-and-density.md);
step 4 replaced and step 5 amended by [ADR 0020](0020-visual-identity.md); steps 1 and 2 replaced and
steps 5 and 6 amended by [ADR 0022](0022-activity-focus-layout.md); steps 4 and 6 amended by
[ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) (2026-10-17, proposed: four themes, a dark and a light theme per device)

## Context

The v3 design handoff (Nocturne design system, `.local/mockups/design_handoff_critical_dashboard/`,
not in the repo) draws the combat dashboard at a fixed 1440 x 900 with fixed column widths
(rail 68, Fil 400, Moi 320 px) and asks for a pixel copy. The user wants it responsive instead: it
must look right on a laptop and on a large high-density screen, using the whole width. Mobile is
not a target yet. The handoff also brings six colour ambiances, light / dark / system appearance,
Inter and Phosphor icons, and its own visual conventions, some of which (tiny tracked capitals,
hairline panels) the house rules of the design-taste-frontend skill reject.

## Decision

1. **Regions.** *Replaced by [ADR 0022](0022-activity-focus-layout.md) step 2 (cards: header, character card, rail,
   stage, dice footer; no Fil or Moi column).* One screen, no page scroll. A full-width header region (round, buffs, later the
   turn timeline) sits above a grid of four columns:
   `rail | workspace | Fil | Moi` =
   `clamp(68px, 5vw, 96px) | minmax(0, 1fr) | clamp(340px, 26vw, 520px) | clamp(300px, 22vw, 440px)`.
   Each column scrolls on its own (`min-h-0` and its own `overflow-y-auto`, or an inner scroller).
2. **Workspaces.** *Replaced by [ADR 0022](0022-activity-focus-layout.md) step 2.4-2.5 (activities Act, Spells,
   Experience, Inventory; the mount-once-and-keep rule stands).* The rail switches the workspace column only: Agir (act: attack, spells, saves
   tabs), Inventaire (items and weapons), Sorts (spell lookup); Expérience comes later. Fil (last
   result, journal) and Moi (identity, HP) stay beside every workspace. A workspace mounts on its
   first visit and stays mounted while hidden, so its state (tab, forms, scroll) survives switching.
3. **Viewport range.** The layout holds from 1280 to 2560 CSS px wide at full width (no max-width
   centring) and has no horizontal scroll down to 1024. Below 1024 and mobile layouts are out of
   scope. *Amended by [ADR 0017](0017-type-scale-and-density.md) step 1:* the root font size is a
   fixed 16 px (it was `clamp(16px, 0.3125vw + 12px, 20px)`); extra width goes to columns and
   content, not to bigger type.
4. **Tokens.** *Replaced by [ADR 0020](0020-visual-identity.md) steps 1-4 (three palettes per
   ambiance, role tokens, semantic colours, contrast); the ambiance ids, the light-mode ramp
   reversal and the near-neutral rule stand.* *Amended by [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) step 3 (2026-10-17): the
   ambiance list becomes four themes (Arcane, Deep, Parchment, Plain), some in one mode only; the
   ramp reversal and the near-neutral rule (Plain) stand.* The theme is an appearance mode (`dark | light | system`) and an ambiance (hue H,
   chroma C): Arcane 289 / 0.125 (default), Sylve 152 / 0.11, Sang 22 / 0.15, Ocean 240 / 0.11,
   Ambre 72 / 0.12, Obsidienne 265 / 0.014. `web/src/lib/theme.ts` `themeVars` computes:
   - accent ramp `--amb-100..900` = `oklch(L C*F H)` with L
     `[.97 .93 .87 .78 .68 .58 .48 .38 .27]` and F `[.15 .3 .55 .85 1 .95 .8 .6 .4]`;
   - neutral ramp `--neu-100..900`, L `[.97 .93 .87 .77 .66 .56 .46 .37 .28]`, a faint tint of H;
   - `--ground` (page), `--surface` (panels), `--ink` (text) and `--tint` (the accent), all tinted
     with H;
   - in light mode both ramps are reversed (100 is the darkest), so `amb-900` is always a quiet
     tint and `amb-300` a readable highlight;
   - state colours (danger, heal, warning) are `oklch(lA + lB * L, C, H)` with `lA, lB = 0, 1` in
     dark and `1.3, -1` in light, so they keep a readable lightness in both;
   - a near-neutral ambiance (C < 0.02) highlights in grey, not colour.
   The variables are set inline on `<html>`; `web/src/index.css` maps them onto the shadcn variables
   and the Tailwind v4 `@theme` tokens (`bg-surface`, `text-amb-300`, `bg-danger`, …), so every
   component follows. `index.css` holds the Arcane values as the first-paint default (a unit test
   keeps them equal to `themeVars`).
5. **House rules (design-taste-frontend, adapted to Nocturne).** No gradients (a flat accent tint
   instead). No tiny uppercase letter-spaced labels: sentence case (*replaced by [ADR 0022](0022-activity-focus-layout.md) step 4.2:* section labels
   are uppercase and letter-spaced; *amended by
   [ADR 0017](0017-type-scale-and-density.md) step 2:* labels are 11 px, the 12 px minimum is gone;
   sizes come from its five role tokens). Surfaces are
   separated by a tone step (ground → surface → `neu-900`), not hairlines (*amended by [ADR 0022](0022-activity-focus-layout.md)
   step 4.1:* every zone is an outlined card on the ground, 12 px apart, with no rule anywhere). Shadows only on what
   floats (popovers, menus: `shadow-float`). Tabular figures on every number (set on `<html>`). At
   most 3-4 type sizes per panel. One family (Inter) and one icon set (Phosphor). Existing panels
   adopt these when they are restyled; the shell follows them now.
6. **Preferences.** Language, appearance and ambiance are per-device display preferences, chosen
   in the settings popover (gear at the bottom of the rail; *amended by [ADR 0022](0022-activity-focus-layout.md) step 2.6:*
   the cog is first in the dice footer), applied live and stored in
   `localStorage['critical.settings.v1'] = {lang, mode, amb}` (`web/src/lib/settings.ts`; *amended by
   [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) step 4 (2026-10-17):* a dark theme and a light theme instead of
   `amb`, the mode choosing which applies; a saved ambiance that no longer exists is mapped once). Without
   it, the older `critical.theme` / `critical.language` keys give the initial value once and are
   removed. "System" follows `prefers-color-scheme` live. They are never sent to the API. Anything
   that is game state (dice favourites, the attack form's last choices) lives on the server.

## Consequences

- Before the app starts, an inline script in `index.html` sets `.dark` (and `lang`) from the saved
  settings, so there is no light flash; a non-default ambiance shows its colours when the app's
  first module runs (Arcane values until then).
- Hidden workspaces stay in the DOM once visited: tests that locate by label or text scope to the
  visible region (role locators skip hidden elements).
- The panels moved into the regions unchanged; each later item restyles one region against these
  rules. Ambiance ids are the design system's names (`sylve`, `obsidienne`, …); their display
  names are i18n keys.
- Not covered: mobile and narrow layouts, user-defined ambiances, per-character preferences.
