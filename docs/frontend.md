# Critical — Frontend Context (React Agents)

## Stack

- **Framework**: React 18 + Vite + TypeScript (strict mode)
- **Styling**: Tailwind CSS v3
- **Components**: shadcn/ui (components live in `web/src/components/ui/`)
- **State**: Zustand (one store per domain: hp, combat, buffs, spells)
- **API client**: native `fetch` with typed wrappers in `web/src/api/`
- **No game logic**: if you are computing anything beyond display formatting, stop and move it to Go

## Project Structure (web/)

```
web/
  src/
    api/              ← typed fetch wrappers (hp.ts, combat.ts, buffs.ts, spells.ts)
    components/
      ui/             ← shadcn/ui primitives (Button, Card, Badge, etc.)
      panels/
        LeftPanel.tsx     ← HP bar + active buffs (always visible)
        CenterPanel.tsx   ← switchable work area (spells / combat / lookup)
        RightPanel.tsx    ← round counter + recent events
    stores/           ← Zustand stores (hp.ts, combat.ts, buffs.ts, spells.ts)
    types/            ← TypeScript types mirroring Go response structs
    App.tsx           ← root layout, three-panel grid
```

## Dashboard Layout

The app is a **persistent three-panel dashboard**. Nothing is hidden behind tabs.

```
┌─────────────┬──────────────────────┬─────────────┐
│  LEFT       │  CENTER              │  RIGHT      │
│  (fixed)    │  (switchable)        │  (fixed)    │
│             │                      │             │
│  HP bar     │  Active work area:   │  Round      │
│  Current/   │  - Spell slots       │  counter    │
│  Max        │  - Combat rolls      │             │
│             │  - Spell lookup      │  Recent     │
│  Active     │  - Character stats   │  events     │
│  buffs      │                      │  log        │
│  list       │                      │             │
└─────────────┴──────────────────────┴─────────────┘
```

- Left and Right panels are **always mounted** — never conditionally render them
- Center panel switches via a tab bar at the top of the center column
- The dashboard grid uses Tailwind: `grid grid-cols-[280px_1fr_240px] h-screen`

## API Consumption Pattern

All API calls live in `web/src/api/`. Never call `fetch` directly from a component.

```typescript
// web/src/api/hp.ts
export async function appendHpEvent(delta: number, source: string): Promise<HpEvent> {
  const res = await fetch('/api/hp/event', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ delta, source }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
```

## TypeScript Types

Mirror Go response structs exactly. Keep them in `web/src/types/`.

```typescript
// types/combat.ts
export interface RollResult {
  d20Roll: number;
  d100CritRoll: number;
  isCrit: boolean;
  critMultiplier: number;
  baseThac0: number;
  modifiers: Array<{ source: string; value: number }>;
  effectiveThac0: number;
  targetACNeeded: number;
  hit: boolean;
  hitMargin: number;
  baseDamage: number;
  spellsBuff: number;
  weaponBuff: number;
  multipliers: number;
  strengthBonus: number;
  totalDamage: number;
  damageBreakdown: string;
}
```

## Zustand Store Pattern

```typescript
// stores/hp.ts
import { create } from 'zustand';

interface HpStore {
  currentHp: number | null;
  maxHp: number | null;
  events: HpEvent[];
  fetchHp: () => Promise<void>;
  appendEvent: (delta: number, source: string) => Promise<void>;
}

export const useHpStore = create<HpStore>((set) => ({
  currentHp: null,
  maxHp: null,
  events: [],
  fetchHp: async () => { /* call api/hp.ts */ },
  appendEvent: async (delta, source) => { /* call api/hp.ts, then fetchHp */ },
}));
```

## shadcn/ui Usage

- Use `Card`, `CardContent`, `CardHeader` for panel sections
- Use `Badge` for buff tags and spell slot indicators
- Use `Button` with `variant="outline"` for secondary actions, default for primary
- Run `npx shadcn@latest add <component>` to install new components into `src/components/ui/`

## Display Conventions

- HP bar: colored progress bar (green > 50%, yellow 25-50%, red < 25%)
- Buff list: each buff shows name + remaining charges/rounds as a Badge
- Roll results: show the full breakdown in a readable format — never just the final number
- Expiry alerts: use a toast notification (shadcn Toast) when a buff expires
