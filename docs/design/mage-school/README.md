# Mage school design round: make the switch to a school of magic easy to find (CRI-392)

**Round 1, not approved yet.** Every layout here is a proposal until the user approves it.

The problem (user, 2026-11-14): the switch of ADR 0050 step 15 works, but nobody finds it: the
character's name is plain text, its popover's button reads only "Changer", and nothing says
"école" before the class list opens. User's choice: "Make it easy to find, mockups first".

- **Pages**: `web/design/mage-school/` (React on the app's stylesheet, `Button`, Phosphor icons),
  `?screen=<id>&lang=fr|en` (`&dark=1` for dark Arcane). Data in `data.ts`: ADR 0050's specialist
  table and the two characters of the developer's screenshots (elf mage 6; elf fighter 6 / mage 1).
- **Shots**: `just mage-school-shots` writes `shots/<screen>--<fr|en>-<WxH>.png` at 1920x1080 and
  2560x1440 in light Arcane, and the gallery `shots/index.html`; fails on cut text or a page scroll.
- A dashed magenta outline marks what the round adds or changes; it is not part of the design.

| Screen | Proposal |
|---|---|
| `a-tag`, `a-tag-dual`, `a-tag-after` | A: a "Choisir une école" tag on the class line while the mage has no school; gone after the switch, the class then reads "Transmutateur · Altération" |
| `b-class-link` | B: the class name is a link (dotted underline, caret, tooltip), like the level |
| `c-spells` | C: a "Magicien généraliste" band atop the spellbook with the button |
| `popover` | common: a caret on the name; "École" on the Classe row of the details instead of "Changer" |
| `picker` | common: the picker lists the schools as cards (opposed schools, ability minimum, race warning) |
