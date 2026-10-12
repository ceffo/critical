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

## Phase 1 Scope (Combat Loop)

Only these features are in scope right now:

1. HP Ledger
2. Combat Roll Resolution (attack + crit + damage)
3. Active Buffs + Round Counter
4. Spell Slots

Do not implement Phase 2+ features (character stats display, saving throws, spell database lookup, skill checks, XP ledger, inventory) even if they seem related.
