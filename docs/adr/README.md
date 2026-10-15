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
