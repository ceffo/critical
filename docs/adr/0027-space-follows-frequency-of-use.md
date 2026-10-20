# ADR 0027: Space follows frequency of use: French as the layout reference, one-word buttons and the per-screen layout pass

**Status**: accepted 2026-10-17 (user OK, proposals as written); written (critical-u9hr.1; the user OKs it on the PR). Amends
[ADR 0022](0022-activity-focus-layout.md) steps 2.2, 2.3, 2.5, 2.6, 2.7, 3.4, 4.2 (its example label),
5.1, 5.3 and 7; [ADR 0017](0017-type-scale-and-density.md) step 6 (the timeline's turn columns and
the effect rows' buttons); [ADR 0026](0026-contrast-ladder-raised-stage-four-themes.md) step 5.9 (the
initiative's size only); the UI side of [ADR 0014](0014-skill-checks.md) step 5 (the Skills tab no
longer offers the pre-roll adjustment; the API stays). Builds on ADR 0026 steps 5 and 6 (selection
mark, "latest" outline, chips, verdict pill, bands) without changing them. The points marked
**proposed, user OK on the PR** are values the user did not speak to; each is listed in Consequences.

## Context

A walkthrough of every screen with the user, in French (2026-10-17), from the captures of
`web/ui-review/screens.spec.ts` "full set FR" and
[`docs/design/ui-review/shots/`](../design/ui-review/README.md). The user, after the colour round:
_"I am a bit disappointed. all that work only for theming. I was under the impression that the layout
equilibrium was also going to be addressed."_

The rule behind every change, in the user's words: _"balancing space allocated vs frequency of
usage"_, and _"this last comment is what bothers me the most with layouts overall"_. Of the Weapons
tab: _"the first section takes too much space relative to its use. the situational is rarely used so
it should be discoverable but not always in the face."_

Two house rules came with it:

- _"you should show the mockups in french since text takes more space than english and can affect
  layout choices"_.
- _"reduce text in buttons always, prefer just one action word"_ (of the preparation editor: _"here
  the button should only be 'Enregistrer'"_).

Per screen, the user's words (the work items of the layout pass carry the full triage):

- **Header**: _"round number takes too much horizontal space. i think label and number should be
  vertically stacked, to give a chance for the branding not to be collapsed to the logo only"_;
  _"initiative: this results from going to the next turn, would be logical to be below the next turn.
  the initiative throw result is more important than the turn number so it should pop up. the d10
  label is not useful, can be on hover"_. Accepted proposals: effect edit / remove on hover, about
  5 turn columns instead of 8, the last result bigger, on one line, closer to the turn block.
- **Character card** (accepted proposals, _"agree"_): wider on wide screens so French save names fit
  one line, a labelled HP change; _"when a stat gets changed by a spell, don't put the original
  underneath, just on a hover ... of course keep modified in a different colour like it is already"_.
- **Weapons tab**: _"at some point we had a regression. a previous design, which was better, only
  listed to switch to the previous weapon and then add a drop-down to select another one. we don't
  need to see all weapons."_ Accepted proposals: a one-line target bar, Size and Situational one
  click away and shown as a tag when set; the act row grows to the tab's content. Of the journal:
  _"could be a little bit compact vertically, but it's not that big of an issue since we can drag it
  and dock it."_
- **Result card** (_"yes, and …"_): the verdict as a badge by the result, the adjustment collapsed to
  one line, the attack as tiles; every check is adjusted there, after the roll.
- **Journal**: _"don't space out turn blocks in the journal. a bit more compact and maybe some
  alternate background to tell them apart"_. Accepted proposals: a narrow turn marker, cancelled rows
  muted, the time closer to the text. After the journal pass (2026-10-17, brand design round):
  _"in the journal, i want the timestamps to return to be right aligned. it's too cramped having
  them. also i don't get the grey background and grouping for the round beginning and initiative
  roll"_ (steps 9.4 and 9.6).
- **Spells tab**: _"cast button: should be a button with a drop down: clicking on the button is the
  more common operation: cast on self for a defensive spell, and the drop down gets the alternate
  target choice. no need for a global check box"_; _"long rest merits its button always visible. just
  make it shorter no parenthesis, add a moon crescent icon"_; _"we can resume all that info with dots
  and 3 color schemes. an empty dot for a spell slot not used. one color for available another one
  for used. this reacts of course to casting the rary mnemonic spell which grants more spells for the
  day"_; _"no spell details at all, just the name. spell is clickable to show a popup with spell
  details"_.
- **Spell preparation**: _"never from scratch"_; _"add a global filter per level (that the character
  has)"_; _"should be able to increase the count of a spell with plus or minus buttons, last minus
  removes it"_; _"use the same dots language for the count of spells prepared vs possible"_; _"can go
  over quota during setup but the register button gates that the selection is not over budget"_;
  _"we don't necessarily need a pop-up, it could be in the same available space"_.
