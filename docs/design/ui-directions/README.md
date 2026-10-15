# Design directions: the screen around the focused activity

Three whole-screen directions for Critical!, as static pages built on the app's real theme tokens
(`shared/tokens.css` is generated from `web/src/index.css`; type roles from ADR 0017, role colours
and brand from ADR 0020) with the Bard Kelsen Balstriel at level 15, his real weapons, items and
spellbook, a turn in progress with three effects, a journal of sixteen entries and long spell names.
Plus four options for the brand mark. Nothing here is app code: the next item writes ADR 0022 from
the pick, then the shell item builds it.

Every page keeps the **last result, HP and the turn with its effects** visible in some form, and no
important text is cut: `just design-shots` fails when any element clips its text or a page scrolls.

Rebuild and screenshot: `just design-shots` (pages from `build.mjs` + `lib/`, screenshots at
1707x1300 and 1440x900 in dark Arcane into `<direction>/shots/`). Open any `<direction>/*.html`
in a browser to see it live (after `cd web && npm ci`, for the fonts).

## Where the directions differ

|                           | A · Vitals bar + stages                                                                         | B · Anchor column + focus follows touch                                                                                          | C · Mosaic: tiles that expand and collapse                                                                                       |
| ------------------------- | ----------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| Always visible            | One dense bar on top: turn + initiative + Next turn, effects timeline, HP with bar, last result | The anchor column on the left: HP (hero number), turn, effects list with Absorb, abilities, HP change, saves; the result band below | A one-row status strip: turn + Next turn, effect pills, HP mini bar, last result line                                             |
| How you switch            | Explicitly, with the rail: Act, Spells, Experience, Inventory                                   | You don't: the pane you touch widens, the other two fall back to a compact card beside it                                        | You don't: everything is on screen; a tile expands to the canvas (a companion can stay pinned), the rest collapse into a dock    |
| Act (weapons, spells…)    | A three-column stage: act panel, result + journal + dice, character                             | The wide pane; result, journal and dice in the band                                                                              | A tile in the overview; expanded with the Result tile pinned when more room is needed (Spells tab)                               |
| Spell lookup              | The whole stage: list beside stats and the description side by side                             | The wide pane: list and detail; Act and XP shrink to cards                                                                       | A tile with list + stats in the overview; expanded, stats and description side by side                                           |
| Experience                | The whole stage: progress, ledger and the form                                                  | The wide pane                                                                                                                    | A tile with the total and the bar; expanded, progress + form beside the ledger                                                   |
| Space sharing             | Vertical: the bar takes ~105 px, the stage takes everything else                                | Horizontal: 19.5 rem anchor, 16 rem compact column, the rest to the focused pane; the band is 15.5 rem tall (21 rem at 1300)    | A 2 × 3 grid at rest; expanded: all of it, minus a 2.5 rem dock                                                                  |
| What it costs             | A mode switch to look something up mid-fight; the journal is only in Act                        | The band keeps the journal short at 900 px; compact cards are summaries, not controls                                            | Six live tiles are busy at rest; the journal and result go to the dock while a tile is expanded                                  |
| Best at                   | Deep work in one activity (lookup, XP) with nothing in the way                                  | Flow: cast, look up, add XP without ever "going" anywhere; the character is one glance away                                      | Seeing everything at once on the 1707x1300 table screen; room on demand                                                          |
| Brand mark shown          | Facet (act pages), Impact (lookup and XP pages)                                                 | Monogram                                                                                                                         | Blade                                                                                                                            |

What all three keep from the v3 mockup and the triage: uppercase section labels, panels with a
visible outline on a surface, the HP bar with the hatched dying zone, threshold colours and the
last-damage ghost, one line per journal entry with Reroll on hover (shown on the free-roll row),
14–16 px panel padding, the 28 / 32 px control heights, no bigger buttons. Large numbers only where
the block is about that number (turn, HP, the damage total, the XP total): a `hero` size (40 px)
is used for the turn in A, HP in B and the XP total in A and C; ADR 0022 decides whether that
becomes a sixth role or stays `headline`.

## A · Vitals bar + stages

Screenshots: [act-weapons](a-vitals-bar/shots/act-weapons-1707x1300.png) ·
[act-spells](a-vitals-bar/shots/act-spells-1707x1300.png) ·
[lookup](a-vitals-bar/shots/lookup-1707x1300.png) · [xp](a-vitals-bar/shots/xp-1707x1300.png) ·
at 1440x900: [act-weapons](a-vitals-bar/shots/act-weapons-1440x900.png) ·
[act-spells](a-vitals-bar/shots/act-spells-1440x900.png) ·
[lookup](a-vitals-bar/shots/lookup-1440x900.png) · [xp](a-vitals-bar/shots/xp-1440x900.png).

