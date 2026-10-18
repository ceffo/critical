# New-themes round: green, red and true amber (critical-4jaw, 2026-10-17)

**Approved 2026-10-17** by the user, "Approve all as shown" (names and ids proposed in the round,
accepted with it). Three themes on the ADR 0026 contrast ladder and raised stage, the role tables
in [`web/ui-review/candidate-themes.ts`](../../../web/ui-review/candidate-themes.ts) (the app's
`Theme` shape, ready to copy into `THEMES`).

| Theme (id / en / fr) | Modes | Primary | Moved semantic hue (ADR 0026 step 3.5) |
|---|---|---|---|
| `forest` / Forest / Forêt | dark, light | emerald 160 on forest surfaces | success to lime 128 |
| `ember` / Ember / Braise | dark | crimson-rose 355 on wine surfaces (tint L .725) | danger to vermilion 40 |
| `amber` / Amber / Ambre | dark | amber 70, amber ink, warm black | warning to orange 42, highlight lemon 105 |

Shots (real app, seeded table, French, Act · Weapons, 1440x900 and 1707x1300): [`shots/`](shots/).
Regenerate with `just ui-themes`: it paints the candidates over the app through `themeVars`, runs
the no-cut audit, then runs the app's `theme.test.ts` over them (only its three tests pinning
today's theme list fail, by design).
