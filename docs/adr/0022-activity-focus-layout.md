# ADR 0022: The screen around the focused activity: cards, no cut text and the Facet mark

**Status**: accepted, 2026-10-15 (critical-uwtx.3, from the design approved in critical-uwtx.2). Replaces
[ADR 0009](0009-dashboard-layout-and-theme-system.md) steps 1 and 2 and amends its steps 5 and 6;
amends [ADR 0017](0017-type-scale-and-density.md) steps 2, 4, 5, 6 and 7; amends
[ADR 0020](0020-visual-identity.md) step 5. Steps 2.4, 3.2, 4.1 and 4.2 amended, step 2.8 rechecked (unchanged), by
[ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) (2026-10-17, proposed: one selection mark, the "latest" outline, card
titles as section labels, the raised stage and its header bands). Steps 2.2, 2.3, 2.5, 2.6, 2.7, 3.4,
5.1, 5.3 and 7 and step 4.2's example label amended by [ADR 0027](0027-space-follows-frequency-of-use.md)
(2026-10-17, proposed: space follows frequency of use; the header's stacked turn, initiative and full
brand, a wider character card, the now band fitting its tab instead of the 60 % cap, the previous
weapon and a drop-down instead of weapon chips, the result card, the compact journal, the Skills tab,
the lookup filters, Inventory and Experience, the dice footer).

## Context

The dashboard of ADR 0009 gives every region a fixed column (rail | workspace | Fil | Moi) whatever
the player is doing, and the columns are narrow enough to cut names. The user, in triage
(2026-10-15):

- _"There should not be text that is cut and ellipsed because of bad layout decision. never ever
  ever. it's a big tell that the layout is wrong."_ Refined: _"important text, given reasonable width
  of the canevas should not be cut."_
- The last result, HP and the turn with its effects are always visible; the rest reshapes around the
  focused activity (act with the dice and the journal, spell lookup, XP), and those are not always
  mutually exclusive.
- Spell names in the UI language only. Journal: one line per entry, the Note feature removed, Reroll
  on hover. Bring back section labels, panel outlines and contrast, the HP bar's dying zone and
  thresholds, 14-16 px padding. No bigger buttons.
- _"the critical logo is ugly"_.

Whole-screen directions were then drawn as static pages on the app's real tokens and data
([docs/design/ui-directions/](../design/ui-directions/README.md)). On round 2 the user said: _"I like
having the character on the left. it should even be on the border with the selector right to it"_;
of A3, _"the layout with the journal on the bottom and the dice roller as a footer is really nice.
provided we can drag the separator with the journal to change its size or even collapse it"_ and
_"not the header though, we loose too much information"_; _"the weapon in hand section is still
ridiculously wasting space. the attack button looks small compared to the huge section with empty
space"_; _"a2 spell lookup pane is better than a1"_; for the logo _"the first one but drop the
triangle"_; and _"the dice silhouettes are not looking good. abort that idea"_.

Direction B combined those picks. On round 4 the user reviewed it live and asked for: _"bare
everywhere"_ (the logo); of the 44 px Attack, _"ok but the icon is ugly, bring back the swords and
center that icon hori&vert in the space left to the words"_; _"damage and healing buttons … on the
same line … i want the blood droplet icon that was in the mocks"_; _"hitpoints have no business up
there. i want them between stats and hp change"_; _"effects and round timeline next to initiative.
last result summary docked to the right of the timeline"_; of a menu in the logo, _"not a fan. maybe
we put back the cog on the left of the dice footer"_; _"journal: the result to the far right, not a
good idea … before the logo? keep the time as the last column"_; _"the name Act is superfluous, it's
already in the selector. tabs need icons"_; and _"no separators … i really meant everywhere. i'd
rather have borderless zones or cards"_. Shown cards and flat zones side by side, the user approved:
_"YES. cards it is"_.

