# Architecture and rules decisions (ADRs)

House rules and design decisions live here as `docs/adr/NNNN-<slug>.md` (four digits, next free
number). Game rules and design live **only** here; `docs/backend.md` is infrastructure and
architecture. This index is the entry point for rules.

Each ADR has:

- **Status**: proposed | accepted | superseded by NNNN.
- **Context**: why the decision was needed.
- **Decision**: the rule as **numbered steps**. Each step is testable on its own: code and tests
  cite it as `ADR NNNN step N`.
- **Consequences**: what follows, including edge cases and what is deliberately not covered.

Changing a rule means a new or amended ADR in the same PR as the code. Never change the code alone.

## Index

| # | Title | Status |
|---|---|---|
| 0001 | [Critical hits](0001-critical-hits.md) | accepted |
| 0002 | [Attack resolution](0002-attack-resolution.md) | accepted, step 6 and Consequences amended by 0016 |
| 0003 | [Damage](0003-damage.md) | accepted, steps 4, 5 and 11 amended by 0016 |
| 0004 | [Buffs](0004-buffs.md) | accepted, steps 10 and 11 and Consequences amended and step 18 added by 0016; step 19 (add as effect) added |
| 0005 | [Rounds and time](0005-rounds-and-time.md) | accepted |
| 0006 | [Saving throws](0006-saving-throws.md) | accepted, steps 2, 4 and 5 amended and step 8 added 2026-10-16 (the adjustment moves the target), step 6 amended by 0028 |
| 0007 | [HP ledger](0007-hp-ledger.md) | accepted, steps 2, 3, 8 and 9 amended by 0018, step 2 by 0019 |
| 0008 | [Spell slots and long rest](0008-spell-slots-and-long-rest.md) | accepted |
| 0009 | [Dashboard layout and theme system](0009-dashboard-layout-and-theme-system.md) | accepted, steps 3 and 5 amended by 0017, step 4 replaced and step 5 amended by 0020, steps 1 and 2 replaced and steps 5 and 6 amended by 0022; steps 4 and 6 amended by 0026 (four themes, a dark and a light theme per device) |
| 0010 | [Action stack and undo](0010-action-stack-and-undo.md) | accepted, steps 1 and 2 and Consequences amended by 0018, Consequences by 0019, step 6 amended by 0028 |
| 0011 | [HP below zero](0011-hp-below-zero.md) | accepted |
| 0012 | [Attack against an unknown AC](0012-attack-against-unknown-ac.md) | accepted |
| 0013 | [Initiative](0013-initiative.md) | accepted, steps 2 and 4 amended by 0018 |
| 0014 | [Skill checks](0014-skill-checks.md) | accepted; step 5 no longer offered by the Skills tab (API unchanged), by 0027 (proposed) |
| 0015 | [Experience](0015-experience.md) | accepted, step 4 amended by 0018 |
| 0016 | [Inventory and editors](0016-inventory.md) | accepted; steps 9 to 16 (an item's own powers) added 2026-10-16 |
| 0017 | [Type scale and density](0017-type-scale-and-density.md) | accepted, steps 2, 4, 5, 6 and 7 amended by 0022; step 6 rechecked (unchanged) by 0026; step 6 amended by 0027 (5 turn columns, effect buttons on hover) |
| 0018 | [Every mutation is a transaction; undo pops the last one](0018-transactions-and-undo.md) | accepted, step 6 and Consequences amended by 0019 |
| 0019 | [Redo replays the newest popped transaction](0019-redo.md) | accepted |
| 0020 | [Visual identity: Critical! palettes, roles and brand mark](0020-visual-identity.md) | accepted, step 5 amended by 0022; steps 1 to 6 amended by 0026 (contrast ladder, non-text contrast, per-theme semantic hues, Cinzel labels in Parchment) |
| 0021 | [Reference data: the spell catalogue artifact, storage and deployment](0021-spell-catalogue.md) | accepted, per-language fields amended by 0023; step 9 (rule tables, format 3) accepted 2026-10-16; step 6 (engine) amended by 0025 |
| 0022 | [The screen around the focused activity: cards, no cut text and the Facet mark](0022-activity-focus-layout.md) | accepted; steps 2.4, 3.2, 4.1 and 4.2 amended and step 2.8 rechecked by 0026 (selection mark, "latest" outline, card titles as labels, raised stage and bands); steps 2.2, 2.3, 2.5, 2.6, 2.7, 3.4, 5.1, 5.3 and 7 and 4.2's example amended by 0027 (header, character card width, now band fits its tab, previous weapon + drop-down, result card, compact journal, Skills, lookup, Inventory, XP, dice footer) |
| 0023 | [Localized text: one row per language, no per-language field](0023-localized-text.md) | accepted; steps 9 to 12 (item and weapon names) added 2026-10-16 |
| 0024 | [Deployment on fly.io: environments, release flow, image and data](0024-deployment.md) | proposed (step 8 and open defaults); step 5 accepted and steps 2, 4.2, 5, 6.1, 7 and 9 amended by 0025, 2026-10-16; production starts from the seeds (amendment 2026-10-16); step 9.2 answered: the restore point is a copy database, production has no public-IP bypass (amendment 2026-10-16); staging versions from the conventional commits, tagged by CI, steps 1.4, 3, 4 and 10.2 (amendment 2026-10-17, accepted, user OK 2026-10-17); promotion on demand from main, newest staging version or the one asked, admins only, steps 4 and 10.2 (amendment 2026-10-17, accepted, user OK 2026-10-17); `just major` for a manual major, step 3 (amendment 2026-10-17, accepted, user OK 2026-10-17); one tag ruleset, `release tags`, step 10.2 (amendment 2026-10-17) |
| 0025 | [PostgreSQL storage and tests without a database](0025-postgresql-storage.md) | accepted; step 9 (SQLite copy tool) retired 2026-10-16 |
| 0026 | [Colour and depth: the contrast ladder, the raised stage and four themes](0026-contrast-ladder-raised-stage-four-themes.md) | accepted (user OK 2026-10-17, proposals as written); step 5.9's initiative size amended by 0027 |
| 0027 | [Space follows frequency of use: French as the layout reference, one-word buttons and the per-screen layout pass](0027-space-follows-frequency-of-use.md) | accepted (user OK 2026-10-17, proposals as written) |
| 0028 | [Ability checks](0028-ability-checks.md) | accepted (user OK 2026-10-17) |
