# ADR 0026: Colour and depth: the contrast ladder, the raised stage and four themes

**Status**: accepted 2026-10-17 (user OK, proposals as written); written (critical-jwo3.1; the user OKs it on the PR). Amends
[ADR 0009](0009-dashboard-layout-and-theme-system.md) steps 4 and 6;
[ADR 0020](0020-visual-identity.md) steps 1 to 6; [ADR 0022](0022-activity-focus-layout.md) steps 2.4,
3.2, 4.1 and 4.2. Rechecks, without changing them, ADR 0022 step 2.8 and
[ADR 0017](0017-type-scale-and-density.md) step 6. Three points are **proposed, user OK on the PR**:
the themes' French and English names and ids (step 3.2), the default pair (step 3.3) and the mapping
of a saved theme that no longer exists (step 4.3). Step 5.9's initiative size amended by
[ADR 0027](0027-space-follows-frequency-of-use.md) step 4.2 (2026-10-17, proposed: still plain ink,
no chip, at `text-hero`, larger than the turn number).

## Context

The UI review ([`docs/design/ui-review/`](../design/ui-review/README.md)) found that "things don't
pop": tone steps too small to read (card on page ΔL .055, raised step on card .065 dark and .025
light), field edges at 1.6-1.9:1, one selection mark per component, three chip styles, and three of
the six ambiances (Grove, Blood, Amber) whose primary hue sits on a status hue. The mock-up round
([`docs/design/ui-review/mockups/`](../design/ui-review/mockups/README.md)) painted four options over
the running app, in English and French, and measured every theme
([`themes.md`](../design/ui-review/mockups/themes.md), from
`web/ui-review/mockup-tokens.ts`, removed since the themes shipped).

The user's pick, 2026-10-17:

- **"A3"**: option 1 (contrast ladder) + option 3 (raised stage) of the mock-up round.
- **"B2"**: themes **Arcane** (dark + light, retuned), **Deep** (dark only, from Ocean dark),
  **Parchment** (light only, from Amber light), **Plain** (from Obsidian, dark + light). Grove, Blood,
  Amber dark and Ocean light go.
- One-mode themes: **the user picks a dark theme and a light theme; the appearance mode (dark |
  light | system) chooses which applies; "system" follows the OS live** (answer "b"). Deep is offered
  only as a dark theme, Parchment only as a light theme, Arcane and Plain in both.
- Layout and contrast are checked on the French text too (French is the layout reference).

Two findings of the round shape the rules below. **WCAG ratios flatten dark tones**: a card at L .25
on a page at L .10 is an obvious step on screen and measures 1.28:1, so tone steps are judged by
their OKLab lightness difference. **The review's "every accent 1.8:1 off tint" cannot hold for
danger**: every accent must stay AA as text on the card, which bounds the dark ladder below at
L ≈ .62, so danger (L .74) and the primary (L .65) sit about 1.3:1 apart and red stands out by hue
and as a filled pill instead.

## Decision

The token values below are the measured ones of the mock-up round (option 4's theme set, which is
option 1's ladder plus Deep and Parchment), copied from `themes.md` (chroma already clamped into
sRGB). They are the starting values: the implementer may retune any of them **within the rules of
steps 1-3**, and the tests decide. Where a value of the round misses a rule, the rule wins.

