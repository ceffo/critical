// The dashboard around the psionics mock-ups (ADR 0022 step 2): header, character card, rail,
// stage and dice footer on the app's real classes, with the psionicist's parts the round decides:
// the PSP block in the character card, the maintenance a Next Round would charge, and the rail
// without a Powers activity (powers are played on Act, browsed under Spells).
import type { ReactNode } from "react";
import {
  ArrowRight,
  ArrowsClockwise,
  Backpack,
  BookOpenText,
  DotsThree,
  Drop,
  Gear,
  HandFist,
  Heart,
  Moon,
  SlidersHorizontal,
  TrendUp,
} from "@phosphor-icons/react";
import { BrandIcon } from "@/components/shell/BrandMark";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { lt, tr } from "./lang";
import { CHAR, SAVES } from "./data";

export type Activity = "act" | "spells" | "inventory" | "xp";

export interface Effect {
  name: string;
  /** Width of its bar, 0 to 100 (100 for one that lasts until stopped). */
  width: number;
  note?: string;
}

/** The header: the turn block with the PSPs the next round charges, the effects, brand and last result. */
export function Header({
  effects,
  upkeep,
  round = 3,
  last,
}: {
  effects: Effect[];
  upkeep: number;
  round?: number;
  last?: ReactNode;
}) {
  return (
    <header className="zone-card flex shrink-0 items-stretch gap-4 px-4 py-3">
      <div className="group-band flex items-center gap-4 rounded-lg px-3 py-2">
        <div className="flex flex-col items-center">
          <span className="section-label">{tr("Tour", "Turn")}</span>
          <span className="text-headline tabular-nums">{round}</span>
        </div>
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <Button variant="main" className="h-auto flex-col items-start gap-0 py-1">
              <span className="flex items-center gap-2">
                {tr("Tour suivant", "Next turn")}
                <ArrowRight aria-hidden />
              </span>
              {upkeep > 0 && (
                <span className="text-label">
                  {tr(`−${upkeep} PFP d'entretien`, `−${upkeep} PSP upkeep`)}
                </span>
              )}
            </Button>
            <Button variant="quiet" size="icon">
              <DotsThree aria-hidden />
            </Button>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-body text-ink-muted">Initiative</span>
            <span className="text-hero leading-none tabular-nums">7</span>
            <ArrowsClockwise className="size-4 text-ink-muted" aria-hidden />
          </div>
        </div>
      </div>
      <div className="grid min-w-0 flex-1 grid-cols-[auto_1fr] items-center gap-x-4 gap-y-1">
        <span className="section-label">{tr("Effets", "Effects")}</span>
        <div className="grid grid-cols-5 text-center text-label text-ink-muted">
          {["T3", "T4", "T5", "T6", "T7"].map((t) => (
            <span key={t}>{t}</span>
          ))}
        </div>
        {effects.map((e) => (
          <div key={e.name} className="contents">
            <span className="text-body">{e.name}</span>
            <div className="h-5">
              <div
                className="flex h-5 items-center rounded-full border border-tint bg-selected px-2 text-label text-ink-muted"
                style={{ width: `${e.width}%` }}
              >
                {e.note}
              </div>
            </div>
          </div>
        ))}
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1">
        <div className="flex items-center gap-2">
          <BrandIcon className="size-6" />
          <span className="font-display text-key font-bold">
            Critical<span className="text-highlight">!</span>
          </span>
        </div>
        <div className="group-band flex flex-col items-center rounded-lg px-3 py-1">
          <span className="section-label">
            {tr("Dernier résultat", "Last result")}
          </span>
          <span className="text-body">
            {last ?? tr("Aucun jet pour l'instant.", "No roll yet.")}
          </span>
        </div>
      </div>
    </header>
  );
}

/**
 * The PSP block of the character card: the pool is every-round information beside the hit points,
 * so it is a number and a bar, with what the next round charges and the way into resting. The
 * manual adjust and the recovery form leave the card (the rest popover and the cog).
 */
export function PspBlock({
  current,
  max,
  upkeep,
  restOpen,
}: {
  current: number;
  max: number;
  upkeep: number;
  restOpen?: boolean;
}) {
  const pct = Math.round((current / max) * 100);
  return (
    <div className="flex flex-col gap-1.5">
      <span className="section-label">
        {tr("Points de force psionique", "Psionic strength points")}
      </span>
      <div className="flex items-end justify-between gap-2">
        <span className="text-body text-ink-muted">
          <b className="text-hero leading-none text-ink tabular-nums">
            {current}
          </b>{" "}
          / {max}
        </span>
        {upkeep > 0 && (
          <span className="text-label text-ink-muted">
            {tr(
              `Tour suivant : −${upkeep} → ${current - upkeep}`,
              `Next turn: −${upkeep} → ${current - upkeep}`,
            )}
          </span>
        )}
      </div>
      <div className="flex h-2 overflow-hidden rounded-full bg-input">
        <div className="bg-magic" style={{ width: `${pct}%` }} />
      </div>
      <div className="mt-1 flex gap-2">
        <Button
          variant="quiet"
          className={cn("flex-1", restOpen && "bg-raised-hover")}
          aria-expanded={restOpen}
        >
          <Moon aria-hidden />
          {tr("Repos", "Rest")}
        </Button>
        <Button variant="quiet" className="flex-1 border-dashed">
          <SlidersHorizontal aria-hidden />
          {tr("Ajuster", "Adjust")}
        </Button>
      </div>
    </div>
  );
}

