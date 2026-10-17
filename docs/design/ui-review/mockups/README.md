# Mock-up round: four ways to make the UI pop (2026-10-17)

The options of the [UI review](../README.md)'s "Recommendations", built as asked in its "Which to
pick": **1 alone, 1 + 2, 1 + 3, and 4's Parchment**. Each one is shown on the same three screens as
today: Act · Weapons after an attack, the spell lookup on Fireball, and the inventory (the screen
where section separation shows best: two cards, row stripes, and the card-title vs section-label
clash). Each screen is at 1707x1300 and 1440x900, in dark and light mode. **Pick one** (or a
combination, see "My ranking"); a developer then writes the ADR amendment and builds it.

**Where to look first:** open [`index.html`](index.html) in a browser. It shows any option beside
today on the same screen, theme and viewport, and keys ← → switch options. On GitHub, use the
tables below.

## How these were made (and why they are trustworthy)

- **The real app, not a drawing.** `just ui-mockups` starts the app on the e2e stack (the
  `api/testdata` fixture, in-memory API, never `.dev/` or the dev ports), seeds one table (turn 1,
  three effects, damage taken and healed, an undone "Dragon breath", an attack as the latest result),
  then paints each option over the running app. The overlay has two parts: the option's **tokens**
  ([`web/ui-review/mockup-tokens.ts`](../../../../web/ui-review/mockup-tokens.ts)) and its
  **component rules** ([`css/`](css/)). "Today" is the same run without the overlay, so every pair of
  shots shows identical data, down to the dice. Roll values differ from the review's shots because
  this is a new run.
- **No text cut.** Every capture, in English and French, goes through the no-cut audit of ADR 0022
  step 1.3, and the run fails on any cut. It caught one problem along the way: Parchment's display
  face cut "Temps d'incantation" in a lookup tile, so that face is now kept off the tile labels.
- **Measured, not eyeballed.** Each option's theme set is in [`themes.md`](themes.md) (generated),
  with every token value and the WCAG ratio of every text, edge and accent pair, and as a
  sheet of swatches (`shots/<option>--themes.webp`).
- **App code untouched.** `web/src` has not changed. Shots are lossless WebP (identical pixels, 40 %
  of the PNG weight).

One finding changes how the review's numbers read. **WCAG ratios flatten dark tones.** A card at
L .25 on a page at L .10 is an obvious step on screen but measures only 1.28:1. So the theme sheets
judge tone steps by their OKLab lightness difference instead: ΔL ≥ .10 for a card on the page and
≥ .07 for a raised block on a card. Today's values are .055 and .065 dark, .055 and .025 light.

**The review's "every accent 1.8:1 off tint" rule can't hold for danger.** Every accent must stay
AA (4.5:1) as text on the card, so in dark mode none can go below L ≈ .62. Danger at L .74 and the
primary at .65 then sit only about 1.3:1 apart, so red can only stand out by hue. The options make
it stand out as a **filled pill** (the verdict), and direction 2 also removes the primary from
everything that isn't an action.

## The options at a glance

