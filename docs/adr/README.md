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
| 0006 | [Saving throws](0006-saving-throws.md) | accepted, steps 2, 4 and 5 amended and step 8 added 2026-10-16 (the adjustment moves the target) |
| 0007 | [HP ledger](0007-hp-ledger.md) | accepted, steps 2, 3, 8 and 9 amended by 0018, step 2 by 0019 |
| 0008 | [Spell slots and long rest](0008-spell-slots-and-long-rest.md) | accepted |
| 0009 | [Dashboard layout and theme system](0009-dashboard-layout-and-theme-system.md) | accepted, steps 3 and 5 amended by 0017, step 4 replaced and step 5 amended by 0020, steps 1 and 2 replaced and steps 5 and 6 amended by 0022 |
| 0010 | [Action stack and undo](0010-action-stack-and-undo.md) | accepted, steps 1 and 2 and Consequences amended by 0018, Consequences by 0019 |
| 0011 | [HP below zero](0011-hp-below-zero.md) | accepted |
| 0012 | [Attack against an unknown AC](0012-attack-against-unknown-ac.md) | accepted |
| 0013 | [Initiative](0013-initiative.md) | accepted, steps 2 and 4 amended by 0018 |
| 0014 | [Skill checks](0014-skill-checks.md) | accepted |
| 0015 | [Experience](0015-experience.md) | accepted, step 4 amended by 0018 |
| 0016 | [Inventory and editors](0016-inventory.md) | accepted; steps 9 to 16 (an item's own powers) added 2026-10-16 |
| 0017 | [Type scale and density](0017-type-scale-and-density.md) | accepted, steps 2, 4, 5, 6 and 7 amended by 0022 |
| 0018 | [Every mutation is a transaction; undo pops the last one](0018-transactions-and-undo.md) | accepted, step 6 and Consequences amended by 0019 |
| 0019 | [Redo replays the newest popped transaction](0019-redo.md) | accepted |
| 0020 | [Visual identity: Critical! palettes, roles and brand mark](0020-visual-identity.md) | accepted, step 5 amended by 0022 |
| 0021 | [Reference data: the spell catalogue artifact, storage and deployment](0021-spell-catalogue.md) | accepted, per-language fields amended by 0023 |
| 0022 | [The screen around the focused activity: cards, no cut text and the Facet mark](0022-activity-focus-layout.md) | accepted |
| 0023 | [Localized text: one row per language, no per-language field](0023-localized-text.md) | accepted |
| 0024 | [Deployment on fly.io: environments, release flow, image and data](0024-deployment.md) | proposed |