export function CharacterCard({
  single,
  psp,
  upkeep,
  restOpen,
}: {
  /** A single-class psionicist instead of the table's thief/psionicist. */
  single?: boolean;
  psp: number;
  upkeep: number;
  restOpen?: boolean;
}) {
  const classes = single ? [CHAR.classes[1]] : CHAR.classes;
  return (
    <aside className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col gap-5 overflow-y-auto px-4 pb-3.5">
      <div className="group-band bleed-x-4 flex items-start justify-between gap-2 pt-3 pb-2">
        <div className="flex flex-col">
          <span className="text-title">{CHAR.name}</span>
          <span className="text-body text-ink-muted">
            {classes.map((c) => `${lt(c.name)} ${c.level}`).join(" · ")}
          </span>
        </div>
      </div>
      <div className="grid grid-cols-[repeat(6,minmax(0,1fr))_auto] gap-1">
        {CHAR.abilities.map(([n, v]) => (
          <div
            key={n.en}
            className="flex flex-col items-center rounded-md bg-raised px-1 py-1"
          >
            <span className="text-label text-ink-muted">{lt(n)}</span>
            <span className="text-key tabular-nums">{v}</span>
          </div>
        ))}
      </div>
      <div className="flex items-stretch gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="section-label">
            {tr("Points de vie", "Hit points")}
          </span>
          <span className="text-body text-ink-muted">
            <b className="text-hero leading-none text-ink tabular-nums">
              {CHAR.hp}
            </b>{" "}
            / {CHAR.hpMax}
          </span>
          <div className="mt-1 flex h-2 overflow-hidden rounded-full bg-input">
            <div className="w-[92%] bg-success" />
          </div>
        </div>
        <span className="relative flex size-16 shrink-0 items-center justify-center self-center">
          <svg viewBox="0 0 48 56" className="absolute inset-1 size-14" aria-hidden>
            <path
              d="M24 2 L44 9 V27 C44 40 35 49 24 54 C13 49 4 40 4 27 V9 Z"
              fill="var(--amb-800)"
              stroke="var(--tint)"
              strokeWidth="2"
            />
          </svg>
          <span className="relative -mt-1 text-key font-bold tabular-nums">
            {CHAR.ac}
          </span>
        </span>
      </div>
      <PspBlock
        current={psp}
        max={CHAR.pspMax}
        upkeep={upkeep}
        restOpen={restOpen}
      />
      <div className="flex flex-col gap-2">
        <span className="section-label">{tr("Modifier les PV", "HP change")}</span>
        <div className="flex gap-2">
          <span className="flex h-7 w-14 items-center rounded-md border border-input px-2 text-body text-ink-muted">
            {tr("PV", "HP")}
          </span>
          <Button variant="quiet" className="flex-1 border-dashed">
            <Drop aria-hidden />
            {tr("Dégâts", "Damage")}
          </Button>
          <Button variant="quiet" className="flex-1 border-dashed">
            <Heart aria-hidden />
            {tr("Soins", "Healing")}
          </Button>
        </div>
      </div>
      <div className="flex flex-col gap-1.5">
        <span className="section-label">
          {tr("Jets de sauvegarde", "Saving throws")}
        </span>
        {SAVES.map(([n, v]) => (
          <div
            key={n.en}
            className="flex items-center justify-between rounded-md bg-raised px-2.5 py-1 text-body"
          >
            <span>{lt(n)}</span>
            <span className="font-medium tabular-nums">{v}</span>
          </div>
        ))}
      </div>
    </aside>
  );
}

export function Rail({ active }: { active: Activity }) {
  const items: [Activity, typeof HandFist, string, string][] = [
    ["act", HandFist, "Agir", "Act"],
    ["spells", BookOpenText, "Sorts", "Spells"],
    ["inventory", Backpack, "Inventaire", "Inventory"],
    ["xp", TrendUp, "Expérience", "Experience"],
  ];
  return (
    <nav className="flex w-[4.25rem] shrink-0 flex-col items-stretch gap-1">
      {items.map(([a, Icon, fr, en]) => (
        <span
          key={a}
          className={cn(
            "flex flex-col items-center gap-1 rounded-lg px-0.5 py-2.5 text-center text-label break-words [&_svg]:size-5",
            a === active ? "selected font-medium" : "text-muted-foreground",
          )}
        >
          <Icon weight={a === active ? "fill" : "regular"} aria-hidden />
          <span className="max-w-full">{tr(fr, en)}</span>
        </span>
      ))}
    </nav>
  );
}

export function Footer() {
  return (
    <footer className="zone-card flex min-w-0 shrink-0 items-center gap-3 px-3 py-2">
      <Gear className="size-5 text-ink-muted" aria-hidden />
      <span className="section-label">{tr("Dés", "Dice")}</span>
      {["d4", "d6", "d8", "d10", "d12", "d20", "d%"].map((d) => (
        <span key={d} className="chip px-2 py-1 text-body">
          {d}
        </span>
      ))}
    </footer>
  );
}

/** The whole screen: header, character card, rail, the stage's content, footer. */
export function Dashboard({
  activity,
  children,
  effects,
  upkeep,
  psp = CHAR.psp,
  single,
  last,
  restOpen,
  overlay,
  round,
}: {
  activity: Activity;
  children: ReactNode;
  effects: Effect[];
  upkeep: number;
  psp?: number;
  single?: boolean;
  last?: ReactNode;
  restOpen?: boolean;
  overlay?: ReactNode;
  round?: number;
}) {
  return (
    <div className="relative flex h-full flex-col gap-3 overflow-hidden p-3">
      <Header effects={effects} upkeep={upkeep} last={last} round={round} />
      <div className="flex min-h-0 flex-1 gap-3">
        <CharacterCard
          single={single}
          psp={psp}
          upkeep={upkeep}
          restOpen={restOpen}
        />
        <Rail active={activity} />
        <main className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
          {children}
        </main>
      </div>
      <Footer />
      {overlay}
    </div>
  );
}
