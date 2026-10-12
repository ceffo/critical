# Critical — Project Context (All Agents)

## What This Project Is

Critical is a personal AD&D 2.5 character sheet engine for a single Bard character. It automates mechanical computation (attack resolution, damage calculation, buff expiry, spell slot tracking) and presents a persistent dashboard. The player sends intent; the app returns fully-explained results.

**This is a personal tool, not a product.** No authentication, no multi-tenancy, no SaaS concerns.

## Monorepo Layout

```
critical/
  api/              ← Go backend (ALL game logic lives here)
  web/              ← React frontend (presentation only)
  seeds/            ← JSON seed data (spells.json, spell_effects.json, character.json, crit_table.json, weapons.json)
  docs/             ← Agent context files (this folder)
  docker-compose.yml
  nginx.conf
```

## Core Architectural Rule

**Zero game logic in the React frontend.** The frontend sends intent to the API and renders structured responses. All dice rolling, rule resolution, formula calculation, and state mutation happens exclusively in Go. If you find yourself writing a calculation in TypeScript, stop — move it to Go.

## REST API Conventions

- Base path: `/api/`
- JSON request/response bodies
- Errors: `{"error": "message"}` with appropriate HTTP status code
- All roll results return a complete breakdown struct (never just hit/miss or a bare number)
- Endpoints follow resource/action naming: `POST /api/hp/event`, `POST /api/roll/attack`, `POST /api/round/advance`

## Git & Commit Rules

- Commit after each meaningful unit of work (passing tests, working endpoint, etc.)
- Commit message format: `<type>: <short description>` (e.g. `feat: HP ledger append endpoint`)
- Always run `br sync --flush-only` before committing to export bead state
- Never commit broken tests or failing builds

## Key Domain Vocabulary

- **Bard**: the single character this app manages in Phase 1
- **HP Ledger**: append-only event log of all damage/healing. Current HP = SUM(delta).
- **Buff**: an active effect with either a charge count (decremented on trigger) or a duration in rounds (decremented on Next Round)
- **Round Counter**: manually advanced by the player; one tap = Next Round = decrement all duration buffs
- **THAC0**: "To Hit Armor Class 0" — lower is better. Attack hits if `roll + modifiers ≥ THAC0 - target_AC`
- **Spell Slot**: a prepared spell instance consumable once per day; reset on long rest
- **Vertical Slice**: one bead covers schema + Go engine + REST endpoint + React panel for one feature

## Phase 1 Scope (MVP for the 2026-10-17 game)

The goal is a tool the player can use at the table on Saturday 2026-10-17, run locally on a laptop
(`just dev`, desktop layout, no hosting). Hard cut at Saturday. The epic is `critical-qki`; items
labelled `mvp-saturday` are the must-haves.

**In for Saturday:**

1. Real character data: the campaign Google Sheet is the source of truth (level 15 Bard, HP 85,
   THAC0 13, STR 12), plus the PHB Strength / Dexterity / Bard spell tables as seed data
2. HP Ledger and Combat Roll Resolution (attack + crit + damage) with real weapons, ammo by arrow
   type, and the full AD&D 2.5 modifiers (STR, DEX, buffs); no special natural 1 / natural 20
3. Active Buffs + Round Counter, including charge buffs and effective STR (Power Gem effects apply
   only at effective STR 18, usually via Strength spells cast by others)
4. Spells with effects (Wizard-list effects extracted into a seed) and spells received from the party
5. Daily spell preparation from the spellbook within the PHB Bard limits
6. One append-only action log (also the combat log) with before/after snapshots; any action can be
   amended or voided, audited
7. Weapons and items in DB tables seeded from JSON; charged items and consumables
8. i18n: en/fr string keys for the UI and spell names (spell descriptions stay English)
9. A round-centric dashboard; Playwright e2e harness behind `just e2e`

**After Saturday (not to be built now):** the real crit multiplier table (house crit stays x2 as a
placeholder), editing weapons/items in the app, Gaia fire/lightning protections, Power Gem weapon
forms, hosting/deployment (`critical-8g1`), other devices/layouts, multiple characters, XP ledger,
skill checks.

Do not implement anything from the "after" list even if it seems related.