- **Skills tab**: _"both right-most columns can be dropped, and let's have the adjustment value to
  the caract. pop up a bit if we put it in a pill"_; the DM's adjustment moves to the result card
  (_"yes"_); proficiencies and bard skills side by side (accepted).
- **Spell lookup**: _"filters, except the school, are important. they should stay on view"_; _"also
  the 0th spell level makes no sense"_.
- **Inventory and Experience** (_"agree with all"_) and **dice bar** (_"ok with that"_): the proposals
  of their items. The side menu (rail) stays as it is.

## Decision

1. **Space follows frequency of use.**
   1. Every control and every piece of information belongs to one of three tiers, by how often the
      player uses it at the table:
      - **every turn**: Attack and its "needs N vs AC" line, AC, Next turn, the initiative result,
        HP and the HP change, Cast, the last result, the journal's newest rows, the dice;
      - **every fight** (or a few times a session): switching weapon, a saving throw, a skill check,
        an effect's edit, the target's size, an adjustment after the roll, Long rest;
      - **rarely** (once a day or less): preparing spells, the lookup's school filter, situational
        modifiers, editors (weapons, items, crit table), settings, stat originals, a die's name.
   2. **Every-turn controls get the room**: visible in their zone without scrolling at 1440x900 in
      French, at their full size (the one tall control, Attack, of ADR 0022 step 3.3; the `lg`
      controls of ADR 0017 step 5 for Next turn and Cast).
   3. **Every-fight controls are visible but compact**: one line, a small control, never a block of
      their own.
   4. **Rare controls are one click away and discoverable**, _"not always in the face"_: behind a
      labelled toggle, a menu, a drop-down, hover and keyboard focus, or a tooltip. Never only on
      hover: what shows on hover also shows on keyboard focus, and an action that hides at rest has a
      visible way in (a toggle, a menu, or its row).
   5. **A rare control that is set is never active unseen**: a modifier, a filter or an option that
      differs from its default shows as a tag (the chip of ADR 0026 step 5.3) where it applies, on
      the line it changes, until it is reset.
   6. **The user can promote a control** above its tier; that is recorded where the screen is
      decided (Long rest stays visible, step 10.2).
   7. **How a change is judged**: a layout change names the tier of each control it moves or adds
      (in the PR). A finding is a control larger or more visible than its tier, an every-turn control
      that scrolls or hides at 1440x900 in French, an empty area beside a cramped one in the same
      zone, or a set rare control without its tag.
