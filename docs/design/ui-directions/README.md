# Design directions: the screen around the focused activity

Static pages for Critical!, built on the app's real theme tokens (`shared/tokens.css` is generated
from `web/src/index.css`; type roles from ADR 0017, role colours and brand from ADR 0020) with the
Bard Kelsen Balstriel at level 15, his real weapons, items and spellbook, a turn in progress with
three effects, a journal of sixteen entries and long spell names. Plus the brand-mark options.
Nothing here is app code: the next item writes ADR 0022 from the pick, then the shell item builds it.

Round 2 (2026-10-15): the user kept **A** (vitals bar + stages) and dropped B and C. A is the
**baseline** here; **A1, A2 and A3** keep A's foundations (an always-visible vitals bar with turn,
initiative, effects, HP and the last result; explicit activity switching; each activity owning the
width) and each pushes further from today's app in a different way. Every page keeps the last
result, HP and the turn with its effects visible, and no important text is cut: `just design-shots`
fails when any element clips its text (wide or tall) or a page overflows; `just design-shots
--self-test` proves the audit catches those.

Rebuild and screenshot: `just design-shots` (pages from `build.mjs` + `lib/`, screenshots at
1707x1300 and 1440x900 in dark Arcane into `<direction>/shots/`). Open any `<direction>/*.html`
in a browser to see it live (after `cd web && npm ci`, for the fonts).

## How each direction differs from today's app and from A

**Today's app**: a header band (brand, turn, initiative, Next turn, effects timeline) spread thin;
a rail; in Act three columns (act panel | result + journal + dice | character); the spell lookup
and XP squeezed into the left column while the result, journal and character stay.

**A (baseline)** moved HP and the last result into a dense vitals bar and gave each activity the
whole width, but the Act stage is still the same three columns as today, and the lookup and XP
stages are the old panels made wider.

**A1 · Character strip.** The vitals bar gains a second row that *is* the character: abilities,
HP change (amount, Damage, Healing, source), the five saves as click-to-roll buttons; the Stoneskin
Absorb button moves onto its effect row in the timeline. The character column is gone, so the Act
stage is two wide columns: the act panel (the Spells tab in three columns of levels) and the result
with its calculation open, the journal and the dice. The lookup is a faceted browser: class, level
and school as vertical lists with counts, the results as wide rows with school, range and duration,
the detail with stats, a cast box and the description. XP: progress and the form stacked on the
left, the ledger as the big panel. From v3: the effects track, the HP bar with the ghost of the
last hit, the saves as buttons, the result card's d20 and headline. Differs from today and from A
by removing a column, not adding one: the character becomes chrome, the activity gets the width.

**A2 · Timeline + centre stage.** The vitals bar is a thin command row (brand, `TURN 3`,
initiative, Next turn, HP with its bar, the last result line) over the effects timeline at full
width with twelve turn columns: the v3 mockup's "clock", where turns and effects are one concept.
In Act the action and its result share the centre column, cause over effect (the result card sits
right under the weapon that produced it and stays visible when the Spells tab scrolls); the
character is a narrow left column (abilities, HP change, Stoneskin, saves as buttons) and the
journal a full-height right column with the dice composer at its foot. The lookup detail is the
hero: the stats as a row of tiles like the weapon card's, a cast box, the description in two text
columns. XP is the mockup's workspace: progress and the form across the top, the ledger below.
From v3: the `TOUR` hero and the 12-column track, the result's hex d20 and `Touche CA` headline,
the saves as rows that roll, the level chips. Differs from today and from A by re-grouping: the
result leaves the journal column and joins the action; the timeline leaves the header's corner and
takes the width it needs.

**A3 · Now and history.** The vitals bar is one 44 px strip with the activities as tabs instead of
a rail (brand, Act · Spells · Experience · Inventory, turn + initiative + Next turn, effect pills,
HP with a mini bar, the last result). The Act stage is two bands: **now**, with the act panel, the
result with its calculation and the character side by side, and **history**, the journal as a
full-width table (turn | entry | time | value) with the dice composer as one line under it. The
lookup is a table with the stats as columns (level, school, range, duration, casting, save) so
spells compare at a glance, beside the detail. XP is the ledger as the hero with the total, the
form and the thresholds in a side column. From v3: the calmer journal rows and the composer folded
under them, the ledger table, dense section labels. Differs from today and from A by density and
grouping: no rail, no column for the journal; time runs down the screen and the whole width is a
ledger.

