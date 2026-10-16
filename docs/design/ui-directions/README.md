# Design directions: the screen around the focused activity

Static pages for Critical!, built on the app's real theme tokens (`shared/tokens.css` is generated
from `web/src/index.css`; type roles from ADR 0017, role colours and brand from ADR 0020) with the
Bard Kelsen Balstriel at level 15, his real weapons, items and spellbook, a turn in progress with
three effects, a journal of sixteen entries and long spell names. Plus the brand-mark options.
Nothing here is app code: the next item writes ADR 0022 from the pick, then the shell item builds it.

**Round 4 (2026-10-15): B reviewed with the user and approved** ("YES. cards it is"). The feedback and
what each point changed is under "Round 4" below. **Round 3: direction B** combines the user's picks from round 2 (below). A2 and A3 stay
for comparison; A (baseline) and A1 (rejected: "cramped, too dense, no harmony") are deleted (git
history has them). The die silhouettes of round 2 are gone everywhere ("not looking good, abort");
die values are plain again. Every page keeps the last result, HP and the turn with its effects
visible, and no important text is cut: `just design-shots` fails when any element clips its text
(wide or tall) or a page overflows; `just design-shots --self-test` proves the audit catches those.

Rebuild and screenshot: `just design-shots` (pages from `build.mjs` + `lib/`, screenshots at
1707x1300 and 1440x900 in dark Arcane into `<direction>/shots/`). Open any `<direction>/*.html`
in a browser to see it live (after `cd web && npm ci`, for the fonts).

## B · Character left, history below

### Round 4: the user's feedback on round 3, applied (approved)

| Feedback (user, verbatim)                                                                                                                    | In B                                                                                                                                                                                                                                                 |
| -------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| "bare everywhere"                                                                                                                            | **Facet · Bare** is the mark, in every header (A2/A3 too) and as the favicon; the heavy take is deleted                                                                                                                                              |
| "[the 44 px Attack] ok but the icon is ugly, bring back the swords and center that icon hori&vert in the space left to the words"            | Attack stays 44 px, two lines; the icon is the app's Phosphor `Sword` in its own 44×44 cell, centred, left of the words                                                                                                                                |
| "damage and healing buttons are badly placed. they should be on the same line … prefer on one line. icon for damage is unreadable. i want the blood droplet icon that was in the mocks" | HP change is one line: `HP` input, Damage, Healing; Damage carries the v3 mock's Phosphor `Drop`, Healing `Heart`                                                                                                                                    |
| "hitpoints have no business up there. i want them between stats and hp change"                                                               | Hit points (value / max, last hit, bar with the dying zone) moved from the header to the character card, between the abilities and HP change                                                                                                        |
| "effects and round timeline next to initiative. last result summary docked to the right of the timeline"                                   | The header is one band: brand, `TURN 3`, initiative + Next turn, the 12-turn effects timeline beside them, the last-result line docked right. ~115 px tall (was ~195); the journal gained the difference                                            |
| "menu cog... don't like it. what if the logo is the menu cog?" then "putting the logo in a dropdown button... not a fan. maybe we put back the cog on the left of the dice footer" | The brand is a plain mark + wordmark; the settings cog is the first item of the dice footer                                                                                                                                                         |
| "journal: the result to the far right, not a good idea … can it be before the logo? keep the time as the last column"                        | Journal columns: turn · **value** · icon · entry · time (A3 too); same order in the collapsed bar                                                                                                                                                    |
| "act section. the name Act is superfluous, it's already in the selector. tabs need icons"                                                   | No "Act" title in the panel; the tabs carry Phosphor `Sword`, `Sparkle`, `Backpack`, `ListChecks`                                                                                                                                                    |
| "no separators between sections in the header … I don't like having line separations tbh" then "i really meant everywhere … i'd rather have borderless zones or cards" | Shown two looks (cards vs flat tones); the user picked **cards**: every zone is a card on the ground with one 12 px gutter (header, character, stage panels, dice footer), no rule anywhere; the rail is bare icons between the character card and the stage, the active one on a tint pill |

