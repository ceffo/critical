# Equipment design round: possessions, worn and wielded, the AC, pick list, review list (CRI-32)

**Round 1, 2026-10-18: proposals, awaiting the user's verdict.**

The screens of the equipment epic, for the UI items that build them (CRI-34 to CRI-41). The rules
behind every screen are [ADR 0033](../../adr/0033-equipment.md); this folder decides only where
things sit, under [ADR 0022](../../adr/0022-activity-focus-layout.md),
[ADR 0026](../../adr/0026-contrast-ladder-raised-stage-four-themes.md) and
[ADR 0027](../../adr/0027-space-follows-frequency-of-use.md).

- **Pages**: `web/design/equipment/` (React on the app's own stylesheet, theme code, `Button` and
  Phosphor icons; data in `data.ts`: Kelsen, a level 15 bard, after the conversion of ADR 0033 step
  26; PHB values). `?screen=<id>&lang=fr|en&theme=<id>&light=1`.
- **Shots**: `just equipment-shots` serves them on the app's Vite and writes
  `shots/<screen>--<fr|en>-<WxH>.png` at 1280x800 and 2560x1440, plus `-full.png` when a card
  scrolls at that size. It fails on cut text or a page scroll; `--self-test` checks the audit,
  `--fr` shoots French only, `--only=a,b` some screens.

## Screens

| Screen | What it shows |
|---|---|
| `inventory` | Inventaire: one possessions card, "Sur vous" (worn and ready) and "Dans le sac" (carried), category chips, Ajouter; the AC card on the right; the review band. |
| `review` | The review list opened above the possessions. |
| `add-catalogue` | Ajouter > Catalogue: category, search, the PHB rows with class warnings, "Votre exemplaire" (name, magic), "Porter tout de suite". |
| `add-homebrew` | Ajouter > Saisir: a typed magic item. |
| `edit-armour` | The editor of an armour: base AC, skills column, armour type, "+N". |
| `edit-magic` | The editor of a ring: powers and the templates menu. |
| `warnings` | Class warnings (shield on a bard, two-handed with a shield) on the rows, the AC card and the character card. |
| `ac-popover` | The AC on the character card and its breakdown popover. |
| `ac-override` | The AC overridden (Shield spell), the derived AC beside it. |
| `act-weapons` | Act > Armes: the target's armour on the target bar, its modifier as a tag. |
| `act-weapons-menu` | The weapon drop-down: ready weapons, then the pack's. |
| `act-target-armour` | The target armour drop-down open. |
| `initiative` | The declared action by the initiative, its menu and breakdown. |
| `skills` | Act > Compétences: the skill armour read from the worn armour. |