Every direction also shows the **die silhouettes** the user asked for during round 2: a rolled
value sits inside the shape of its die (d4 triangle, d6 square, d8 rhombus, d10 kite, d12 pentagon,
d20 hexagon; d% is a d10), so "Initiative 4" needs no "(d10)". They appear in the result card (the
d20 and the damage die), the initiative, the journal values and the dice composer's buttons.

## Comparison

|                        | A · Vitals bar + stages (baseline)                                            | A1 · Character strip                                                                            | A2 · Timeline + centre stage                                                                    | A3 · Now and history                                                                             |
| ---------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Vitals bar             | One row, ~105 px: turn, effects track (8 cols), HP, last result               | Two rows, ~130 px: the same, plus abilities, HP change, saves (click to roll)                    | A thin command row + the effects track at full width (12 cols), ~150 px                         | One 44 px strip: tabs, turn, effect pills, HP mini bar, last result                              |
| How you switch         | Rail                                                                          | Rail                                                                                            | Rail                                                                                            | Tabs in the strip                                                                                |
| Act                    | act panel \| result + journal + dice \| character                             | act panel (Spells in 3 columns) \| result (calculation open) + journal + dice                    | character \| act panel, result under it \| journal + dice                                       | now: act \| result \| character; history: journal table at full width, dice as one line          |
| Spell lookup           | list \| stats and description side by side                                    | facets \| wide rows (school, range, duration) \| stats, cast box, description                   | list \| stat tiles, cast box, description in two columns                                        | table with stat columns \| stats, cast box, description                                          |
| Experience             | progress on top, ledger \| form                                               | progress + form \| ledger                                                                       | progress + form on top, ledger below                                                            | ledger \| total, form, thresholds                                                                |
| Where the character is | A column in Act only                                                          | In the bar, always                                                                              | A narrow column in Act                                                                          | A column in the "now" band                                                                       |
| Where the journal is   | Under the result, in Act                                                      | Under the result, in Act (wider)                                                                | Its own column in Act                                                                           | The whole lower band in Act, as a table                                                          |
| What it costs          | Act is still three columns                                                    | A taller bar; saves and HP change are small controls in chrome                                  | The tallest bar; the journal column is narrow at 1440                                           | At 900 px tall the "now" panels scroll; the journal shows ~9 rows                                |
| Best at                | Deep work in lookup and XP                                                    | The act and the journal at full width; hits and saves without leaving the bar                   | Reading the fight as time; seeing the roll you just made under the thing you rolled             | The 1707x1300 table screen: everything about the turn in one band, the whole history below       |
| Brand mark shown       | Facet · Crit                                                                  | Facet · Spine                                                                                   | Facet · Solid                                                                                   | Facet · V                                                                                        |

All four keep from the v3 mockup and the triage: uppercase section labels, panels with a visible
outline on a surface, the HP bar with the hatched dying zone, threshold colours and the last-damage
ghost, one line per journal entry with Reroll on hover (shown on the free-roll row), 14–16 px panel
padding, the 28 / 32 px control heights, no bigger buttons. Large numbers only where the block is
about that number (turn, HP, the damage total, the XP total): a `hero` size (40 px) beyond ADR
0017's five roles; ADR 0022 decides whether that becomes a sixth role or stays `headline`.

## Screenshots