Decisions settled in the round: **Facet · Bare** everywhere; the **44 px Attack** stays (the one tall
control, with the app's Sword icon); **cards, not lines**: zones never touch, so nothing needs a
separator. The icons on the pages that matter for the pick (Attack, Damage, Healing, the act tabs)
are the app's own Phosphor glyphs, read from `web/node_modules/@phosphor-icons/react` at build time
(`lib/phosphor.mjs`); the rest of the small icons are still the pages' stroke sketches.

### Round 3: what the user kept from round 2, and where it went

| Pick (user, round 2)                                                      | In B                                                                                                                                                  |
| ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| A2: "I like having the character on the left … on the border with the selector right to it" | The character is a column on the left screen edge (name, abilities, HP change, Stoneskin, saves as click-to-roll buttons), the activity rail right of it, in **every** activity, so the rail never moves |
| A2: "the settings cogs could then move top right"                        | Round 3 put the cog at the right of the header; round 4 moved it to the left of the dice footer (the user's call); the rail has none                   |
| A3: "not the header though, we lose too much information"               | Round 3: A2's header over the timeline. Round 4: one band, brand · turn · initiative · the 12-turn timeline · last result; HP went to the character card |
| A3: "the journal on the bottom and the dice roller as a footer"          | In Act the journal is the lower band, a full-width table (turn \| entry \| time \| value); the dice composer is a footer across the whole window, in every activity |
| A3: "provided we can drag the separator … or even collapse it"           | A grip between the "now" band and the journal (`split-handle`, cursor row-resize) with a Collapse button; the collapsed state is a one-line bar with the last entry, the count and Undo/Redo (`act-spells-collapsed`) |
| A2: "the weapon in hand section is still ridiculously wasting space … the attack button looks small" | The weapon in hand is one row: name, then its stats as a line of text (`Melee · 1d8 S/M · 1d12 L · +2 / +2 · THAC0 11`); the Attack button is the only control in the row, taller (44 px, two lines: "Attack" / "needs 6 vs AC 5") and primary; the other weapons are chips under it (one click to switch), no dropdown |
| A2: "spell lookup pane is better than A1"                                | A2's lookup: filter + list on the left, the detail as stat tiles, a cast box and the description in two text columns                                 |
| XP (no pick)                                                              | A2's workspace: progress and the form across the top, the ledger below                                                                                |

Where two picks fought, the choice and why:

- **Result beside the action, not under it.** A2 put the result under the act panel (cause over
  effect); A3 put the journal below. Both stacked would leave the journal ~9 rows at 900 px tall,
  so the "now" band is act panel | result side by side, and the band takes only its content's
  height (capped at 60 % of the stage, the act panel scrolls past that: Spells tab); the journal
  gets the rest, at least 14rem. With the Weapons tab at 1707x1300 the whole journal shows.
- **The character column is permanent chrome** (a card since round 4), not an Act panel as in A2: with the rail right of
  it, the rail would jump between activities otherwise, and HP change and saves are needed from the
  lookup too. It costs the lookup and XP stages 280 px of width; the lookup detail still fits five
  stat tiles and two text columns at 1440.
- **A taller Attack button** despite triage's "no bigger buttons": the user's complaint was the
  ratio of button to section. The section shrank to its content first (no stat tiles, no empty
  gradient); the button grew once, to 44 px, because it carries a second line (what it needs to
  hit) and is the one action of the tab. Every other control keeps the 28 / 32 px heights.
- **The dice footer is app-wide**, not Act-only as in A3: a footer that comes and goes is not a
  footer, and a free roll is wanted from the lookup too (a spell's damage). It costs 44 px in
  every activity.
- **At 1440 the Weapons target row wraps** (Situational on a second line) and the last weapon chip
  wraps under the others; nothing is cut. The act panel has ~80 px to spare under the chips at
  1707; that is the result card's height with its calculation open, not the weapon section's.

Pages: `act-weapons`, `act-spells`, `act-spells-collapsed` (journal collapsed, the band takes the
height), `lookup`, `xp`.

## A2 and A3, kept for comparison

**A2 · Timeline + centre stage.** A thin command row over the effects timeline at full width. In
Act the action and its result share the centre column, the character is a narrow left column and
the journal a full-height right column with the dice composer at its foot. Lookup detail as stat
tiles, a cast box and the description in two columns; XP as progress + form over the ledger.

**A3 · Now and history.** One 44 px strip with the activities as tabs. Act is two bands: "now" (act
panel, result with its calculation, character) and the journal as a full-width table with the dice
composer as one line under it. Lookup as a table with stat columns; XP as the ledger with the total,
form and thresholds in a side column.

## Comparison

|                        | A2 · Timeline + centre stage                                                 | A3 · Now and history                                                              | B · Character left, history below                                                                    |
| ---------------------- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Header                 | Command row + effects track (12 cols), ~195 px                               | One 44 px strip: tabs, turn, effect pills, HP mini bar, last result               | One card, ~115 px: brand, turn, initiative, the 12-col track, last result docked right               |
| How you switch         | Rail                                                                         | Tabs in the strip                                                                 | Bare icon rail between the character card and the stage                                              |
| Act                    | character \| act panel, result under it \| journal + dice                    | now: act \| result \| character; history: journal table, dice as one line         | now: act \| result; journal table below with a drag handle (collapsible); dice footer                 |
| Spell lookup           | list \| stat tiles, cast box, description in two columns                     | table with stat columns \| stats, cast box, description                           | A2's                                                                                                 |
| Experience             | progress + form on top, ledger below                                         | ledger \| total, form, thresholds                                                 | A2's                                                                                                 |
| Where the character is | A narrow column in Act                                                       | A column in the "now" band                                                        | A column on the left edge, in every activity                                                         |
| Where the journal is   | Its own column in Act                                                        | The whole lower band in Act, as a table                                           | The lower band in Act, resizable, collapsible to one line                                            |
| Weapon in hand         | Card with four stat tiles, Attack top right                                  | Same card                                                                         | One row: stats as text, a 44 px Attack (Sword icon) with "needs 6 vs AC 5", other weapons as chips   |
| Zones                  | Surfaces with rules between them                                             | Surfaces with rules between them                                                  | Cards on the ground, one gutter, no rules                                                            |
| What it costs          | The journal column is narrow at 1440                                         | At 900 px tall the "now" panels scroll; the journal shows ~9 rows                 | 300 px of width in every activity; at 900 px tall the journal shows ~8 rows unless collapsed/dragged |
| Brand mark shown       | Facet · Bare                                                                 | Facet · Bare                                                                      | Facet · Bare                                                                                         |

All three keep from the v3 mockup and the triage: uppercase section labels, panels with a visible
outline on a surface, the HP bar with the hatched dying zone, threshold colours and the last-damage
ghost, one line per journal entry with Reroll on hover (shown on the free-roll row), 14–16 px panel
padding, the 28 / 32 px control heights. Large numbers only where the block is about that number
(turn, HP, the damage total, the XP total): a `hero` size (40 px) beyond ADR 0017's five roles;
ADR 0022 decides whether that becomes a sixth role or stays `headline`.

## Screenshots

| Direction                          | 1707x1300                                                                                                                                                                                                                                                                                                                                                                                                | 1440x900                                                                                                                                                                                                                                                                                                                                                                                      |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| B · Character left, history below  | [act-weapons](b-character-left/shots/act-weapons-1707x1300.png) · [act-spells](b-character-left/shots/act-spells-1707x1300.png) · [act-spells-collapsed](b-character-left/shots/act-spells-collapsed-1707x1300.png) · [lookup](b-character-left/shots/lookup-1707x1300.png) · [xp](b-character-left/shots/xp-1707x1300.png)                                                                            | [act-weapons](b-character-left/shots/act-weapons-1440x900.png) · [act-spells](b-character-left/shots/act-spells-1440x900.png) · [act-spells-collapsed](b-character-left/shots/act-spells-collapsed-1440x900.png) · [lookup](b-character-left/shots/lookup-1440x900.png) · [xp](b-character-left/shots/xp-1440x900.png)                                                                        |
| A2 · Timeline + centre stage       | [act-weapons](a2-centre-stage/shots/act-weapons-1707x1300.png) · [act-spells](a2-centre-stage/shots/act-spells-1707x1300.png) · [lookup](a2-centre-stage/shots/lookup-1707x1300.png) · [xp](a2-centre-stage/shots/xp-1707x1300.png)                                                                                                                                                                      | [act-weapons](a2-centre-stage/shots/act-weapons-1440x900.png) · [act-spells](a2-centre-stage/shots/act-spells-1440x900.png) · [lookup](a2-centre-stage/shots/lookup-1440x900.png) · [xp](a2-centre-stage/shots/xp-1440x900.png)                                                                                                                                                              |
| A3 · Now and history               | [act-weapons](a3-now-history/shots/act-weapons-1707x1300.png) · [act-spells](a3-now-history/shots/act-spells-1707x1300.png) · [lookup](a3-now-history/shots/lookup-1707x1300.png) · [xp](a3-now-history/shots/xp-1707x1300.png)                                                                                                                                                                          | [act-weapons](a3-now-history/shots/act-weapons-1440x900.png) · [act-spells](a3-now-history/shots/act-spells-1440x900.png) · [lookup](a3-now-history/shots/lookup-1440x900.png) · [xp](a3-now-history/shots/xp-1440x900.png)                                                                                                                                                                  |

## Brand mark: Facet · Bare (decided)

Page: [logo/index.html](logo/index.html) · screenshot
[logo/shots/index-1707x1300.png](logo/shots/index-1707x1300.png) (and
[1440x900](logo/shots/index-1440x900.png)). Each option at 64 px, at header size with the Cinzel
wordmark, at 32 and 16 px, in a browser tab, in dark and light Arcane. The SVGs use `--tint` /
`--highlight` with static Arcane fallbacks, so one file serves the header and the favicon.

Round 2's pick: "the first one but drop the triangle". The page order was Current, Spine, V,
Solid, Crit, so "the first one" is read as **Facet · Spine**, the first option after the current
mark (Current itself has the triangle the user called out in round 1). Round 3 drew Spine without
its front face in two weights; round 4 decided: **Facet · Bare everywhere** ("bare everywhere"), one
file for the header and the favicon. The heavier take is deleted (git history has it).

| Option              | File                                                     | Idea                                                                                                    |
| ------------------- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| Facet · Bare        | [logo/facet-bare.svg](logo/facet-bare.svg)               | Spine's strokes exactly (edge 2, stem 3), the "!" centred on the hexagon's axis. **The mark.**         |

Shown next to Current and Spine for reference. Every direction's header carries Bare.
The name stays `Critical!` with its "!" in every language.

## Files

- `build.mjs` writes the pages, `shared/tokens.css` and `logo/*.svg`; `shots.mjs` builds, then
  screenshots and audits them (`--self-test` checks the audit on a page with known clipping).
  `lib/data.mjs` is the sample data, `lib/ui.mjs` the shared fragments, `lib/phosphor.mjs` the
  app's Phosphor icons (read from `web/node_modules`, so `cd web && npm ci` first), `lib/logos.mjs` the marks,
  `lib/direction-{a2,a3,b}.mjs` one module per direction (its layout CSS included),
  `lib/logo-page.mjs` the logo page.
- The generated `*.html`, `direction.css`, `shared/tokens.css`, `logo/*.svg` and `*/shots/*.png` are
  committed so the PR shows them; re-run `just design-shots` after a change to `lib/`.
- Spell descriptions are placeholder prose: the catalogue's text is never copied into this repo.