The drawing is [direction B](../design/ui-directions/README.md#b--character-left-history-below)
(`docs/design/ui-directions/b-character-left/`, screenshots at 1707x1300 and 1440x900). Where this ADR
states a value the user did not speak to (widths, the 60 % cap, the hero size, the label case), it is
B's choice and is marked "(design)".

## Decision

1. **No cut text.** At every layout viewport (1280x800, 1440x900, 1707x1300, 1920x1080, 2560x1440),
   important text is shown in full: names (spell, weapon, item, effect, character, ambiance), values
   and numbers, journal text, labels, button and tab captions, column headers.
   1. It wraps, or the layout gives it room. `truncate`, `text-ellipsis`, `line-clamp` and a
      `whitespace-nowrap` that pushes text out of a box that hides its overflow are refused on it.
      Text inside a box that scrolls (a list, the journal, a spell description) is not cut: the box
      scrolls.
   2. Exceptions, each with its reason; in each the full text is reachable without a hover:
      - **The dice favourites strip** in the footer (step 2.6) is one line; when the favourites
        outgrow it, the strip scrolls sideways (each chip stays whole). Reason: the footer is one
        line in every activity.
      - **An effect pill running past the timeline window** (ADR 0017 step 6) fades out at the
        window's edge. Reason: the pill is a duration drawn on a time axis; its caption (turns left)
        sits inside the visible part, and the effect's name is in its own column.
      - **The header's last result** (step 2.2) is one short line in a column of at most a sixth of
        the viewport; a longer line ends with an ellipsis and its full wording is the tooltip
        (*amended 2026-10-17, user: "way shorter than that", "restricted to like ... the sixth of
        the screen"*). Reason: it is a glance at the newest roll; the full text (round, time,
        details) is the same entry's journal row and result card, reachable without a hover.
   3. Checked by a Playwright helper, `expectNoClippedText(region)` in `web/e2e/`, the same audit as
      `docs/design/ui-directions/shots.mjs`: it fails when an element inside the region has
      `scrollWidth > clientWidth + 1`, or hides its vertical overflow and has
      `scrollHeight > clientHeight + 1`, and lists the clipped elements. `layout.spec` calls it on the
      whole page in each activity at every layout viewport, with long names in the fixture. The review
      checklist (`docs/agent-workflow.md`) carries the rule.
2. **Regions and activity focus.** One screen, no page scroll. The window is the ground, padded
   12 px; every zone is a card on it (step 4), 12 px apart:

   ```text
   header card (full width)
   character card | rail | stage (the focused activity)
   dice footer card (full width)
   ```

   1. **Always visible**, in every activity: the **header card** (step 2.2), the **character card**
      on the left screen edge (step 2.3) and the **dice footer** (step 2.6). Together they show the
      last result, HP and the turn with its effects at all times.
   2. **Header card**, one band, left to right: the brand (step 7); the turn (label and the turn
      number in `text-hero`); initiative with its reroll, Next turn and the ⋯ menu; the effects
      timeline (ADR 0017 step 6, beside the initiative); the **last result** docked at the right, as
      one short line under its label ("17 · Attack · hit", ADR 0027 step 4.5, amended 2026-10-17; was "Last result · T3 · 21:31": d20, verdict, damage). Hit points are not
      in the header.
   3. **Character card**, 280 px wide (design), the full height between header and footer, scrolling
      on its own: name and class · level; the abilities with THAC0; **hit points** (value / max, the
      newest change, the bar with the hatched dying zone, threshold colours and the last-damage
      ghost); **HP change on one line**: the amount, Damage (Phosphor `Drop`), Healing (Phosphor
      `Heart`), then the source under it; the charge cards (Stoneskin, Absorb); the five saving
      throws as click-to-roll rows.
      The card reads as three groups, Identity / hit points (bar, HP change, charge cards) / saving
      throws: 24 px between groups, 12 px inside one.
   4. **Rail**, 68 px (design), between the character card and the stage, on the ground (not a card):
      bare Phosphor icons with their caption under them, the active one on a tint pill
      (`bg-amb-900 text-tint`; *amended by [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) step 5.1, 2026-10-17:* the
      selected tone with a 3 px tint bar, the one selection mark). Activities: **Act**, **Spells** (lookup), **Experience**,
      **Inventory**. A click focuses that activity; the stage reshapes, the always-visible zones do
      not move. Each activity mounts on its first visit and stays mounted while hidden (ADR 0009
      step 2, kept). No settings item in the rail.
   5. **Stage per activity:**
      - **Act**: the "now" band over the journal. The now band is the act panel (tabs **with icons,
        no panel title**: Weapons `Sword`, Spells `Sparkle`, Items `Backpack`, Skills `ListChecks`)
        beside the **result panel** (the latest result or a selected journal entry, with its
        calculation and corrections), act : result = `minmax(28rem, 1.35fr) | minmax(24rem, 1fr)`
        (design). The band takes its content's height, at most 60 % of the stage (design); the act
        panel scrolls past that. Under it the split handle (step 5.4), then the journal (step 5),
        at least 14rem tall, with the rest of the height.
      - **Skills tab**: the proficiencies and the bard skills are two raised blocks (`bg-neu-900`,
        rounded), 24 px apart; a row's hover is the next tone step (`bg-neu-800`).
      - **Weapons tab**: the weapon in hand is one row: its name, its stats as one line of text
        (`Melee · 1d8 S/M · 1d12 L · +2 / +2 to hit / dmg · THAC0 11`, the size not in use dimmed)
        and the **Attack** button, the only control in the row (step 3.3). The other weapons are
        chips under it (one click switches, never attacks). The target bar stays above.
      - **Spells (lookup)**: A2's pane: filter and list on the left, `minmax(18rem, 22rem)` (design);
        the detail on the right: name and meta, the stats as tiles, the cast box (dice at the
        Bard's level, Add as effect), the description in one column capped at 75 characters (step 3.5),
        scrolling on its own (*amended 2026-10-19, user: one readable column; was two text columns*).
      - **Experience**: progress and the Add XP form side by side in one card (form 24rem, design),
        the ledger card below with the rest of the height.
      - **Inventory**: the inventory and weapons cards (not drawn in B; the area item lays them out
        under these steps).
   6. **Dice footer card**, full width, one line, in every activity: the **settings cog first** (it
      opens the settings popover of ADR 0009 step 6), then the dice, the formula, −1 / +1, Roll and
      the favourites strip (step 1.2).
   7. **Allocation** (design), from the outer padding, gutters, character card and rail:

      | Viewport  | Stage width | Below the header (character card, rail, stage height) | Act: act panel \| result |
      | --------- | ----------- | ----------------------------------------------------- | ------------------------ |
      | 1440x900  | 1044 px     | ~688 px                                               | ~592 \| ~440 px          |
      | 1707x1300 | 1311 px     | ~1088 px                                              | ~746 \| ~553 px          |

      Extra width goes to the stage (ADR 0017's principle). At 1440x900 the journal shows about 8
      rows with the Weapons tab unless it is dragged or collapsed; at 1707x1300 all 16 rows of the
      sample show.
   8. **Content target** (*rechecked, unchanged, by [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) step 6.6,
      2026-10-17*; replaces ADR 0017 step 7's per-column checks; checked in `layout.spec` with
      bounding boxes): at every layout viewport the header card at rest (3 effect rows) is at most
      120 px tall, the page does not scroll and `expectNoClippedText` passes in each activity; at
      1440x900 the character card shows everything from the name to the fifth saving throw without
      scrolling, and the Weapons tab's weapon-in-hand row is fully inside the act panel.
3. **Space and type** (amends ADR 0017).
   1. **Header budget** (amends ADR 0017 step 6): the header card is at most **120 px** tall at rest,
      its outline included (was 112 px for an unframed header); it holds the brand, turn, initiative
      and last result besides the timeline. The rest of step 6 (3 effect rows at rest, the expander,
      the 40 % cap when expanded) stands.
   2. **Padding** (amends ADR 0017 step 4): a card's padding is 14 px vertical, 16 px horizontal
      (`py-3.5 px-4`; was `p-3`; *amended by [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) step 6.5, 2026-10-17:* a
      card's header band replaces its top padding). The ground's padding and the gutter between cards are 12 px. Gaps
      inside a card stay 4 / 8 / 12 px, except the gutter between two text columns in one card (the
      two-column spell list, XP progress | form): 24 px (design). The lookup description is no longer
      one of them (step 3.5).
   3. **The one tall control** (amends ADR 0017 step 5): the Weapons tab's **Attack** button is
      **44 px** tall, primary, two lines ("Attack" / "needs 6 vs AC 5"), with the Phosphor `Sword`
      icon centred in its own 44 x 44 cell left of the words. It is the only control above 32 px;
      every other control keeps step 5's heights.
   4. **A sixth type role** (amends ADR 0017 step 2; design): `text-hero`, 32 / 36 px, semibold, for
      the number a whole zone is about: the turn number in the header, the damage (or roll) total in
      the result panel, the XP total. At most one per card; `text-headline` stays the panel number
      (HP in the character card). B draws the XP total at 40 px; it takes the one hero size, so there
      is no seventh.
   5. **A spell description is one readable column** (*added 2026-10-19, user: "some are rendered in
      two columns, some in one"; one column everywhere*). In the lookup and in the cast result's text
      alike (`SpellDescription`, no CSS columns), the measure is capped at 75 characters (`max-w-[75ch]`,
      left-aligned) at every layout viewport. Blocks (paragraph, list, table) are 16 px apart, clearly
      more than the lines of a paragraph (the one exception to the 4 / 8 / 12 px gaps of step 3.2); a
      list's items are 8 px apart. A table's cells are 12 x 8 px, its header a raised band (`bg-raised`)
      above the rows; a table too wide for the column scrolls inside its own box and never widens the
      page. Checked in `layout.spec` (one column, measure, rhythm, cell padding, no cut text, in
      English and French).
4. **Panel style: cards, no rules** (amends ADR 0009 step 5).
   1. Every zone (header, character, each stage panel, journal, dice footer) is a **card on the
      ground**: `bg-surface`, a 1 px outline (`border`), radius 8-10 px (design). Zones never touch:
      the 12 px gutter separates them, so **no rule or separator line** is drawn anywhere: not
      between zones, not between sections or header cells inside a card. Inside a card, grouping is
      by spacing, a section label or a raised tone step (`bg-neu-900`, ADR 0020 step 2). The result
      panel's outline is the accent (`border-amb-700`, design), to mark the latest result.
      *Amended by [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) steps 5.2 and 6 (2026-10-17):* "latest" is a 2 px tint
      outline; the focused card of the activity rises one tone with a 2 px outline; cards and
      groups get header bands (tones, not lines: "no rules" stands).
   2. **Section labels** are back: 11 px (`text-label`), **uppercase, letter-spaced (0.08em)**,
      `text-ink` (the full ink, one step above `text-ink-muted`, for contrast; design: "TARGET", "HIT POINTS", "SAVING THROWS · CLICK TO ROLL", "LEVEL 1",
      column headers). This replaces ADR 0009 step 5's "no tiny uppercase letter-spaced labels".
      Field labels next to a control ("Initiative", "Situational") stay sentence case.
      *Amended by [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) steps 5.6 and 6.2 (2026-10-17):* card titles (Journal,
      Items, Weapons, Effects) are section labels too, and labels sit in the card's header band.
   3. The rest of ADR 0009 step 5 stands: no gradients (a flat tint), shadows only on what floats,
      tabular figures, at most 3-4 sizes per card, Inter and Phosphor (the Cinzel wordmark excepted,
      ADR 0020).
5. **Journal.**
   1. **One line per entry**, columns **turn · value · icon · entry · time**: the turn label on the
      first entry of a turn ("TURN 3 · CURRENT", "OUT OF COMBAT"), the value right-aligned before
      the icon (red for damage taken, green for healing, as today), the entry text, the time last.
      Nothing is drawn under an entry; a long entry text wraps inside its row (step 1).
   2. **No Note**: the Note feature is removed from the journal (and its row action).
   3. Row actions show **on hover**, in the line: **Reroll** on an entry that rolled dice. Undo and
      Redo are in the journal's head (with the All / Rolls filter and the entry count). The current
      turn's group is tinted; the entry shown in the result panel is highlighted.
   4. **Split**: a handle (a grip, `cursor: row-resize`) between the now band and the journal is the
      only control: a **click** (pointer moved under 4 px) collapses or expands the journal, a
      **drag** resizes it (dragging a collapsed journal opens it at the dragged height). Keyboard:
      arrows resize, Enter / Space toggle. The separator states Collapsed / Expanded
      (`aria-valuetext`). There is no Collapse / Expand button. Collapsed, the journal is a
      one-line bar in the same column order (turn tag, value, icon, the last entry, time), with the
      count and Undo / Redo. The split's height and collapsed state are per-device
      display state, stored with the preferences of ADR 0009 step 6 (never sent to the API).
6. **Spell names.** A spell's name is shown in the UI language only, everywhere (lookup, spellbook,
   journal, result, timeline, dice favourites), through the language map of
   [ADR 0023](0023-localized-text.md) with its per-field English fallback. No second-language name
   beside it.
7. **Brand mark** (amends ADR 0020 step 5's drawing). The mark is **Facet · Bare**: a hexagon outline
   in `tint` (stroke 2 on a 32 viewBox, round joins) with the "!" in `highlight` centred on its axis
   (stem 3 wide, a dot under it), no front face. One drawing serves the header and the favicon:
   [`docs/design/ui-directions/logo/facet-bare.svg`](../design/ui-directions/logo/facet-bare.svg)
   (role variables with static Arcane fallbacks) becomes `BrandMark.tsx` and `web/public/favicon.svg`.
   The wordmark is unchanged (Cinzel, `text-key`, the "!" in `highlight`). In the header card, below
   1500 px wide the mark shows without the wordmark (design); the page title stays `Critical!`.

## Consequences

- ADR 0009 steps 1 (four columns) and 2 (rail switches the workspace column, Fil and Moi beside
  every workspace) are replaced by step 2 here: there is no Fil column (the result is the header's
  last-result line, plus the result panel in Act) and Moi becomes the character card. Step 2's
  "mounted on first visit, kept" stands. ADR 0009 step 3 (viewport range) stands; step 6 changes
  only where the cog lives (dice footer, not the rail).
- ADR 0017 steps 2, 4, 5, 6 and 7 are amended by step 3 and step 2.8; its fixed 16 px root, the
  control heights other than Attack, and the mapping table stand.
- ADR 0020 step 5: the d20-face drawing is replaced by Facet · Bare; the rest (name with "!", Cinzel
  wordmark, `highlight` on the "!") stands.
- The design pages are a drawing, not the rule. Where they differ from this ADR, the ADR wins: the
  weapon-in-hand row's faint gradient becomes a flat tint (step 4.3); the hairlines the pages still
  draw between journal turn groups and under the lookup filter are dropped (step 4.1: a turn group
  is marked by its turn label and tint); the pages' remaining stroke-sketch icons become Phosphor.
- The die silhouettes of round 2 were dropped by the user; die values stay plain.
- The app has about forty `truncate` / `line-clamp` uses today; each area item removes the ones in
  its region and adds its region to the `expectNoClippedText` calls. Until then the helper runs only
  on the regions already reworked.
- The journal's Note goes away from the UI. ADR 0010 step 1 still allows a note amend on the API;
  whether the API keeps it is outside this ADR (a layout decision does not change the log's rules).
- Not covered: mobile and narrow layouts (ADR 0009 step 3), the Inventory stage's layout beyond the
  card style, light mode of the drawing (the pages are dark Arcane; ADR 0020's contrast test still
  holds every role in both modes).
