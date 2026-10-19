// The dashboard around the equipment mock-ups (ADR 0022 step 2): header, character card, rail,
// stage and dice footer, drawn on the app's real classes with Kelsen's data. Only the parts the
// equipment round changes are new: the AC in the character card, the initiative's declaration.
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
export function Header({ initTip }: { initTip?: "weapon" | "cast" }) {
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
            <span className="relative">
              <span className="text-hero leading-none tabular-nums">
                {initTip === "cast" ? "4" : "6"}
              </span>
              {initTip && <InitiativeTip kind={initTip} />}
            </span>
            <ArrowsClockwise className="size-4 text-ink-muted" aria-hidden />
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
 * The initiative's tooltip (hover and focus): the d10 and every modifier. No declaration (user,
 * round 1): Next turn adds the weapon in hand's speed; a spell cast or an item used in the round
 * corrects that round's initiative by itself, logged.
 */
function InitiativeTip({ kind }: { kind: "weapon" | "cast" }) {
  return (
    <span
      // Fixed under its number (measured once): a tooltip floats above the layout, so it never
      // widens the header's boxes (which the no-cut audit would read as cut text).
      ref={(el) => {
        const anchor = el?.parentElement?.getBoundingClientRect();
        if (el && anchor) {
          el.style.left = `${anchor.left}px`;
          el.style.top = `${anchor.bottom + 8}px`;
        }
      }}
      className="fixed z-20 flex w-max flex-col gap-0.5 rounded-md border border-border bg-popover px-3 py-2 text-body shadow-2xl"
    >
      <span>
        d10 <b>5</b> ·{" "}
        {kind === "cast"
          ? tr("Projectile magique", "Magic missile")
          : tr("Épée longue +2", "Long sword +2")}{" "}
        <b>{kind === "cast" ? "+1" : "+3"}</b> · {tr("Hâte", "Haste")} <b>−2</b>{" "}
        = <b>{kind === "cast" ? "4" : "6"}</b>
      </span>
      {kind === "cast" && (
        <span className="text-label text-ink-muted">
          {tr(
            "Corrigée par le sort lancé ce tour (était 6, Épée longue +2)",
            "Corrected by the spell cast this turn (was 6, Long sword +2)",
          )}
        </span>
      )}
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
        <span className="section-label">
          {tr("Modifier les PV", "HP change")}
        </span>
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
 * The character's AC (ADR 0033 step 10), every-turn information beside the hit points (user,
 * round 1): no label, the number inside a filled shield so it stands out, smaller than the HP.
 * A button opening the breakdown (its tooltip and aria-label say "Classe d'armure"). An override
 * draws the shield in the warning hue; a class warning adds a small badge.
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
  const value = override ? "4" : warning ? "−3" : "−2";
  return (
    <span
      className={cn(
        "relative flex size-16 shrink-0 items-center justify-center self-center rounded-lg",
        open && "bg-raised-hover",
      )}
      title={tr("Classe d'armure", "Armour class")}
      aria-label={`${tr("Classe d'armure", "Armour class")} ${value}`}
    >
      <svg viewBox="0 0 48 56" className="absolute inset-1 size-14" aria-hidden>
        <path
          d="M24 2 L44 9 V27 C44 40 35 49 24 54 C13 49 4 40 4 27 V9 Z"
          fill="var(--amb-800)"
          stroke={override ? "var(--warning)" : "var(--tint)"}
          strokeWidth="2"
        />
      </svg>
      <span className="relative -mt-1 text-key font-bold tabular-nums">
        {value}
      </span>
      {warning && (
        <Warning
          weight="fill"
          className="absolute top-0 right-0 size-4 text-warning"
          aria-label={tr("Avertissement de classe", "Class warning")}
        />
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
  initTip,
  card,
  overlay,
}: {
  activity: Activity;
  children: ReactNode;
  initTip?: "weapon" | "cast";
  card?: { acOverride?: boolean; acWarning?: boolean; acOpen?: boolean };
  overlay?: ReactNode;
}) {
  return (
    <div className="relative flex h-full flex-col gap-3 overflow-hidden p-3">
      <Header initTip={initTip} />
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
