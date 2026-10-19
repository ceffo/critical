// The dashboard around the equipment mock-ups (ADR 0022 step 2): header, character card, rail,
// stage and dice footer, drawn on the app's real classes with Kelsen's data. Only the parts the
// equipment round changes are new: the AC in the character card, the initiative's declaration.
import type { ReactNode } from "react";
import {
  ArrowRight,
  ArrowsClockwise,
  Backpack,
  BookOpenText,
  CaretDown,
  DotsThree,
  Drop,
  Gear,
  HandFist,
  Heart,
  Sword,
  TrendUp,
  Warning,
} from "@phosphor-icons/react";
import { BrandIcon } from "@/components/shell/BrandMark";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { lt, tr } from "./lang";
import { CHAR, SAVES } from "./data";

export type Activity = "act" | "spells" | "inventory" | "xp";

/** The header: the turn block (with the initiative's declaration), the effects, the brand and last result. */
export function Header({ declOpen }: { declOpen?: boolean }) {
  return (
    <header className="zone-card flex shrink-0 items-stretch gap-4 px-4 py-3">
      <div className="group-band flex items-center gap-4 rounded-lg px-3 py-2">
        <div className="flex flex-col items-center">
          <span className="section-label">{tr("Tour", "Turn")}</span>
          <span className="text-headline tabular-nums">2</span>
        </div>
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-2">
            <Button variant="main">
              {tr("Tour suivant", "Next turn")}
              <ArrowRight aria-hidden />
            </Button>
            <Button variant="quiet" size="icon">
              <DotsThree aria-hidden />
            </Button>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-body text-ink-muted">Initiative</span>
            <span className="text-hero leading-none tabular-nums">6</span>
            <ArrowsClockwise className="size-4 text-ink-muted" aria-hidden />
            <Declaration open={declOpen} />
          </div>
        </div>
      </div>
      <div className="grid min-w-0 flex-1 grid-cols-[auto_1fr] items-center gap-x-4 gap-y-1">
        <span className="section-label">{tr("Effets", "Effects")}</span>
        <div className="grid grid-cols-5 text-center text-label text-ink-muted">
          {["T2", "T3", "T4", "T5", "T6"].map((t) => (
            <span key={t}>{t}</span>
          ))}
        </div>
        {[
          [tr("Hâte", "Haste"), "40%"],
          [tr("Protection contre le mal", "Protection from evil"), "100%"],
        ].map(([n, w]) => (
          <div key={n} className="contents">
            <span className="text-body">{n}</span>
            <div className="h-5">
              <div
                className="h-5 rounded-full border border-tint bg-selected"
                style={{ width: w }}
              />
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
            <b className="text-key">15</b> {tr("Attaque", "Attack")} ·{" "}
            <span className="text-success">{tr("touché", "hit")}</span>
          </span>
        </div>
      </div>
    </header>
  );
}

/**
 * The declared action for the next round (ADR 0033 step 15): a small drop-down chip by the
 * initiative, the action's icon and modifier; its name in the tooltip and the menu.
 */
function Declaration({ open }: { open?: boolean }) {
  return (
    <span
      className={cn(
        "relative ml-1 flex h-7 items-center gap-1 rounded-md px-2 text-body",
        open ? "chip-chosen" : "chip",
      )}
      title={tr("Action déclarée : Épée longue +2, +3", "Declared: Long sword +2, +3")}
    >
      <Sword aria-hidden className="size-4" />
      <span className="tabular-nums">+3</span>
      <CaretDown aria-hidden className="size-3.5 text-ink-muted" />
    </span>
  );
}

/** The character card (ADR 0022 step 2.3) with the character's AC beside the hit points. */
export function CharacterCard({
  acOverride,
  acWarning,
  acOpen,
}: {
  acOverride?: boolean;
  acWarning?: boolean;
  acOpen?: boolean;
}) {
  return (
    <aside className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col gap-5 overflow-y-auto px-4 pb-3.5">
      <div className="group-band bleed-x-4 -mx-0 flex items-start justify-between gap-2 pt-3 pb-2">
        <div className="flex flex-col">
          <span className="text-title">{CHAR.name}</span>
          <span className="text-body text-ink-muted">{lt(CHAR.cls)}</span>
        </div>
        <span className="text-body text-ink-muted">
          {tr("Niveau", "Level")} {CHAR.level}
        </span>
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
      <div className="flex flex-col gap-2">
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
              <div className="hatch-danger w-[8%]" />
              <div className="w-[82%] bg-success" />
            </div>
          </div>
          <AcTile override={acOverride} warning={acWarning} open={acOpen} />
        </div>
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

/**
 * The character's AC (ADR 0033 step 10): every-turn information (ADR 0027 step 1.1), so on the
 * card at the hit points' size; a button that opens its breakdown. An override shows as the
 * number with an "override" tag; a class warning as an amber mark.
 */
function AcTile({
  override,
  warning,
  open,
}: {
  override?: boolean;
  warning?: boolean;
  open?: boolean;
}) {
  return (
    <span
      className={cn(
        "flex w-[5.5rem] shrink-0 flex-col items-center justify-center gap-0.5 rounded-lg px-2 py-1.5",
        open ? "chip-chosen" : "bg-raised",
      )}
      title={tr("Détail de la CA", "AC breakdown")}
    >
      <span className="flex items-center gap-1 section-label">
        {tr("CA", "AC")}
        {warning && <Warning weight="fill" className="size-3.5 text-warning" />}
      </span>
      <span className="text-hero leading-none tabular-nums">
        {override ? "4" : warning ? "−3" : "−2"}
      </span>
      {override && (
        <span className="text-label text-warning">{tr("forcée", "override")}</span>
      )}
    </span>
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
      <span className="text-label text-ink-muted">
        {tr("Aucun favori", "No favourite yet")}
      </span>
    </footer>
  );
}

/** The whole screen: header, character card, rail, the stage's content, footer. */
export function Dashboard({
  activity,
  children,
  declOpen,
  card,
  overlay,
}: {
  activity: Activity;
  children: ReactNode;
  declOpen?: boolean;
  card?: { acOverride?: boolean; acWarning?: boolean; acOpen?: boolean };
  overlay?: ReactNode;
}) {
  return (
    <div className="relative flex h-full flex-col gap-3 overflow-hidden p-3">
      <Header declOpen={declOpen} />
      <div className="flex min-h-0 flex-1 gap-3">
        <CharacterCard {...card} />
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