1. **Contrast ladder tokens** (amends ADR 0020 steps 2 and 6).
   1. **Surfaces per mode**, four tones, judged by OKLab lightness L:

      | Tone (token)         | Role                                 | Arcane / Deep / Plain dark | Arcane / Plain light | Parchment light |
      | -------------------- | ------------------------------------ | -------------------------- | -------------------- | --------------- |
      | page (`ground`)      | the window behind the cards          | L .10                      | L .89                | L .865          |
      | card (`surface`)     | every zone of ADR 0022 step 4.1      | L .25                      | L .995               | L .97           |
      | raised (`neu-900`)   | tiles, rows, filled chips, save rows | L .335                     | L .915               | L .89           |
      | hover (`neu-800`)    | the hover of a raised row or chip    | L .405                     | L .885               | L .855          |
      | selected (`amb-800`) | a selected row, the chosen chip      | L .39                      | L .88                | L .88           |

      The selected tone is the theme's primary hue at low chroma; hover is the neutral one step past
      raised. Ink is L .95 in dark (Deep .95), .18 in light (Parchment .24, brown-black).

   2. **Translucent mixes** of ink (ADR 0020 step 6's 70 / 18 / 22 %): muted text / card outline
      (`--border`) / field edge (`--input`) = **76 / 32 / 42 %** in dark, **74 / 34 / 48 %** in light,
      **76 / 34 / 50 %** in Parchment.
   3. **Muted ink is for meta only**: timestamps, units, hints, a dimmed size. A value the player
      acts on (the "needs 6 vs AC 5" line, a save's target number, a roll total) is in full ink.
   4. **Accent ladder in dark mode**: every accent sits at its own lightness, in this order:
      **tint < danger < magic < success < warning < highlight** (< ink). Values: tint .71 (Arcane;
      Deep .70), danger .74, magic .78, success .82, warning .86, highlight .90. A near-neutral
      theme's grey tint (step 3.6, Plain .86) is not an accent and stays out of the order. In light
      mode the accents are dark on light (tint .40, magic .47, danger .50, success .485, highlight
      .50, warning .51 in Arcane and Plain; Parchment highlight .485, success .47, warning .49); no
      order is required there. `highlight-fill` stays the gold fill with dark text (L .82 light, =
      `highlight` dark). _Amended 2026-10-17 (the four themes shipped; the contrast test of step 2.3
      decided, hues and chromas unchanged); was: dark tint .65 (Deep too), light success .52,
      highlight .555, warning .565 in Arcane and Plain, Parchment highlight .53, success .50,
      warning .545, all under 4.5:1 as text on the raised step (Consequences); and no word on the
      near-neutral tint, which step 3.6 puts at .86, above every accent._
   5. The full role table per theme and mode (ground, surface, `neu-900`, `neu-800`, ink, tint,
      highlight, highlight-fill, magic, danger, success, warning, `amb-800`, mixes, ramps) is
      `THEMES` in `web/src/lib/theme.ts`. _Amended 2026-10-17; was: the "4 · Bespoke themes" table
      of [`themes.md`](../design/ui-review/mockups/themes.md) (the round's values, before step 1.4's
      retune), with `neu-800` and the Deep and Parchment ramps in `web/ui-review/mockup-tokens.ts`
      (`ladder`, `deep`, `parchment`), removed with the round's capture code._
   6. **Defaults before the app runs** (ADR 0020 step 6): `web/src/index.css` holds the Arcane light
      and dark blocks exactly as the theme code computes them, with the new mixes.

   _Enforced by_ `web/src/lib/theme.test.ts`: the "index.css defaults" test (step 1.6), and a ladder
   test that fails when a dark theme's accents leave the order of step 1.4 or a theme's tones leave
   the order page < card < raised (dark) / page < raised < card (light).

2. **Non-text contrast** (amends ADR 0020 step 4), for every theme in every mode it exists in:
   1. **card on page ΔL ≥ .10** and **raised on card ΔL ≥ .07**, as OKLab lightness differences
      (absolute: light-mode steps go down). Not as WCAG ratios, which flatten dark tones.
   2. The **field edge** (`--input` over the card) and the **focus ring / "latest" outline**
      (`tint` on the card) reach **3:1** on the card (WCAG 1.4.11).
   3. ADR 0020 step 4's text rules stand: ink and muted ink reach AA (4.5:1) on page, card and
      raised; every accent reaches 4.5:1 as text on the card and on the raised step; `on-accent` on
      every accent and `on-highlight` on `highlight-fill` reach 4.5:1; every accent stands 3:1 off
      the card as a fill (light `highlight-fill` exempt); every token is inside sRGB (chroma clamped,
      never lightness).
   4. **Not adopted**: the review's "every accent 1.8:1 off tint". Danger cannot meet it (Context);
      it stands out as a filled verdict pill (step 5.5) and by hue.
   5. The card outline has no floor: with the tone step it no longer carries the boundary alone.

   _Enforced by_ the "contrast" describe block of `theme.test.ts`, extended with the ΔL and 3:1
   pairs, run over every theme and mode of step 3.

3. **Four themes** (amends ADR 0009 step 4, ADR 0020 steps 1 and 3). The ambiance is now a
   **theme**: a hand-tuned role table per mode, not three hues fed to one generator.
   1. **The set**, each with its own role table and semantic hues (step 1.5):

      | Theme     | Modes       | From        | Character                                                                                                                                              |
      | --------- | ----------- | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
      | Arcane    | dark, light | Arcane      | the brand: violet-indigo primary (285), gold highlight (88), teal magic (200); retuned on the ladder                                                   |
      | Deep      | dark only   | Ocean dark  | ink-blue page, cyan-blue primary (232), coral danger (30), jade success (168), gold highlight (85), violet magic (300)                                 |
      | Parchment | light only  | Amber light | warm paper, brown-black ink, lapis primary (262), oxblood danger (27), moss success (140), ochre warning (60), gilt highlight (82), violet magic (310) |
      | Plain     | dark, light | Obsidian    | near-neutral: no colour but the semantic ones and the gold highlight                                                                                   |

      Grove, Blood, Amber dark and Ocean light are removed.

      _Amended 2026-10-17 (the new-themes round, approved by the user as shown;
      [`docs/design/new-themes/`](../design/new-themes/README.md)): three themes join the set, each on
      the same ladder, mixes and raised stage, role tables in `THEMES`. The set is now seven themes._

      | Theme  | Modes       | Character                                                                                                                                         |
      | ------ | ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
      | Forest | dark, light | forest-night surfaces, emerald primary (160), lime success (128, 32° off the primary), gold highlight (88), violet magic (300)                    |
      | Ember  | dark only   | wine surfaces (hue 12-15), crimson-rose primary (355, tint L .725 for 4.5:1 on the raised step), vermilion danger (40, 45° off), teal magic (200) |
      | Amber  | dark only   | warm black (60-65), amber ink (85) and primary (70), orange warning (42, 28° off), lemon highlight (105), teal-blue magic (210)                   |

      Ids `forest`, `ember`, `amber`; names **Forest / Forêt**, **Ember / Braise**, **Amber / Ambre**.
      The dark list is Arcane, Deep, Plain, Forest, Ember, Amber; the light list Arcane, Parchment,
      Plain, Forest. No touches of step 7 for them.

   2. **Ids and names** (_proposed, user OK on the PR_): ids `arcane`, `deep`, `parchment`, `plain`
      (stable, language-free); display names are i18n keys (en / fr): **Arcane / Arcane**,
      **Deep / Abysses**, **Parchment / Parchemin**, **Plain / Sobre**.
   3. **Default pair** (_proposed, user OK on the PR_): dark theme **Arcane**, light theme **Arcane**.
   4. **Semantic hues are per theme** (replaces ADR 0020 step 3's "the same hues in every
      ambiance"): Parchment's danger is its own oxblood (L .48, hue 27), not the dark red flipped;
      Deep's danger is coral (hue 30) and its success jade (168). Arcane and Plain keep danger 25 and
      success 150; their warning moves to the round's hue 65 dark / 62 light (was 55).
   5. **No primary on a status hue** (replaces Sang's "accepted exception" of ADR 0020 step 3, which
      goes with Blood): a theme's primary hue stays at least 25° from each of its own danger,
      success and warning hues. A near-neutral primary (chroma < 0.02) is exempt.
   6. **Near-neutral rule** (ADR 0009 step 4, ADR 0020 Consequences, restated for Plain): a theme
      whose primary chroma is under 0.02 highlights in grey (tint L .86 dark / .30 light), and keeps
      the gold secondary for the highlight.
   7. The light-mode ramp reversal of ADR 0009 step 4 stands for every theme that has ramps.

   _Enforced by_ `theme.test.ts`: every theme gives the same set of variables in each mode it
   exists in; the hue-distance rule of step 3.5; the near-neutral rule of step 3.6; and by the
   i18n key check for the eight names (en + fr). The e2e "theme" test of `layout.spec.ts`
   screenshots each theme in each of its modes, in French.

4. **Stored preference** (amends ADR 0009 step 6).
   1. The device stores **a dark theme and a light theme** (with the language and the appearance
      mode), replacing the single `amb` of `localStorage['critical.settings.v1']`; the stored shape,
      under that key or a new versioned one, is the implementer's. The dark choice
      is one of Arcane, Deep, Plain, Forest, Ember, Amber; the light choice one of Arcane, Parchment, Plain, Forest. _Amended
      2026-10-17 (step 4.6); was: the settings popover offers both choices._
   2. The appearance mode (dark | light | system) chooses which applies; "system" follows
      `prefers-color-scheme` live (unchanged). Switching mode never changes either stored choice.
   3. **A saved theme that no longer exists** is mapped **once at load and saved back**
      (_proposed, user OK on the PR_):

      | Saved `amb`   | Dark theme              | Light theme |
      | ------------- | ----------------------- | ----------- |
      | `ocean`       | Deep                    | Arcane      |
      | `ambre`       | Arcane                  | Parchment   |
      | `sylve`       | Arcane                  | Arcane      |
      | `sang`        | Arcane                  | Arcane      |
      | `obsidienne`  | Plain                   | Plain       |
      | `arcane`      | Arcane                  | Arcane      |
      | anything else | the defaults (step 3.3) |             |

      A stored dark or light choice that is not a theme of that mode falls back to that mode's
      default. The older `critical.theme` / `critical.language` keys keep their one-time migration.

   4. They stay per-device display preferences, never sent to the API.
   5. **The theme list follows the appearance** (user, 2026-10-17: "dark versus light, it's a bit
      confusing. When the appearance is dark, we should only list dark themes. Likewise with light
      and automatically when system is chosen."). The settings popover has one theme list, not two:
      Dark lists only themes with a dark mode, Light only those with a light mode, System the
      list of the OS's current mode, switching live when the OS does. Picking a theme saves it for
      the shown mode only (the dark or the light choice of step 4.1); switching the appearance
      shows and applies the other mode's remembered choice. A theme declares its modes by having a
      palette for them (`THEMES`); a new theme appears in the lists of the modes it has.
   6. **First paint** applies the effective theme's mode before the app runs (the inline script of
      ADR 0009's Consequences), so a Deep or Parchment user sees no flash of the other mode. It
      also paints the effective theme's variables, from a copy of the chosen dark and light themes'
      tokens the app saves on the device (`critical.paint.v1`), so the first frame is not the default
      theme either. _Amended 2026-10-17 (the copy added); was: the mode only._

   _Enforced by_ `web/src/lib/settings.test.ts`: one case per row of the table (mapped, then saved
   back without `amb`), a wrong-mode choice falling back, and the mode switch keeping both choices;
   the e2e "settings" test of `layout.spec.ts` picks a dark and a light theme and checks both
   survive a reload. Step 4.6: `settings.test.ts` (the list per appearance, system following the
   OS, a pick saving for the shown mode only) and the same e2e test (the list per appearance,
   Dark -> Light -> Dark keeping each pick).

5. **Component rules of the ladder** (amends ADR 0022 steps 2.4, 4.1 and 4.2).
   1. **One selection mark**: a selected row, item or rail entry is the **selected tone** (`amb-800`)
      with ink text and a **3 px tint bar** on its left edge. The rail uses it too (replaces step
      2.4's `bg-amb-900 text-tint` pill), as do the lookup list and the journal's selected entry.
   2. **One "latest" mark**: the result card (the latest result) has a **2 px tint outline**
      (replaces step 4.1's `border-amb-700`).
   3. **One chip family**: a chip is filled with the raised tone (`neu-900`) and no outline; a
      **chosen** chip (a segmented thumb, the chosen die) is the selected tone with a tint outline;
      the **highlight fill** (gold) is only for a crit or a maximum. Bonus and dice chips are
      neutral.
   4. **HP bar always semantic**: success above half, then warning, then danger (the thresholds of
      the HP bar stay as they are). Never the theme's primary.
   5. **The verdict is a filled pill**: Hit on `success`, Miss on `danger`, `on-accent` text. The
      header's last-result value chip takes the verdict's fill.
   6. **Card titles are section labels** (amends step 4.2): Journal, Items, Weapons, Effects and the
      other card titles take the section-label style (`text-label`, uppercase, 0.08em, full ink);
      `text-title` stays for names (the character, a spell, the weapon in hand).
   7. **Fields sit on the card**: an input, select or text area has the card's background and the
      field edge of step 2.2, never a darker "hole".
   8. **Gold in the journal only for crit or max**: a roll total is in ink; it is `highlight` only
      when the entry says it was a crit or a maximum. "Undone" is muted, not red.
   9. The weapon in hand sits on the selected tone with a 2 px tint outline; ability tiles and save
      rows are full raised steps (save rows filled, their target number in ink); initiative is a
      plain `text-key` number.

   _Enforced by_ `web/src/lib/hpBar.test.ts` (step 5.4: the fill's colour role per threshold, never
   `tint`; the fill classes are `bg-success`, `bg-warning`, `bg-danger`); the journal's value colour by a unit test of its formatter (step 5.8: gold only with a
   crit or max flag); the rest by the screenshots of the e2e "theme" test (each theme, each mode, FR)
   checked in review, and by `layout.spec`'s no-cut audit.

6. **Raised stage** (amends ADR 0022 steps 3.2, 4.1 and 4.2).
   1. **Three elevations**: the page, the cards, and **the focused card of the activity**, one tone
      up (dark: ink at 5 % over the card; light: the card itself) with a **2 px outline** (ink at
      40 %). The focused card is the act panel (Act), the lookup list (Spells), the Items card
      (Inventory) and the XP progress card (Experience).
   2. **A header band on every card**: a tone (ink at 9 % over the card in dark, 7 % in light),
      holding the card's label on the left and its controls on the right (the act panel's tabs, the
      result's kind / name / time, the journal's head, the inventory titles, the lookup filters, the
      spell's name).
   3. **Sub-bands for groups inside a card** (ink at 5.5 % dark, 4 % light): in the character card
      the identity, hit points, HP change and saving throws; in the header card the turn and the
      last result cells.
   4. **Bands are tones, not lines**: ADR 0022 step 4.1's "no rule or separator line" stands.
   5. **A band replaces the card's top padding** (amends step 3.2): the band runs to the card's
      edges and the card has no padding above it; the 14 / 16 px padding applies below the band.
   6. **Rechecked, unchanged**: ADR 0022 step 2.8's content target (at 1440x900, in French, the
      character card still shows the fifth save and the weapon in hand is inside the act panel) and
      ADR 0017 step 6 / ADR 0022 step 3.1's 120 px header at rest hold with the bands, as the round's
      shots show.

   _Enforced by_ `layout.spec`'s "shell" and "act panel" tests (step 6.6: the header budget, the
   content target and the no-cut audit, in French included) and the e2e "theme" screenshots for the
   bands and the focused card; and by the "raised stage" block of `theme.test.ts`, for every theme
   and mode: the tokens of steps 6.1-6.3 in `themeVars` and the `index.css` defaults, card ≤
   elevated (strictly in dark), card < sub-band < band towards the ink, and step 2.3's text rule on
   them: ink and muted ink AA on the elevated tone, the band (on the card and on the elevated tone)
   and the sub-band; every accent AA as text on the elevated tone, the band on the card and the
   sub-band. The act panel's band (the band on the elevated tone) holds only ink text and the chosen
   tab's tint mark, held to 3:1 (step 2.2); the dark tint reaches 4.1-4.2:1 there. _Added
   2026-10-17 (the raised stage shipped)._

7. **Parchment's touches** (amends ADR 0020 step 5), in Parchment only:
   1. **Section labels in Cinzel capitals** (`font-display`, bold, 0.06em), the card titles of step
      5.6 included. **Not on stat-tile labels** (the lookup's tiles): Cinzel's wider capitals cut
      "Temps d'incantation" in French. This is the second use of the display face, beside the
      wordmark.
   2. **A gilt outline** (`highlight`) on the latest result, instead of the tint one of step 5.2.
   3. **A visible brown outline on cards** (ink at 34 %): paper has little tone depth.

   _Enforced by_ the no-cut audit of `layout.spec` run on Parchment in French (step 7.1), and the
   Parchment screenshots of the e2e "theme" test.

## Consequences

- **Measured gaps to close in the implementation.** The round measured accents as text on the card,
  not on the raised and selected tones. A check of the same values (2026-10-17) finds accents under
  4.5:1 as text on the raised step: dark `tint` (Arcane 3.6, Deep 3.8) and light `highlight`,
  `success` and `warning` (3.7-4.1 in Arcane, Parchment and Plain). Step 2.3 keeps the rule, so the
  implementer retunes (an accent's lightness, or the raised step within ΔL ≥ .07) and the contrast
  test decides. Text on the selected tone is ink (step 5.1), so no accent is required as text there.
- **Crit vs warning**: ADR 0020 step 3 placed warning at hue 55 so a crit (gold, 88) is not read as a
  warning. The round moves warning to 65 / 62; the two now differ also by role (gold only for crit
  and max, step 5.8; warning on the HP bar) and, in dark, by lightness (.86 vs .90).
- ADR 0020 step 1's generator (three hues and a peak chroma per ambiance) gives way to a role table
  per theme (`THEMES` in `web/src/lib/theme.ts`); the nine-step ramps are still drawn from each
  table's three hues. A new theme is a role table, its modes, two i18n names per language and the
  tests of steps 2 and 3. _Amended 2026-10-17; was: Arcane and Plain can still be generated by the
  ladder (`ladder` in `mockup-tokens.ts`), Deep and Parchment hand-made (`deep`, `parchment`)._
- The settings popover changes: an appearance mode plus a dark-theme and a light-theme choice
  instead of six swatches. Its layout follows the layout ADR.
- Users of Grove, Blood or Ocean light lose their theme; the mapping of step 4.3 gives them the
  closest one once.
- The mock-up overlay (`docs/design/ui-review/mockups/css/`) is a drawing, not the rule: its
  selectors are test ids; the implementation uses role tokens and component classes.
- Not covered: a user-defined theme, a high-contrast mode, print; a dark Parchment or a light Deep;
  per-character themes; option 2 (neutral chrome), which stays possible later on top of this.