The header's wasted band becomes the one place for everything that must never leave the screen,
packed: a `TURN 3` hero number with initiative and Next turn stacked beside it, the effects
timeline, HP with its bar, and the last result as a line (d20, hits AC, verdict, damage). The rail
stays, and each activity gets the whole width below. In Act the stage is three columns (act panel |
result card, journal, dice | character: abilities, HP change, charge card, saves). The lookup stage
is a filter + list column and a detail with stats and description side by side; the XP stage is
the progress across the top, the ledger and the form. At 1440 the wordmark drops to the mark alone
so the bar fits.

## B · Anchor column + focus follows touch

Screenshots: [act-weapons](b-focus-follows-touch/shots/act-weapons-1707x1300.png) ·
[act-spells](b-focus-follows-touch/shots/act-spells-1707x1300.png) ·
[lookup](b-focus-follows-touch/shots/lookup-1707x1300.png) ·
[xp](b-focus-follows-touch/shots/xp-1707x1300.png) · at 1440x900:
[act-weapons](b-focus-follows-touch/shots/act-weapons-1440x900.png) ·
[act-spells](b-focus-follows-touch/shots/act-spells-1440x900.png) ·
[lookup](b-focus-follows-touch/shots/lookup-1440x900.png) ·
[xp](b-focus-follows-touch/shots/xp-1440x900.png).

No rail and no modes. The character is a column on the left that never changes: brand, name, HP as
the hero number with its bar, the turn block, the effects as a list (with Absorb on the charge
effect), abilities, HP change, saves. The three activities are live panes: the one last touched
(a click, typing in its search box) is the wide one and gets a tint top edge; the other two stack
on the right as compact cards that still do something (Attack, the lookup list, Add XP). The result
card, the journal and the dice composer are one band across the bottom, under every activity: a
roll made while looking up a spell shows up without switching anything.

## C · Mosaic: tiles that expand and collapse

Screenshots: [act-weapons (overview)](c-mosaic/shots/act-weapons-1707x1300.png) ·
[act-spells (Act expanded, Result pinned)](c-mosaic/shots/act-spells-1707x1300.png) ·
[lookup (expanded)](c-mosaic/shots/lookup-1707x1300.png) ·
[xp (expanded)](c-mosaic/shots/xp-1707x1300.png) · at 1440x900:
[act-weapons](c-mosaic/shots/act-weapons-1440x900.png) ·
[act-spells](c-mosaic/shots/act-spells-1440x900.png) ·
[lookup](c-mosaic/shots/lookup-1440x900.png) · [xp](c-mosaic/shots/xp-1440x900.png).

A one-row status strip (turn, effects as pills, HP, last result) and six tiles that are all live at
rest: Act, Result, Character on the first row; Spell lookup, Journal (with the dice), Experience on
the second. Every tile has an expand button: the tile takes the whole canvas and the five others
become a dock of one-line summaries at the bottom (Act · Long Sword +2 · AC 5, Journal · Orc axe
−9…), each a button back. A tile can keep a companion pinned beside it: the Spells tab page shows
Act expanded with the Result pinned. The lookup tile shows the list and the stats at rest and opens
the description when expanded. This is the direction that uses the 1707x1300 table screen most:
nothing is hidden, and room is one click away.

## Brand mark options

Page: [logo/index.html](logo/index.html) · screenshot
[logo/shots/index-1707x1300.png](logo/shots/index-1707x1300.png) (and
[1440x900](logo/shots/index-1440x900.png)). Each option at header size with the Cinzel wordmark, at
32 and 16 px, in a browser tab, in dark and light Arcane; the current mark first for reference. The
SVGs use `--tint` / `--highlight` with static Arcane fallbacks, so one file serves the header and
the favicon.

| Option   | File                                   | Idea                                                                                                   |
| -------- | -------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Facet    | [logo/facet.svg](logo/facet.svg)       | A line-drawn icosahedron (the d20 from a vertex) in the primary; the front face lit in gold: the crit  |
| Blade    | [logo/blade.svg](logo/blade.svg)       | The "!" itself: a sword point-down in the primary, its dot the gold spark where the blade lands        |
| Impact   | [logo/impact.svg](logo/impact.svg)     | An eight-point burst in the primary carrying a bold gold "!": loud and still readable at 16 px        |
| Monogram | [logo/monogram.svg](logo/monogram.svg) | A heavy "C" in the primary opening onto a gold "!": C!, calm and typographic                           |

The name stays `Critical!` with its "!" in every language; the choice is independent of the
direction.

## Files

- `build.mjs` writes the pages, `shared/tokens.css` and `logo/*.svg`; `shots.mjs` builds, then
  screenshots and audits them. `lib/data.mjs` is the sample data, `lib/ui.mjs` the shared fragments,
  `lib/logos.mjs` the marks, `lib/direction-{a,b,c}.mjs` one module per direction (its layout CSS
  included), `lib/logo-page.mjs` the logo page.
- The generated `*.html`, `direction.css`, `shared/tokens.css`, `logo/*.svg` and `*/shots/*.png` are
  committed so the PR shows them; re-run `just design-shots` after a change to `lib/`.
- Spell descriptions are placeholder prose: the catalogue's text is never copied into this repo.
