# ADR 0017: Type scale and density

**Status**: accepted, 2026-10-15 (critical-kiv.21; amends [ADR 0009](0009-dashboard-layout-and-theme-system.md)
steps 3 and 5); steps 2, 4, 5, 6 and 7 amended by [ADR 0022](0022-activity-focus-layout.md)

## Context

User, 2026-10-15: _"so much space is wasted by humongous font sizes... what's the logic in there.
don't you have a charter about font size?"_ and _"I expect to see more text and less dots"_. ADR
0009 grew the root font size with the viewport (16 px at 1280, 20 px at 2560), so every rem-based
size, padding and control was 25% bigger on a large screen, and gave no type scale: panels used
anything from `text-xs` to `text-5xl` (turn number 48-60 px, HP 36-45 px, a header about 150 px
tall, 17% of a 900 px screen). The user's table screen is a 5120x1440 ultrawide with Critical on a
third of it, about 1707x1300 CSS px. Extra width must go to columns and content, not to bigger
letters.

## Decision

1. **Fixed root.** The root font size is `16px` at every viewport (`web/src/index.css`, `:root`).
   No `clamp()` or `vw` on it. `1rem` is always 16 px; column widths keep their own clamps (ADR 0009
   step 1).
2. **Type scale by role.** Five Tailwind v4 `@theme` tokens in `web/src/index.css`, each with its
   line-height (and weight where it has one). Text uses only these:

   | Role            | Token           | Size / line-height | Weight                          | Use it for                                                                                               |
   | --------------- | --------------- | ------------------ | ------------------------------- | -------------------------------------------------------------------------------------------------------- |
   | Label, caption  | `text-label`    | 11 / 16 px         | regular (`font-medium` allowed) | field labels, stat names (STR, THAC0), column headers, hints, units, timestamps, badges                  |
   | Body            | `text-body`     | 13 / 18 px         | regular (`font-medium` allowed) | running text, values in lists, buttons, inputs, selects, list rows, menu items                           |
   | Panel title     | `text-title`    | 14 / 20 px         | semibold (set by the token)     | the heading of a panel, card or popover; the character's name                                            |
   | Key value       | `text-key`      | 16 / 20 px         | semibold (set by the token)     | numbers the player reads at a glance: ability scores, THAC0, damage dice, a d20, a weapon's name in hand |
   | Headline number | `text-headline` | 24 / 28 px         | semibold (set by the token)     | the one number a panel is about: HP, the turn, a roll total. **At most one per panel.**                  |
   - No other size: no `text-xs|sm|base|lg|xl|2xl…9xl` and no arbitrary `text-[…]` size in a swept
     file. Icons are sized with `size-*`, not with text sizes.
   - `cn()` (`web/src/lib/utils.ts`) knows the five tokens as font sizes, so `cn("text-body",
"text-muted-foreground")` keeps both.
   - Hierarchy comes from the role, weight and tone (`text-muted-foreground` for secondary text),
     never from a sixth size. *Amended by [ADR 0022](0022-activity-focus-layout.md) step 3.4:* a sixth role, `text-hero`
     (32 / 36 px), for the number a whole zone is about.

3. **Mapping old classes (for the sweeps).** Apply mechanically, then check the one-headline rule:

   | Old class                                                                   | New token       |
   | --------------------------------------------------------------------------- | --------------- |
   | `text-xs`, `text-[0.7rem]`…`text-[0.75rem]`                                 | `text-label`    |
   | `text-sm`, `text-base`, `text-[0.8rem]`                                     | `text-body`     |
   | a panel or card heading (`h2`/`h3`, `CardTitle`, popover title) at any size | `text-title`    |
   | `text-lg`, `text-xl` on a value                                             | `text-key`      |
   | `text-lg`, `text-xl` on a name or heading                                   | `text-title`    |
   | `text-2xl` and larger on the panel's main number                            | `text-headline` |
   | `text-2xl` and larger on anything else (a second number, a word, a verdict) | `text-key`      |

   Drop `font-black`, `tracking-*` and `uppercase` that came with large sizes (ADR 0009 step 5);
   keep `font-medium` where it marks a name.

