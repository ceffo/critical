# Critical — Frontend Context (React Agents)

## Stack

- **Framework**: React 18 + Vite + TypeScript (strict mode)
- **Styling**: Tailwind CSS v3
- **Components**: shadcn/ui (components live in `web/src/components/ui/`)
- **i18n**: i18next + react-i18next (see "Internationalisation")
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

## Internationalisation (en / fr)

Every user-visible string goes through a key; never write a literal in JSX (`web/src/i18n/catalogues.test.ts`
fails on JSX text and on `placeholder`/`title`/`aria-label`/`alt` literals in `App.tsx` and `components/`
outside `ui/`).

- Catalogues: `web/src/i18n/locales/<lng>/<namespace>.json`, languages `en` (fallback) and `fr`.
- Namespaces: `common` (all UI strings, default namespace) and `spells` (spell names only).
- `common` keys: `area.thing`, camelCase leaves, grouped by panel/feature (`hp.applyDamage`,
  `buffs.none`, `combat.weapons.longsword`). Interpolation uses `{{name}}`: `t("spellSlots.level", { level })`.
  Add the key to **both** `en` and `fr`; a test requires identical key sets.
- `spells` keys: the spell's stable id (snake_case of the English name, e.g. `magic_missile`), value is the
  name. Render with `spellName(t, id, englishName)` from `@/i18n/spellName`; a missing `fr` key falls back
  to `en`, then to the supplied English name. `spellId(name)` derives the id until the spell database
  carries an explicit one. Spell descriptions stay English.
- Use `const { t } = useTranslation()` in components. Dynamic keys are fine, e.g. a template literal per tab id.
- Language toggle (`components/LanguageToggle.tsx`) switches live and persists to `localStorage`
  (`critical.language`); presentation only, never sent to the API.
- Not translated: server error messages and user-entered text (buff names, HP sources).