| | Idea in one line | Themes kept | Cost | ADR steps it amends |
|---|---|---|---|---|
| **1 · Contrast ladder** | Same design, made visible: darker page, lighter cards, one raised step, accents on a lightness ladder, one selection mark, one chip family | Arcane, Ocean, Obsidian (both modes); **drops** Grove, Blood, Amber (their primary sits on a status hue, which a token change cannot fix) | **small** | 0020 steps 2, 4, 6; 0022 step 4.1 (2 px "latest" outline, one raised tone), 4.2 (section labels for card titles too), 2.4 (rail pill); 0009 step 4 (ambiance list) |
| **1 + 2 · Neutral chrome** | 1, plus colour only where it means something: chrome, selection and tracks go grey; the ambiance hue keeps only the mark, the focus ring and the primary actions (Attack, Cast, Roll, Next turn) | **all six**, both modes: Grove and Blood work again because their hue no longer paints bars and values | **medium** | 1's steps, plus 0020 step 1 (the primary palette's role), step 2 (`neu-*` as chrome and selection), step 3 (Sang's exception no longer needed); 0009 step 4 (near-neutral rule generalised) |
| **1 + 3 · Raised stage** | 1, plus structure: the card you act in rises a step with a 2 px outline, and every card and group gets a header band (a tone, not a line) | as 1 | **medium** | 1's steps, plus 0022 step 3.2 (bands replace top padding), 4.1 (a third, elevated tone; bands), 4.2 (labels in bands); 2.8 (content target at 1440 rechecked: it holds in the shots) |
| **4 · Bespoke themes** | 1, with three hand-designed themes instead of six generated ones: **Parchment** (light, from Amber light), **Deep** (dark, from Ocean), Arcane kept, Obsidian as **Plain** | Arcane (both modes), Deep (dark only), Parchment (light only), Plain (both modes); **drops** Grove, Blood, Amber dark, Ocean light | **large** | 1's steps, plus 0009 steps 4 and 6 (theme list; a theme that exists in one mode only), 0020 steps 1 to 3 (role table per theme, semantic hues per theme), step 5 (Cinzel in section labels, Parchment only) |

What "cost" covers (an estimate, from the overlay's size and the components it touches):
- **small**: new values in `themeVars` and the `index.css` defaults, three new non-text checks in
  `theme.test.ts`, and about 15 class changes (the HP bar, save rows, the verdict pill, selection,
  chips, headings, inputs on the surface).
- **medium**: 1, plus about 40 uses of `amb-*` / `tint` moved to a neutral role (direction 2), or a
  card-header band shared by about 8 cards, with the 1440 height budget rechecked in `layout.spec`
  (direction 3).
- **large**: 1, plus per-theme role tables, new theme ids, a migration of the stored preference for
  Grove, Blood and Ocean users, the settings popover, the i18n names, per-theme contrast tests, and a
  rule for themes that exist in only one mode.

## 1 · Contrast ladder

| Today | Option 1 |
|---|---|
| ![](shots/today--act-weapons--arcane-dark-1707x1300.webp) | ![](shots/o1-ladder--act-weapons--arcane-dark-1707x1300.webp) |
| ![](shots/today--act-weapons--arcane-light-1440x900.webp) | ![](shots/o1-ladder--act-weapons--arcane-light-1440x900.webp) |
| ![](shots/today--inventory--arcane-dark-1440x900.webp) | ![](shots/o1-ladder--inventory--arcane-dark-1440x900.webp) |

- **Tokens** (dark / light): page L .10 / .89, card .25 / .995, raised step .335 / .915, a
  `neu-800` hover / selected step. Muted ink 76 / 74 % (meta only), outlines 32 / 34 %, field edges
  42 / 48 % (3.6 / 3.3:1, was 1.9 / 1.6). Accents in dark: tint .65 < danger .74 < magic .78 <
  success .82 < warning .86 < highlight .90.
- **Header**: initiative as a plain `text-key` number (no chip); the last result's value chip filled
  in the verdict colour; the brand mark's stroke heavier.
- **Character card**: the HP bar is green when healthy (it was the theme colour: red in Blood);
  ability tiles a full raised step; saves are filled rows with the target number in ink.
- **Weapons**: weapon in hand on the selected tone with a 2 px tint outline; "needs 6 vs AC 5" at
  full strength; weapon chips filled (one chip family); fields on the card, not darker than it.
- **Result card**: a 2 px tint outline marks "latest"; the verdict is a filled pill ("Hit" green,
  "Miss" red).
- **Journal**: roll totals in ink (gold now means crit / max only); "Undone" muted, not red; the
  selected row is the selected tone plus a 3 px tint bar, and the same mark is used in the rail and
  the lookup.
- **Lookup, inventory**: one raised step for tiles and rows; card titles as section labels; the
  inventory cards outlined like every other card (they had none); the bonus and dice chips neutral.

## 1 + 2 · Neutral chrome

| Today | Option 1 + 2 |
|---|---|
| ![](shots/today--act-weapons--arcane-dark-1707x1300.webp) | ![](shots/o12-neutral-chrome--act-weapons--arcane-dark-1707x1300.webp) |
| ![](shots/today--act-weapons--arcane-light-1440x900.webp) | ![](shots/o12-neutral-chrome--act-weapons--arcane-light-1440x900.webp) |
| ![](shots/today--lookup--arcane-dark-1440x900.webp) | ![](shots/o12-neutral-chrome--lookup--arcane-dark-1440x900.webp) |
| Blood today: [`../shots/dark-blood--act-weapons-1440x900.png`](../shots/dark-blood--act-weapons-1440x900.png) | ![](shots/o12-neutral-chrome--act-weapons--sang-dark-1440x900.webp) |

- Everything in 1, plus: the primary ramp is painted as the neutral one, so selected rows, the
  rail pill, segmented thumbs, the weapon in hand, the current turn and the charge card become grey
  steps by themselves.
- **Colour left on screen**: the four primary actions (Attack, Roll, Next turn; Cast on the Spells
  tab), the brand mark, the focus ring, verdict pills, HP (green / warning / red), damage and healing
  values, gold for the total and crits, and the magic hue for spell levels and effect pills.
- **Result card**: the d20 hexagon in ink (the verdict pill carries the colour); "latest" is a 2 px
  ink outline.
- **Tabs, the timeline's current column, level progress, scrollbars**: ink or grey.
- **Inventory**: the "+2" bonus is gold text, no chip.
- **Trade-off**: less brand on every screen; the ambiance becomes the colour of the mark and of
  three buttons. The Blood shot is the proof: red now means danger, and only the actions are crimson.

## 1 + 3 · Raised stage

| Today | Option 1 + 3 |
|---|---|
| ![](shots/today--act-weapons--arcane-dark-1440x900.webp) | ![](shots/o13-raised-stage--act-weapons--arcane-dark-1440x900.webp) |
| ![](shots/today--act-weapons--arcane-light-1440x900.webp) | ![](shots/o13-raised-stage--act-weapons--arcane-light-1440x900.webp) |
| ![](shots/today--lookup--arcane-dark-1707x1300.webp) | ![](shots/o13-raised-stage--lookup--arcane-dark-1707x1300.webp) |
| French, the longest copy | ![](shots/o13-raised-stage--act-weapons--arcane-dark-fr-1440x900.webp) |

- Everything in 1, plus three elevation levels: the page, the cards, and the card being acted in
  (the act panel, the spell list, the Items card), which is one tone up with a 2 px outline.
- **Bands**: the act panel's tabs, the result card's kind / name / time, the journal's head, the
  inventory titles, the lookup filters and the spell's name each sit in a header band (ink at 9 %
  over the card). In the character card, the name, HP, HP change and saves each get a sub-band.
- **Header**: the turn block and the last result become banded cells (the brand and the timeline
  are unchanged; the review's "four banded cells" would need markup changes).
- **Height**: the bands take a few pixels per card (the result card's and the journal's heads grow, the
  tabs lose their top padding). At 1440x900 in French the character card still
  shows the fifth save, and the weapon in hand is fully inside the act panel (ADR 0022 step 2.8). The
  audit passes.
- **Trade-off**: more visual weight, closest to a classic VTT. Light mode's grey bands on white read
  well because the page is darker now.

## 4 · Bespoke themes: Parchment and Deep

| Today | Option 4 |
|---|---|
| ![](shots/today--act-weapons--arcane-light-1707x1300.webp) | ![](shots/o4-bespoke--act-weapons--parchment-light-1707x1300.webp) |
| ![](shots/today--lookup--arcane-light-1440x900.webp) | ![](shots/o4-bespoke--lookup--parchment-light-1440x900.webp) |
| ![](shots/today--act-weapons--arcane-dark-1440x900.webp) | ![](shots/o4-bespoke--act-weapons--deep-dark-1440x900.webp) |
| | Arcane, retuned: ![](shots/o4-bespoke--act-weapons--arcane-dark-1440x900.webp) |

- **Parchment** (light): warm paper (page L .865, cards .97 cream, raised .89), brown-black ink, a
  **lapis** primary (blue ink is far from every status hue; a brown primary would sit on danger or
  warning), oxblood danger, moss success, ochre warning, gilt highlight, violet magic. Section labels
  in Cinzel capitals, a gilt outline on the latest result, a visible brown outline on cards.
- **Deep** (dark): ink-blue page, a cyan-blue primary, coral danger, jade success, gold highlight,
  violet magic, on the same lightness ladder as 1.
- **Arcane** is kept and retuned (it is 1's Arcane); Obsidian becomes **Plain**. Dark mode is
  captured in Deep, light mode in Parchment.
- **Trade-off**: the most identity ("this is Dungeons & Dragons, not a spreadsheet"), and the most
  work. Users of Grove, Blood or Ocean light lose their theme; Parchment has no dark mode and Deep no
  light mode, which the settings popover would have to express.

## Theme sets

Each sheet shows every theme of the option in its own colours: swatches, the measured ratio of
every text pair, the fills, the raised step and field edge, and the accent distances (`✗` under the
floor). Dropped themes are dimmed. The numbers are in [`themes.md`](themes.md).

| Today | 1 · ladder (also 1 + 3) | 1 + 2 · neutral chrome | 4 · bespoke |
|---|---|---|---|
| ![](shots/today--themes.webp) | ![](shots/o1-ladder--themes.webp) | ![](shots/o12-neutral-chrome--themes.webp) | ![](shots/o4-bespoke--themes.webp) |

## My ranking

1. **1 + 2, neutral chrome.** It answers the most of what you said with the least new structure.
   "Things don't pop": against grey chrome, the green Hit pill, the gold total, the HP bar and the
   one Attack button are the only colours, so they are what you see first. "Inconsistencies
   everywhere": one chrome, one selection mark, one chip family. "One or two themes are usable":
   all six become usable, because the hue no longer competes with red, green and gold. Its weak
   point is "sections are hard to see", which it improves only as much as 1 does.
2. **1 + 3, raised stage.** The strongest answer to "sections are hard to see": every card and
   group has a visible head, and the card you act in stands out. But the violet stays on everything,
   Grove and Blood stay broken, and the screen gets heavier. The bands are independent of 2's rules,
   so **1 + 2 + 3** is possible later: add the character card's sub-bands and the card header bands
   if 1 + 2 still feels flat.
3. **4, bespoke themes.** Parchment is the most beautiful screen of the round, and the one that
   looks like the game. It costs the most, drops four themes, and still needs 1 underneath.
   Parchment could also be added later as one more theme on top of 1 + 2.
4. **1 alone.** It is the floor every other option stands on, and it is visibly better than today.
   But it keeps the "violet on everything" problem and drops three themes, so it is the weakest
   answer to the colour complaint.

Whatever you pick, 1 comes with it: its tokens and its component fixes are in every option.

## What the mock-ups do not show

- **Structure that needs markup**: the attack card's stat tiles (the review's "as for the save"),
  the header as four banded cells, an Apply button that doesn't look disabled, and spell vs mundane
  effect pills (CSS can't tell them apart, so every pill is drawn as a spell).
- **Screens not captured**: the Spells / Items / Skills tabs (Cast as a 28 px tint-outlined button),
  Experience, the popovers. The rules are written to apply there too, but they weren't checked.
- **Gold only on crits in the journal**: the journal can't tell a crit from a plain roll, so every
  roll total is ink in the mock-ups. The real change would keep gold on crit and max rows.
- **The ambiance picker** in the settings popover is unchanged in every option.

## Regenerating

```sh
just ui-mockups   # captures every option (~1.5 min), audits EN + FR for cut text, writes themes.md,
                  # theme sheets and shots/ (lossless WebP; needs ImageMagick's `magick`)
```

Change an option in `web/ui-review/mockup-tokens.ts` (tokens, theme sets) or in `css/<name>.css`
(component rules), then rerun. The full capture, including the French shots of every option, stays in
the gitignored `web/e2e-results/ui-mockups/`.