4. **Spacing.** Panel and card padding is `p-3` (12 px) (*amended by [ADR 0022](0022-activity-focus-layout.md) step 3.2:* 14 px
   vertical, 16 px horizontal; 24 px between two text columns). Gaps and stacks use only 4, 8 or 12 px
   (`gap-1`, `gap-2`, `gap-3`; the same for `space-*`, `gap-x-*`, `gap-y-*`). Inner padding of a
   control or chip follows its height (step 5), not this rule.
5. **Control heights**, by component (`web/src/components/ui/`, `fieldStyles.ts`):

   | Control                                                                                 | Height                  | Notes                                                  |
   | --------------------------------------------------------------------------------------- | ----------------------- | ------------------------------------------------------ |
   | `Button` default, `sm`; `icon`, `icon-sm`                                               | 28 px (`h-7`, `size-7`) | text `text-body`                                       |
   | `Button` `lg`, `icon-lg`                                                                | 32 px (`h-8`, `size-8`) | primary actions only: Attack, Next turn, Cast          |
   | `Button` `xs`, `icon-xs`                                                                | 24 px (`h-6`, `size-6`) | only inside a dense row (timeline row, list row, chip) |
   | `Input`, `fieldClass` inputs and selects, `SelectTrigger` default                       | 28 px (`h-7`)           | text `text-body`                                       |
   | `SelectTrigger` `sm`                                                                    | 24 px (`h-6`)           | dense rows only                                        |
   | Clickable chips, segmented choices, menu items, list rows that are buttons (a save row) | 28 px                   |                                                        |
   | `Badge` (a non-clickable tag)                                                           | 20 px (`h-5`)           | text `text-label`                                      |

   No `h-9`, `h-10` or larger on a control, and no per-use height overrides above 32 px
   (*amended by [ADR 0022](0022-activity-focus-layout.md) step 3.3:* the Attack button, 44 px, is the one exception).

6. **Header.** *Amended by [ADR 0022](0022-activity-focus-layout.md) step 3.1:* the header is a card holding the brand, turn,
   initiative, timeline and last result, at most 120 px at rest. The header region holds the turn block (turn number as the headline, status
   line, Next turn, ⋯ menu) and the effects timeline side by side. The timeline is its turn row
   (24 px) plus one 24 px row per effect, and it shows at most 3 effect rows at rest, with no
   scrollbar. With the header's 8 px vertical padding that makes the header at most
   24 + 3 x 24 + 16 = **112 px** tall at rest; with fewer effects it is only as tall as they need
   (no empty rows reserved). With more than 3 effects an expander button in the "Effects" label
   cell (`aria-expanded`, shows the hidden count as "+N") grows the track in place, pushing the
   content down, to show all of them; collapsing returns to 3. The expanded track is capped at
   40% of the viewport height and scrolls only past that.
7. **Content target.** *Replaced by [ADR 0022](0022-activity-focus-layout.md) step 2.8 (the Moi column is gone).* Checked in `web/e2e/layout.spec.ts` ("layout" test) with bounding boxes, at
   the layout viewports 1280x800, 1440x900, 1707x1300, 1920x1080 and 2560x1440:
   - the header at rest (5 effects, collapsed) is at most 112 px tall at every viewport, the track
     has no scrollbar, and with 0, 1 and 3 effects the header grows with the rows;
   - at 1280x800, the Moi column shows the identity, the abilities, the HP block, the HP change
     and all five saving throws without scrolling;
   - at 1280x800, the Agir weapon-in-hand card is fully inside the workspace column.

## Consequences

- ADR 0009 step 3 (fluid root) and step 5's 12 px minimum are amended by this ADR; its other house
  rules (no gradients, tone steps, tabular figures, Inter, Phosphor, at most 3-4 sizes per panel)
  stand.
- A large screen now shows more content at the same size instead of the same content bigger. On
  the 5120x1440 ultrawide third (1707x1300) the type is the same as on a laptop.
- The whole app follows this ADR. Tailwind's default `text-*` sizes are removed from the theme
  (`--text-*: initial` in `web/src/index.css`), so `text-xs`..`text-9xl` build to no CSS and an old
  size is visibly missing instead of silently allowed.
- A transient inline form in the header (editing an effect) may make it taller while it is open;
  the 112 px limit is for the header at rest. The expanded track is taller by design.
- The header costs 48 px more than a one-row header (3 effect rows instead of one), the price of
  seeing three effects without scrolling. The Moi column still fits at 1280x800 (step 7).
- Not covered: mobile, a user-adjustable text size.