| Direction                   | 1707x1300                                                                                                                                                                                                                                                                                                                         | 1440x900                                                                                                                                                                                                                                                                                                                    |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A · Vitals bar + stages     | [act-weapons](a-vitals-bar/shots/act-weapons-1707x1300.png) · [act-spells](a-vitals-bar/shots/act-spells-1707x1300.png) · [lookup](a-vitals-bar/shots/lookup-1707x1300.png) · [xp](a-vitals-bar/shots/xp-1707x1300.png)                                                                                                         | [act-weapons](a-vitals-bar/shots/act-weapons-1440x900.png) · [act-spells](a-vitals-bar/shots/act-spells-1440x900.png) · [lookup](a-vitals-bar/shots/lookup-1440x900.png) · [xp](a-vitals-bar/shots/xp-1440x900.png)                                                                                                         |
| A1 · Character strip        | [act-weapons](a1-character-strip/shots/act-weapons-1707x1300.png) · [act-spells](a1-character-strip/shots/act-spells-1707x1300.png) · [lookup](a1-character-strip/shots/lookup-1707x1300.png) · [xp](a1-character-strip/shots/xp-1707x1300.png)                                                                                 | [act-weapons](a1-character-strip/shots/act-weapons-1440x900.png) · [act-spells](a1-character-strip/shots/act-spells-1440x900.png) · [lookup](a1-character-strip/shots/lookup-1440x900.png) · [xp](a1-character-strip/shots/xp-1440x900.png)                                                                                 |
| A2 · Timeline + centre stage | [act-weapons](a2-centre-stage/shots/act-weapons-1707x1300.png) · [act-spells](a2-centre-stage/shots/act-spells-1707x1300.png) · [lookup](a2-centre-stage/shots/lookup-1707x1300.png) · [xp](a2-centre-stage/shots/xp-1707x1300.png)                                                                                             | [act-weapons](a2-centre-stage/shots/act-weapons-1440x900.png) · [act-spells](a2-centre-stage/shots/act-spells-1440x900.png) · [lookup](a2-centre-stage/shots/lookup-1440x900.png) · [xp](a2-centre-stage/shots/xp-1440x900.png)                                                                                             |
| A3 · Now and history        | [act-weapons](a3-now-history/shots/act-weapons-1707x1300.png) · [act-spells](a3-now-history/shots/act-spells-1707x1300.png) · [lookup](a3-now-history/shots/lookup-1707x1300.png) · [xp](a3-now-history/shots/xp-1707x1300.png)                                                                                                 | [act-weapons](a3-now-history/shots/act-weapons-1440x900.png) · [act-spells](a3-now-history/shots/act-spells-1440x900.png) · [lookup](a3-now-history/shots/lookup-1440x900.png) · [xp](a3-now-history/shots/xp-1440x900.png)                                                                                                 |

## Brand mark: four developments of Facet

Page: [logo/index.html](logo/index.html) · screenshot
[logo/shots/index-1707x1300.png](logo/shots/index-1707x1300.png) (and
[1440x900](logo/shots/index-1440x900.png)). Each option at 64 px, at header size with the Cinzel
wordmark, at 32 and 16 px, in a browser tab, in dark and light Arcane; the current mark first for
reference. The SVGs use `--tint` / `--highlight` with static Arcane fallbacks, so one file serves
the header and the favicon.

Round 1's Facet was "interesting but undecipherable", and the current mark's triangle "has zero
harmony with the exclamation mark". All four variants share one geometry: a hexagon of radius 13.5
(the d20 seen from a vertex) and its front face, the triangle on the alternate vertices with the
apex down. The "!" sits on the die's vertical axis, inside the face, and its dot lands on the apex
vertex, so the die's edges, the stem and the dot are one construction; the inner spokes that
muddied round 1 are gone. Stroke weight is 2 for every die edge and 3 for the stem.

| Option         | File                                           | Idea                                                                                                   |
| -------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Facet · Spine  | [logo/facet-spine.svg](logo/facet-spine.svg)   | Outline hexagon and front face at one stroke; the gold "!" down the face's axis, dot on the apex       |
| Facet · V      | [logo/facet-v.svg](logo/facet-v.svg)           | The face open at the top: a V that frames a tapered gold wedge "!" with a small facet for its dot      |
| Facet · Solid  | [logo/facet-solid.svg](logo/facet-solid.svg)   | The die filled, the face cut as dark edges, the gold "!" inside: the heaviest at 16 px                 |
| Facet · Crit   | [logo/facet-crit.svg](logo/facet-crit.svg)     | The face lit in gold (round 1's crit face) with the "!" cut out of it in the dark                      |

The name stays `Critical!` with its "!" in every language; the choice is independent of the
direction.

## Files

- `build.mjs` writes the pages, `shared/tokens.css` and `logo/*.svg`; `shots.mjs` builds, then
  screenshots and audits them (`--self-test` checks the audit on a page with known clipping).
  `lib/data.mjs` is the sample data, `lib/ui.mjs` the shared fragments, `lib/logos.mjs` the marks,
  `lib/direction-{a,a1,a2,a3}.mjs` one module per direction (its layout CSS included),
  `lib/logo-page.mjs` the logo page.
- The generated `*.html`, `direction.css`, `shared/tokens.css`, `logo/*.svg` and `*/shots/*.png` are
  committed so the PR shows them; re-run `just design-shots` after a change to `lib/`.
- Spell descriptions are placeholder prose: the catalogue's text is never copied into this repo.
