# Critical — Project Context (All Agents)

## What This Project Is

Critical is a personal AD&D 2.5 character sheet engine for a single Bard character. It automates mechanical computation (attack resolution, damage calculation, buff expiry, spell slot tracking) and presents a persistent dashboard. The player sends intent; the app returns fully-explained results.

**This is a personal tool, not a product.** No authentication, no multi-tenancy, no SaaS concerns.

## Monorepo Layout

```
critical/
  api/              ← Go backend (ALL game logic lives here)
  web/              ← React frontend (presentation only)
  seeds/            ← JSON seed data (spell_effects.json, spellbook.json, character.json, weapons.json, skills.json); spells come from the catalogue artifact (ADR 0021, `just catalogue`)
  docs/             ← Agent context files (this folder)
  docker-compose.yml
  nginx.conf
```

## Core Architectural Rule

**Zero game logic in the React frontend.** The frontend sends intent to the API and renders structured responses. All dice rolling, rule resolution, formula calculation, and state mutation happens exclusively in Go. If you find yourself writing a calculation in TypeScript, stop — move it to Go.

## REST API Conventions

- Base path: `/api/`
- JSON request/response bodies
- Errors: `{"error": "message", "code": "stable_code"}` (plus `"field"` for a field error) with appropriate HTTP status code. The code comes from the domain error through the one mapping table in `internal/render`; the web words known codes in en + fr (`apiError.*` i18n keys) and shows `error` as the fallback.
- All roll results return a complete breakdown struct (never just hit/miss or a bare number)
- Endpoints follow resource/action naming: `POST /api/hp/event`, `POST /api/roll/attack`, `POST /api/round/advance`

## Git & Commit Rules

- Commit after each meaningful unit of work (passing tests, working endpoint, etc.)
- Commit message format: `<type>: <short description>` (e.g. `feat: HP ledger append endpoint`)
- Always run `br sync --flush-only` before committing to export bead state
- Never commit broken tests or failing builds

## Key Domain Vocabulary

- **Bard**: the single character this app manages in Phase 1
- **HP Ledger**: the append-only action log of damage and healing; current HP is derived from it ([ADR 0007](adr/0007-hp-ledger.md))
- **Buff**: an active effect with a charge count, a duration in rounds, both or neither; every buff change is in the action log ([ADR 0004](adr/0004-buffs.md))
- **Round Counter**: manually advanced by the player; Next Round, End combat and passing time are in [ADR 0005](adr/0005-rounds-and-time.md)
- **THAC0**: "To Hit Armor Class 0"; lower is better. The attack math is in [ADR 0002](adr/0002-attack-resolution.md)
- **Spell Slot**: a prepared spell instance, cast once per day; reset on long rest ([ADR 0008](adr/0008-spell-slots-and-long-rest.md))
- **Vertical Slice**: one bead covers schema + Go engine + REST endpoint + React panel for one feature

## Phase 1 Scope (MVP for the 2026-10-17 game)

The goal is a tool the player can use at the table on Saturday 2026-10-17, run locally on a laptop
(`just dev`, desktop layout, no hosting). Hard cut at Saturday. The epics are `critical-qki` and the
dashboard v3 redesign `critical-kiv`; items labelled `mvp-saturday` are the must-haves.

**In for Saturday:**

1. Real character data: the campaign Google Sheet is the source of truth (level 15 Bard, HP 85,
   THAC0 13, STR 12), plus the PHB Strength / Dexterity / Bard spell tables as seed data
2. HP Ledger and Combat Roll Resolution (attack + crit + damage) with real weapons, ammo by arrow
   type, and the full AD&D 2.5 modifiers (STR, DEX, buffs); no special natural 1 / natural 20
3. Active Buffs + Round Counter, including charge buffs and effective STR (Strength bonuses stack up
   to 18, Improved Strength replaces them up to 23: "all str spells stack up to 18. unless it's
   improved strength that can go to 23"; Power Gem effects apply at effective STR 18 or more,
   usually via Strength spells cast by others; user: "not exactly 18 >= 18")
4. Spells with effects (Wizard-list effects extracted into a seed) and spells received from the party
5. Daily spell preparation from the spellbook within the PHB Bard limits
6. One append-only action log (also the combat log) with before/after snapshots, used as a stack.
   User, 2026-10-14: "no amending values. just the note.. can only pop elements", for every entry
   kind: an amend only sets the entry's note, and only the newest non-voided entry can be voided
   (popped, restoring what it changed); both are audited corrections
7. Weapons and items in DB tables seeded from JSON; charged items and consumables
8. i18n: en/fr string keys for the UI; spell and item texts are localized-text rows served as language maps (ADR 0023)
9. A round-centric dashboard; Playwright e2e harness behind `just e2e`
10. Dashboard v3 (ADR 0009): responsive desktop layout from 1280 to 2560 CSS px (rail, workspaces,
    Fil and Moi columns), Nocturne themes with six ambiances, light / dark / system, and the per-device
    settings popover
11. Skill checks: bard skills and non-weapon proficiencies from the campaign sheet, rolled on the
    server
12. XP ledger: total, progress to the next level and the ledger (no level-up)

**After Saturday (not to be built now):** Gaia fire/lightning protections, mobile and other device
layouts (below 1024 CSS px), level-up mechanics, multiple characters.

**Hosting and deployment** (`critical-8g1`) is no longer deferred (user, 2026-10-16): fly.io, staging
from main and production from version tags, designed in [ADR 0024](adr/0024-deployment.md).

Do not implement anything from the "after" list even if it seems related.
