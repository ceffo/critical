# Psionicist design round (CRI-241)

**Status: round 1, awaiting the user's verdict.** Nothing here is approved yet; every layout is a proposal.

The user's words (2026-10-22): "The psionist ui design is quite dumb and frankly unusable. we'll have to redo it thoroughly."
Round 1 feedback (2026-10-24): psionics should "be like combat or spells, being able to control and see what triggers,
maintains effects and see consequences"; the interface stays divided between Act (gameplay) and browsing (research);
psionics go on Act; out-of-combat actions (mind link, rejuvenation) and an intuitive resting workflow matter; single and
multi-class sheets both.

- **Pages**: `web/design/psionics/` (the app's stylesheet, theme code and components; data in `data.ts`: Joran Veil, thief 7 /
  psionicist 8, Wisdom 17). `?screen=<id>&lang=fr|en&theme=<id>&light=1`.
- **Shots**: `just psionics-shots` writes `shots/<screen>--<fr|en>-<WxH>.png` at 1920x1080 and 2560x1440 (plus `-full.png` when a
  card scrolls). It fails on cut text or a page scroll; `--self-test`, `--fr`, `--only=a,b` as the equipment round's.

| Screen | What it shows |
|---|---|
| `act-powers` | Act > Pouvoirs: target bar, defence row, attacks with an inline form for the DM's rolls, powers, result card, "Entretenu" card |
| `act-attack` | After an attack: two contests, the third tangent makes contact, the second check is not rolled |
| `act-defend` | "Attaque subie": the DM's mode and rolls, the contest against the raised defence, keep a roll |
| `act-contact` | The Contact power's inline form (distance, life order, resists, cost by level) |
| `act-single` | A single-class psionicist, before combat |
| `rest`, `rest-blocked` | The Repos popover of the character card, free and blocked by a maintained power |
| `lookup-powers` | The browsing side: Sorts > Pouvoirs, catalogue and known powers (picks) |
