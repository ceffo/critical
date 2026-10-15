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
| 0002 | [Attack resolution](0002-attack-resolution.md) | accepted |
| 0003 | [Damage](0003-damage.md) | accepted |
| 0004 | [Buffs](0004-buffs.md) | accepted |
| 0005 | [Rounds and time](0005-rounds-and-time.md) | accepted |
| 0006 | [Saving throws](0006-saving-throws.md) | accepted |
| 0007 | [HP ledger](0007-hp-ledger.md) | accepted, steps 2, 3, 8 and 9 amended by 0018 |
| 0008 | [Spell slots and long rest](0008-spell-slots-and-long-rest.md) | accepted |
| 0009 | [Dashboard layout and theme system](0009-dashboard-layout-and-theme-system.md) | accepted, steps 3 and 5 amended by 0017 |
| 0010 | [Action stack and undo](0010-action-stack-and-undo.md) | accepted, steps 1 and 2 and Consequences amended by 0018 |
| 0011 | [HP below zero](0011-hp-below-zero.md) | accepted |
| 0012 | [Attack against an unknown AC](0012-attack-against-unknown-ac.md) | accepted |
| 0013 | [Initiative](0013-initiative.md) | accepted, steps 2 and 4 amended by 0018 |
| 0014 | [Skill checks](0014-skill-checks.md) | accepted |
| 0015 | [Experience](0015-experience.md) | accepted, step 4 amended by 0018 |
| 0016 | [Inventory and editors](0016-inventory.md) | accepted |
| 0017 | [Type scale and density](0017-type-scale-and-density.md) | accepted |
| 0018 | [Every mutation is a transaction; undo pops the last one](0018-transactions-and-undo.md) | accepted |