2. **French is the layout reference.**
   1. Mock-ups, design pages and the screenshots a layout decision is made on are shown **in French
      first** (English on request). French text is the longer one and decides the widths.
   2. A layout is checked in French first: the content targets of ADR 0022 step 2.8 and of the steps
      below hold in French at 1440x900 and 1707x1300, and the no-cut audit (ADR 0022 step 1.3) runs
      in **French and English** at every layout viewport.
   3. **Where `layout.spec` stands today** (checked 2026-10-17): its test "no cut text in French and
      with long inventory names, 1280 to 2560" audits the character card, the act stage (Weapons
      tab), the lookup (long spell, a spell with an effect) and the Inventory stage in French at the
      five layout viewports. The header, rail, journal, dice footer, Experience and the other act
      tabs (the "shell" and "act panel" tests) are audited in English only; the adjusted save card
      runs in both. Each screen item of this pass adds its region to the French audit, keeping the
      English one, so that at the end of the pass every region is audited in both languages.
   4. The layout screenshots kept for review (`layout.spec`'s 1440x900 and 1707x1300 shots) are taken
      in French for every region a screen item touches.
3. **One action word per button caption.**
   1. A button's caption is **one action word** in the UI language: "Attaquer", "Lancer",
      "Préparer", "Enregistrer", "Annuler", "Ajouter", "Appliquer". An icon may go with it, or
      replace it when the icon is unambiguous and the button has a tooltip.
   2. When no single word names the action, a short fixed term is allowed: "Tour suivant",
      "Repos long" (the user's own words). Never a sentence, a parenthesis, a count or an
      explanation in the caption.
   3. The detail goes in the **tooltip and the `aria-label`** ("Réinitialiser les emplacements de
      sorts", "Passer à la Dague"), and the information a button needs (Attack's "needs 6 vs AC 5",
      a switch button's weapon name) goes on its second line or as its object, not as more verbs.
   4. Main versus quiet: each zone has at most one main (filled) button, the every-turn action of
      that zone (Attaquer on Act's Weapons tab, Tour suivant in the header, Lancer for a spell);
      the others are quiet (the shared pieces of the colour epic). Add buttons are quiet.
4. **Header** (amends ADR 0022 steps 2.2, 3.4 and 7, ADR 0017 step 6, ADR 0026 step 5.9).
   1. **The turn is stacked**: its label ("TOUR") over its number. The number is `text-headline`
      (24 px), no longer `text-hero`.
   2. **The initiative sits under "Tour suivant"**, which is the header's main button. The
      initiative result is the header's **`text-hero`** number (the one per card of ADR 0022 step
      3.4), larger and stronger than the turn: plain ink, no chip, as ADR 0026 step 5.9 says, at
      the hero size instead of `text-key`. The die ("d10") is in its tooltip; the reroll stays
      reachable beside it (icon button with a tooltip).
   3. **Effects timeline: 5 turn columns** at every width (*proposed, user OK on the PR*; was 8, 12
      or 16 by width). The freed width goes to the effect-name column, so names do not wrap at
      1440x900 in French with the fixture's long names. A pill running past the window fades out
      (ADR 0022 step 1.2 stands). The **name column is as wide as the longest effect name, the
      hidden ones included** (user pick, brand round 2026-10-17), so it does not jump when "+N"
      expands the list; a name longer than the column's cap (`clamp(14rem, 22vw, 26rem)`, a
      player's very long custom name) wraps, never cut. The 5 turn columns share all the width
      the timeline gets (step 4.6).
      **Out of combat (round 0) the round-number row and the current-round highlight go** (*amended
      2026-10-17, user: "When we're out of combat, we should not see the rounds", bars, no
      numbers*); the row itself stays, empty, so the "Effets" label cell and the effect rows keep
      their place, and the bars, pills, charges and item powers draw as in combat.
   4. **Effect rows show their edit and remove buttons on hover and keyboard focus** only (step 1.4),
      not at rest. Add stays visible in the "Effets" label cell.
   5. **The last result is one line**, larger than today (its value in `text-key`, the words in
      `text-body`), on one line in French at 1440x900 and 1707x1300. It sits under the brand in the
      header's right column (step 4.6; was "follows the timeline directly, the spare width to its
      right", changed 2026-10-17 by the brand round's pick).
      **Shortened** (*amended 2026-10-17, user: "the text in the last result ... should be way
      shorter than that. We shouldn't have most of the details of the result", "restricted to like
      ... the sixth of the screen", picked "value + what + verdict", "some coloring hints for the
      success vs failure. no caps ... prefer writing failed in red rather than FAILED"*): one short
      line, "value · what · verdict", built in the frontend from the entry's payload (the d20 or
      total, a word or name, a verdict for a roll that has one), in a column of at most 1/6 of the
      viewport that never wraps: a longer line ends with an ellipsis, the full journal wording and
      its round and time are the tooltip (ADR 0022 step 1.2 exception). The verdict is coloured
      (success green, failure red, crit highlight), sentence case, never upper case. Every log
      kind has a short wording (`lib/lastResultShort.ts`, a unit test per kind); the journal's
      wording is unchanged. The width freed goes to the timeline.
   6. **The brand shows in full** (mark and wordmark) at every layout width (replaces step 7's
      1500 px breakpoint). **Its place is design E** of the brand round
      ([README](../design/brand-placement/README.md), user pick 2026-10-17): the brand (mark +
      wordmark) at the top right, the last result under it, **6 px** apart, both right-aligned in
      one column docked at the header's right edge; that column's right edge lines up with the
      result card below (the header and the now band share their right edge). The **timeline
      takes all the width** between the initiative and that column, its 5 turn columns sharing
      it; effect names are never cut. The header opens on the turn.
   7. The header stays at most **120 px** at rest (ADR 0022 step 3.1, ADR 0026 step 6.6).

   *Checked by* `layout.spec`: the "shell" test (the header budget and the no-cut audit in English,
   the wordmark at every width) and the French no-cut test (`expectHeaderLayout` at every layout
   viewport: the turn stacked, the initiative under "Tour suivant" and larger than the turn, 5
   columns, the brand over the last result right-aligned 6 px apart and docked at the right edge,
   at most 120 px; at 1440x900 and 1707x1300 every effect name and the last result on one line;
   an effect's actions hidden at rest, shown on hover and on keyboard focus).
5. **Character card** (amends ADR 0022 steps 2.3, 2.7 and 4.2's example).
   1. **Width grows with the viewport**: `clamp(280px, 21vw, 384px)` (*proposed, user OK on the
      PR*): about 302 px at 1440, 358 px at 1707, 384 px from 1920 (was 280 px everywhere). The
      rule behind the value: at 1440x900 and 1707x1300, in French, every save name
      ("Bâton/Sceptre/Baguette", "Pétrification/Métamorphose") with its target and the saves heading
      are on one line; the implementer may raise the middle term until they are, and the test decides.
   2. **The saves heading is "JETS DE SAUVEGARDE" / "SAVING THROWS"**; "click to roll" is the rows'
      tooltip (replaces step 4.2's example "SAVING THROWS · CLICK TO ROLL").
   3. **HP change** (every turn): a **labelled** amount field ("Montant"), then Dégâts / Soins on the
      same line; the source under it, smaller (`text-label`), its placeholder never cut.
   4. **A stat changed by an effect shows only the modified value**, in its colour as today; the
      original is in a tooltip on hover and keyboard focus, not under the value and not on click (a
      click on a stat is kept for the ability check, its own ADR).
   5. ADR 0022 step 2.8 holds in French: at 1440x900 the card shows the name to the fifth save
      without scrolling.
   6. **Allocation** (amends step 2.7's table): the stage loses the card's extra width, about 1022 px
      at 1440x900 and 1233 px at 1707x1300; the act : result split of step 2.5 is unchanged.

   *Checked by* `layout.spec`: the "no cut text in French" test with a one-line check on each save
   row and the heading at 1440x900 and 1707x1300, and the content target of step 2.8.
6. **Act stage: the now band fits its tab** (replaces ADR 0022 step 2.5's 60 % cap).
   1. Until the player drags the split, the now band (act panel | result panel) is **as tall as the
      active tab's content**, and the journal takes the rest of the stage, at least 14rem (step 2.5).
      Changing tab resizes the band. When the tab's content is taller than the stage minus the
      journal's minimum and the handle, the act panel scrolls inside.
      *Implementation note (2026-10-17):* a result taller than the tab (an attack against an unknown
      AC with the GM's Hit / Miss) also sets the band's height, so that every-turn control does not
      hide under a scroll (step 1.7); the band is the taller of the two, with no cap.
   2. **Weapons does not scroll** at 1440x900 in French with the header at rest.
   3. The split handle is unchanged (ADR 0022 step 5.4): a drag sets the journal's height (and the
      band takes the rest), a click collapses it; both stay per-device preferences.

   *Checked by* `layout.spec`'s "act panel" test: at 1440x900 in French, the Weapons tab's
   `scrollHeight` equals its `clientHeight` and the journal is at least 14rem tall.
7. **Weapons tab** (amends ADR 0022 step 2.5 "Weapons tab").
   1. **Switching weapon**: one button to switch to the **previous weapon** (the one the server
      stores in the attack form), its caption the swap icon and the weapon's name, its tooltip
      "Passer à <arme>" (step 3.3); one click switches, never attacks. Beside it a **drop-down**
      listing every other weapon, names in full in the UI language. The chips of step 2.5 (one per
      other weapon) go. A bow's ammunition choice stays in the row.
   2. **Target bar on one line**: AC stepper and "Inconnue" (every turn); **Size and Situational
      behind a small toggle** (rare). *Amended (2026-10-17, user: "The selection for the size of the
      creature should not be hidden behind a menu. It should be on the same line. This is like the
      most important setting."):* the **target size is not behind the toggle**: the S/M · L chips sit
      on the target line beside the AC stepper, always visible, one click to change; the "large" tag
      goes (the chips show it). **Only the situational modifier** stays behind the toggle, and shows
      as a **tag** on the line when ≠ 0 (step 1.5). The line fits on one line from 1280 px wide
      (French). The choices still persist on the server across reload.
   3. The weapon in hand and its "needs N vs AC" line follow ADR 0026 step 5.9 and step 5.3's
      full-ink rule; Attack stays the one tall control (ADR 0022 step 3.3).

   *Checked by* the attack spec's switch flow (previous weapon, then one from the drop-down, never an
   attack) and `layout.spec`'s French no-cut audit of the act stage with the drop-down open.
8. **Result card** (amends ADR 0022 step 2.5's result panel; builds on ADR 0026 steps 5.2 and 5.5).
   1. **The verdict pill sits beside the result it judges** (the save's total, the check's roll, the
      attack's d20), not at the card's far edge.
   2. **"Ajuster après le jet" is one line**, closed by default and open when an adjustment is in
      effect; its "Appliquer" is a real button (disabled only while nothing changed).
   3. **Every check is adjusted here, after the roll**: saves (ADR 0006 steps 5-7), skill checks
      (ADR 0014 step 6), and later ability checks. The rules of those steps are unchanged.
   4. **The attack shows stat tiles** like the save (d20, target, margin); "Détail du calcul" and the
      AC correction stay reachable, folded.
   5. The card is filled by its content: no third of it left empty at 1440x900 or 1707x1300 for an
      attack or a save.

   *Checked by* the saves and skills specs' adjust flow (one happy path each) and `layout.spec`'s
   adjusted-card test in French and English.
9. **Journal** (amends ADR 0022 steps 5.1 and 5.3).
   1. **Turn blocks follow each other with no gap**; turns alternate between the card's tone and the
      sub-band tone (ADR 0026 step 6.3) to tell them apart. The current turn keeps one mark (the
      turn marker's tint), not a tinted group.
   2. **Compact rows: 22 px** for a one-line entry (`text-body`'s 18 px line and 2 px above and
      below), no gap between rows (*proposed, user OK on the PR*; about 26 px today). Still one line
      per entry (step 5.1); a long entry wraps inside its row.
   3. **A narrow turn marker** (at most 3rem, *proposed, user OK on the PR*): "T3" on a turn's first
      row, "Hors combat" folded to an icon with its label in the tooltip; the full "Tour 3 · en cours"
      in its tooltip and `aria-label`. It replaces the ~136 px turn column.
   4. **The time is right-aligned in its own last column** (*amended 2026-10-17, user's words in
      Context; was "the time sits right after the entry text"*): the entry text takes the row's
      width and wraps inside it, the time ends the row, so the times of all rows line up on the
      right. ADR 0022's "time as the last column" stands: nothing comes after it, so the row action
      (Reroll, step 5.3 of ADR 0022) sits before the time.
   5. Cancelled rows are muted with a muted tag (ADR 0026 step 5.8); the selected row is the
      selection mark (ADR 0026 step 5.1). The collapsed bar and the split handle are unchanged.
   6. **A turn's opening rows are the turn's own** (*2026-10-17, user's words in Context*): Next
      Round writes the advance ("Tour 2 → 3") and the new turn's initiative as one transaction (ADR
      0013, ADR 0018 step 1), which the journal framed like any multi-entry transaction (the raised
      tone and a tinted left edge, close to the selection mark), so the turn's start read as an
      unexplained grey group inside the turn block. Those rows are now plain rows of the turn they
      open, under its marker, on the block's tone (ADR 0026 step 6.3); each undone one carries its
      own Undone tag. Undo still pops them together. Other multi-entry transactions keep their frame.

   *Checked by* `layout.spec`'s journal checks: a row's height at most 24 px for a one-line entry,
   no vertical gap between two turn groups, every time ending at the same right edge (9.4), no frame
   around the turn's opening rows (9.6), and the no-cut audit in French and English; the opening
   transaction by `groupByRound.test.ts` (`opensTurn`).
10. **Spells tab** (new; the act panel's Sorts tab, no ADR 0022 step drew it).
    1. **"Lancer" is a split button**: the main part casts with the server's default target (today's
       call, no `onSelf`); the arrow opens the other target (`onSelf: false`, or what the server
       offers). The global "on an ally" checkbox goes.
       1. **The arrow is for a defensive spell only** (ruling from the user's words: _"cast on self
          for a defensive spell, and the drop down gets the alternate target choice"_): it shows
          only for a buff the cast can put on the Bard (ADR 0004 step 19's "adds a buff") whose
          target is "other", "self or other" or an area, and which is not hostile. A self-only
          spell, a damage or utility spell and a hostile buff (Blindness, Pain Touch, Slow, the
          Malisons) get a plain button. The cast endpoint still takes an explicit `onSelf` on any
          of them.
       2. **The menu holds one entry, the alternate** of the default: "Sur un allié" when the
          default is the Bard, "Sur moi" otherwise.
       3. **The default target**: always the Bard for a self-only or "self or other" spell; for an
          area, the Bard unless the buff is hostile; never for "other". **Hostile** means a
          modifier that hurts the bearer at the caster level, dice at their largest: to-hit,
          damage or save below 0, AC above 0, movement or attacks below 100 %. A penalty to
          attackers (`attackers_to_hit`) or a better initiative is not hostile.
       4. **The server decides it** and serves it on the spellbook entry as `cast_target`
          (`default` and `alternate`, `self` or `other`; absent: no choice); the browser only
          picks the menu's label.

       *Checked by* `TestEntry_TargetChoiceAt`, `TestEntry_DefaultOnSelf`, `TestEntry_Hostile`,
       `TestRepoSeeds_EveryBuffDefaultTargetAndChoice` (every buff of the seeds),
       `TestInfo_ServesTheTargetChoice`, `TestGetInfo_ServesTheCastTarget`, castsvc's
       `TestCast_PerSeedEntry` (Slow off the Bard by default and on him when asked, Stoneskin on an
       ally) and the cast spec (no arrow on Rary's, one entry on Strength).
    2. **"Repos long"** always visible (promoted, step 1.6), no parenthesis, a crescent-moon icon;
       **"Préparer"** beside it.
    3. **Slots as dots in three states** per level: empty (a slot not used, no spell prepared in it),
       available, used. They come from the server's slot data, extra slots (Rary's Mnemonic
       Enhancer) included, and update after a cast.
    4. **A row is the spell's name and its cast button**, nothing else; clicking the name opens a
       popup with the spell's details (the lookup's detail view), closable with Escape.
    5. Levels with no prepared spell are hidden.

    *Checked by* the cast spec (main cast, the other target from the arrow, the dots after a cast)
    and a unit test of the dot states from the server's slot data.
11. **Spell preparation in place** (new).
    1. "Préparer" turns the Sorts tab into the editor in the same space (no popup); the daily list is
       hidden while editing. "Annuler" and "Enregistrer" (step 3).
    2. It opens **pre-filled with today's preparation**.
    3. A **level filter** with only the character's spell levels filters the grimoire and the
       preparation; the grimoire is grouped by level (no "Niveau N" on each row).
    4. **− count +** per prepared spell; the last − removes it; adding from the grimoire adds one.
    5. **Per level, the dots of step 10.3** show prepared versus possible. Over budget is shown and
       allowed while editing; **"Enregistrer" is disabled while a level is over budget**. The budget
       is the server's slot count per level: the browser only compares two numbers the server gave
       it, and the server still refuses an over-budget save.
    6. A spell name opens the details popup (step 10.4).

    *Checked by* the spells spec's prepare flow (one happy path) and a unit test of the over-budget
    gate.
12. **Skills tab** (amends ADR 0022 step 2.5 "Skills tab"; the UI side of ADR 0014 step 5).
    1. **Columns: the name and the ability with its modifier as a pill** ("DEX 17 [−1]"). The
       "Seuil" and "Dernier" columns go (the target and verdict are on the result card).
    2. **No pre-roll adjustment in the tab**: the DM's adjustment is made on the result card after
       the roll (step 8.3, ADR 0014 step 6). ADR 0014 step 5's API (an optional adjustment with the
       roll) stays, unused by this tab.
    3. **Proficiencies and bard skills side by side** when the act panel holds two columns of at
       least 15rem, stacked otherwise (*proposed, user OK on the PR*); at 1440x900 in French both
       blocks are side by side. A thief's or a bard's block also writes the skill point budget
       (available with its formula, spent, left) and each skill's terms as text (user, 2026-10-19:
       transparency over a short tab), so that tab scrolls inside the act panel; the page does not.
    4. Rows look clickable (the shared row affordance).

    *Implementation note (2026-10-17):* the rows are the saves' clickable row (a raised stripe, hover
    and focus ring) and the section labels head each block; the column headers go with the columns
    they named. A proficiency without a modifier shows no pill ("CHA 16"), as the journal words it.
    A bard skill's row keeps its percentage (the skill's own value, not a target column). The two
    columns are a grid of `minmax(15rem, 1fr)` tracks: side by side at 1440x900 and up, stacked at
    1280x800, where the act panel scrolls inside (step 6.1) and the page does not.

    *Checked by* `layout.spec`'s French no-cut test (the Skills tab at every layout viewport; at
    1440x900 both blocks side by side inside the panel, the page not scrolling) and the skills spec's
    roll-then-adjust flow.
13. **Spell lookup** (amends ADR 0022 step 2.5 "Spells (lookup)").
    1. **Search, the class control and the level chips always in view**; the school behind a small
       "Filtres" button, shown as a tag when set (step 1.5).
    2. **Level chips are the levels present in the catalogue**, derived from the data, not
       hard-coded (no level 0 today).
    3. **The cast box shows only when there is something to roll**.
    4. Stat-tile labels fit one line in French at 1440x900; the selected row is the selection mark.

    *Checked by* the lookup spec and `layout.spec`'s French audit of the lookup; a unit test of the
    levels derived from the catalogue.
14. **Inventory and Experience** (amends ADR 0022 step 2.5 "Experience" and "Inventory").
    1. **Item rows**: the name, its dice right after it, the quantity stepper, then edit / delete on
       hover and focus. The meta line shows the behaviour in the UI language ("Stock", "Charges",
       "Quotidien"); the free-text type stays in the edit form.
    2. **Add buttons are quiet and one word** ("Ajouter").
    3. **Items and weapons side by side** when the stage is at least 1200 px wide (1707x1300 and up),
       stacked below that (*proposed, user OK on the PR*). At 1707x1300 both fit without scrolling;
       at every viewport the page never scrolls, the cards scroll inside.
    4. **Experience**: the ledger card is as tall as its rows, bounded by the available height (was
       the rest of the height); "Ajouter" is a real button, disabled-looking only when disabled.

    *Checked by* `layout.spec`'s inventory and XP checks (no page scroll; at 1707x1300 the two cards
    inside the stage; the ledger card's height following two rows) and the French audit.
15. **Dice footer** (amends ADR 0022 step 2.6).
    1. The chosen die's count sits **inside its die button**, readable.
    2. **"Lancer" is quiet**: on Act, "Attaquer" stays the one main button (step 3.4).
    3. "Aucun favori" is a small hint (`text-label`, muted); the controls use the shared chip and
       button pieces. The rest of step 2.6 (cog first, one line) stands.

    *Checked by* the dice spec and `layout.spec`'s no-cut audit of the footer in French.

## Consequences

- **Proposed points for the user's OK on the PR**: the timeline's 5 turn columns at every width
  (4.3); the last result packed against the timeline (4.5); the mark alone at 1280 (4.6); the
  character card's `clamp(280px, 21vw, 384px)` (5.1); the journal's 22 px rows (9.2), its 3rem turn
  marker (9.3); the Skills blocks' two-column threshold
  (12.3); items and weapons side by side from a 1200 px stage (14.3).
- **The initiative changes size, not style**: ADR 0026 step 5.9 called it "a plain `text-key`
  number"; it stays plain (ink, no chip) at `text-hero` (step 4.2), and the turn number drops to
  `text-headline`. The header still has one hero (ADR 0022 step 3.4).
- ADR 0022's tiers were implicit (the 44 px Attack, the always-visible zones); step 1 makes them the
  rule every later layout change is judged by, and the review checklist
  (`docs/agent-workflow.md`, "Review") carries steps 2 and 3.
- **The 60 % cap goes** (step 6): with a short tab the journal gets more room, with a tall one
  (Spells, Skills before step 12) less; the 14rem minimum and the drag stay the player's way out.
- The weapon chips of ADR 0022 step 2.5 were themselves the regression the user named; the previous
  weapon is already stored by the server, so step 7.1 needs no API change.
- Every check adjusted on the result card (step 8.3) leaves ADR 0014 step 5's pre-roll adjustment
  without a UI. It stays in the API (no rule changes here); dropping it from the API would be its own
  decision.
- The spell preparation gate (step 11.5) compares the server's prepared count and slot count per
  level; it computes no rule. The server's refusal stays the rule.
- `docs/frontend.md` describes today's screens; each screen item updates its part as it lands.
- Not covered: colours, bands and themes (ADR 0026); the rail (unchanged, user OK); mobile and
  narrow layouts (ADR 0009 step 3); the ability check itself (its own ADR); the crit-table and item
  editors' layouts.
