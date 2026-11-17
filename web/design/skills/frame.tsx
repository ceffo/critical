// The dashboard around the Skills mock-ups (ADR 0022 step 2): header, character card, rail and dice
// footer on the app's real classes. The rail carries the Skills entry and its red pill (CRI-400).
import type { ReactNode } from "react";
import {
  ArrowRight,
  Backpack,
  BookOpenText,
  DotsThree,
  Drop,
  Gear,
  GraduationCap,
  HandFist,
  Heart,
  TrendUp,
} from "@phosphor-icons/react";
import { BrandIcon } from "@/components/shell/BrandMark";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { ABILITY, hasPending, type Ability, type Fixture } from "./data";
import { LANG, tr } from "./lang";

export const l2 = (p: [string, string]) => (LANG === "fr" ? p[0] : p[1]);

export function Header({ last }: { last?: ReactNode }) {
  return (
    <header className="zone-card flex shrink-0 items-stretch gap-4 px-4 py-3">
      <div className="group-band flex items-center gap-4 rounded-lg px-3 py-2">
        <div className="flex flex-col items-center">
          <span className="section-label">{tr("Tour", "Turn")}</span>
          <span className="text-headline tabular-nums">0</span>
        </div>
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <Button variant="main">
              {tr("Tour suivant", "Next turn")}
              <ArrowRight aria-hidden />
            </Button>
            <Button variant="quiet" size="icon">
              <DotsThree aria-hidden />
            </Button>
          </div>
          <span className="text-body text-ink-muted">
            {tr("Hors combat", "Out of combat")}
          </span>
        </div>
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="section-label">{tr("Effets", "Effects")}</span>
        <span className="text-body text-ink-muted">
          {tr("Aucun effet actif", "No active effect")}
        </span>
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

const ORDER: Ability[] = ["str", "dex", "con", "int", "wis", "cha"];

/** The six scores: in the card, and as the strip creation shows. `lit` marks the ones in play. */
export function Abilities({
  who,
  lit = [],
  thac0 = true,
}: {
  who: Fixture;
  lit?: Ability[];
  thac0?: boolean;
}) {
  const cells: [string, number, boolean][] = ORDER.map((a) => [
    ABILITY[a][LANG === "fr" ? 0 : 1],
    who.abilities[a],
    lit.includes(a),
  ]);
  if (thac0) cells.push([tr("TAC0", "THAC0"), who.thac0, false]);
  return (
    <div
      className={cn(
        "grid gap-1",
        thac0
          ? "grid-cols-[repeat(6,minmax(0,1fr))_auto]"
          : "grid-cols-6",
      )}
    >
      {cells.map(([n, v, on]) => (
        <div
          key={n}
          className={cn(
            "flex flex-col items-center rounded-md px-1 py-1",
            on ? "bg-selected ring-1 ring-tint" : "bg-raised",
          )}
        >
          <span className="text-label text-ink-muted">{n}</span>
          <span className="text-key tabular-nums">{v}</span>
        </div>
      ))}
    </div>
  );
}

export function CharacterCard({ who, lit }: { who: Fixture; lit?: Ability[] }) {
  return (
    <aside className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col gap-5 overflow-y-auto px-4 pb-3.5">
      <div className="group-band bleed-x-4 flex items-start justify-between gap-2 pt-3 pb-2">
        <div className="flex min-w-0 flex-col">
          <span className="text-title break-words">{who.name}</span>
          <span className="text-body text-ink-muted">
            {who.classes.map((c) => `${l2(c.name)} ${c.level}`).join(" / ")}
          </span>
        </div>
      </div>
      <Abilities who={who} lit={lit} />
      <div className="flex items-stretch gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="section-label">
            {tr("Points de vie", "Hit points")}
          </span>
          <span className="text-body text-ink-muted">
            <b className="text-hero leading-none text-ink tabular-nums">
              {who.hp}
            </b>{" "}
            / {who.hpMax}
          </span>
          <div className="mt-1 flex h-2 overflow-hidden rounded-full bg-input">
            <div
              className="bg-success"
              style={{ width: `${(who.hp / who.hpMax) * 100}%` }}
            />
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
            {who.ac}
          </span>
        </span>
      </div>
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
        {who.saves.map(([n, v]) => (
          <div
            key={n[1]}
            className="flex items-center justify-between rounded-md bg-raised px-2.5 py-1 text-body"
          >
            <span>{l2(n)}</span>
            <span className="font-medium tabular-nums">{v}+</span>
          </div>
        ))}
      </div>
    </aside>
  );
}

export type Activity = "act" | "spells" | "inventory" | "skills" | "xp";

/** The pill's looks the round compares (decision 7). */
export type PillLook = "dot" | "count";

export function Rail({
  active,
  pill,
  look = "dot",
  count = 3,
}: {
  active: Activity;
  pill: boolean;
  look?: PillLook;
  count?: number;
}) {
  const items: [Activity, typeof HandFist, string, string][] = [
    ["act", HandFist, "Agir", "Act"],
    ["spells", BookOpenText, "Sorts", "Spells"],
    ["inventory", Backpack, "Inventaire", "Inventory"],
    ["skills", GraduationCap, "Compétences", "Skills"],
    ["xp", TrendUp, "Expérience", "Experience"],
  ];
  return (
    <nav className="-mx-1.5 flex w-[5rem] shrink-0 flex-col items-stretch gap-1">
      {items.map(([a, Icon, fr, en]) => (
        <span
          key={a}
          className={cn(
            "relative flex flex-col items-center gap-1 rounded-lg px-0 py-2.5 text-center text-label break-words [&_svg]:size-5",
            a === active ? "selected font-medium" : "text-muted-foreground",
          )}
        >
          <Icon weight={a === active ? "fill" : "regular"} aria-hidden />
          <span className="max-w-full tracking-tight">{tr(fr, en)}</span>
          {a === "skills" && pill && look === "dot" && (
            <span data-signal="rail-dot" className="absolute top-1.5 right-2 size-2.5 rounded-full bg-danger ring-2 ring-background" />
          )}
          {a === "skills" && pill && look === "count" && (
            <span className="absolute top-1 right-1.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-danger px-1 text-[0.6875rem] leading-none font-semibold text-on-accent tabular-nums ring-2 ring-background">
              {count}
            </span>
          )}
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
    </footer>
  );
}

export function Dashboard({
  activity,
  who,
  pill,
  look,
  lit,
  children,
  last,
}: {
  activity: Activity;
  who: Fixture;
  pill?: boolean;
  look?: PillLook;
  lit?: Ability[];
  children: ReactNode;
  last?: ReactNode;
}) {
  return (
    <div className="relative flex h-full flex-col gap-3 overflow-hidden p-3">
      <Header last={last} />
      <div className="flex min-h-0 flex-1 gap-3">
        <CharacterCard who={who} lit={lit} />
        <Rail active={activity} pill={pill ?? hasPending(who)} look={look} />
        <main className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
          {children}
        </main>
      </div>
      <Footer />
    </div>
  );
}
