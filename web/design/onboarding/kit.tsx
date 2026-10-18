// Shared pieces of the onboarding mock-ups: the brand, the creation frame (header, step list,
// step card, the character being built) and small controls, all on the app's real classes.
import type { ReactNode } from "react";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Prohibit,
  SignOut,
  Warning,
  X,
} from "@phosphor-icons/react";
import { BrandIcon } from "@/components/shell/BrandMark";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import { tr } from "./lang";
import { ABILITIES, CHAR, STEPS } from "./data";

export function Wordmark({ size = "key" }: { size?: "key" | "hero" }) {
  return (
    <div className="flex shrink-0 items-center gap-2">
      <BrandIcon className={size === "hero" ? "size-12" : "size-7"} />
      <span
        className={cn(
          "font-display leading-none font-bold tracking-wide select-none",
          size === "hero" ? "text-hero" : "text-key",
        )}
      >
        Critical<span className="text-highlight">!</span>
      </span>
    </div>
  );
}

export function LangSwitch() {
  const fr = tr("fr", "en") === "fr";
  return (
    <div className="flex gap-1">
      {[
        ["English", !fr],
        ["Français", fr],
      ].map(([l, on]) => (
        <span
          key={String(l)}
          className={cn(
            "min-h-7 rounded-md px-2 py-1 text-label",
            on ? "chip-chosen" : "chip",
          )}
        >
          {l}
        </span>
      ))}
    </div>
  );
}

export function Logout() {
  return (
    <Button variant="quiet">
      <SignOut aria-hidden />
      {tr("Se déconnecter", "Log out")}
    </Button>
  );
}

/** A note under a control: a warning (amber, never blocks) or a refusal (red, blocks). */
export function Note({
  kind,
  children,
}: {
  kind: "warning" | "refusal" | "info";
  children: ReactNode;
}) {
  const Icon =
    kind === "refusal" ? Prohibit : kind === "warning" ? Warning : Check;
  return (
    <p
      className={cn(
        "flex items-start gap-2 rounded-md px-3 py-2 text-body",
        kind === "refusal" &&
          "bg-raised text-danger shadow-[inset_3px_0_0_var(--danger)]",
        kind === "warning" &&
          "bg-raised text-warning shadow-[inset_3px_0_0_var(--warning)]",
        kind === "info" && "bg-raised text-ink-muted",
      )}
    >
      <Icon aria-hidden weight="bold" className="mt-0.5 size-4 shrink-0" />
      <span className="min-w-0">{children}</span>
    </p>
  );
}

/** A labelled text field showing a value. */
export function Field({
  label,
  value,
  unit,
  hint,
  className,
  invalid,
  wide,
}: {
  label: string;
  value: string;
  unit?: string;
  hint?: ReactNode;
  className?: string;
  invalid?: boolean;
  wide?: boolean;
}) {
  return (
    <div className={cn("flex min-w-0 flex-col gap-1", className)}>
      <span className={labelClass}>{label}</span>
      <div className="flex items-center gap-2">
        <span
          className={cn(
            fieldClass,
            "flex items-center",
            wide ? "w-full" : "w-24",
            invalid && "border-destructive",
          )}
        >
          {value}
        </span>
        {unit && <span className="text-body text-ink-muted">{unit}</span>}
      </div>
      {hint && <span className="text-label text-ink-muted">{hint}</span>}
    </div>
  );
}

/** A segmented choice (radio look of the settings popover). */
export function Seg({
  options,
  chosen,
  className,
}: {
  options: ReactNode[];
  chosen: number;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-wrap gap-1", className)}>
      {options.map((o, i) => (
        <span
          key={i}
          className={cn(
            "flex min-h-7 items-center justify-center gap-1 rounded-md px-3 py-1 text-body [&_svg]:size-4",
            i === chosen ? "chip-chosen" : "chip",
          )}
        >
          {o}
        </span>
      ))}
    </div>
  );
}

/** One die of a roll's breakdown; a dropped die is struck through. */
export function Die({ v, dropped }: { v: number; dropped?: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex h-6 min-w-6 items-center justify-center rounded-md px-1 text-body tabular-nums",
        dropped
          ? "text-ink-muted line-through decoration-2"
          : "bg-raised-hover font-medium",
      )}
    >
      {v}
    </span>
  );
}

/** A card head: the section label on the band. */
export function CardHead({
  children,
  right,
}: {
  children: ReactNode;
  right?: ReactNode;
}) {
  return (
    <div className="card-band flex min-h-9 items-center justify-between gap-3 px-4 py-2">
      <h2 className="section-label">{children}</h2>
      {right}
    </div>
  );
}

/** A group heading inside a step card. */
export function Group({
  title,
  right,
  children,
  className,
}: {
  title: string;
  right?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("flex flex-col gap-3", className)}>
      <div className="group-heading flex items-center justify-between gap-3 px-3">
        <h3 className="section-label">{title}</h3>
        {right}
      </div>
      {children}
    </section>
  );
}

/** Where the creation stands: the steps as a vertical list, done ones with the choice made. */
function StepList({ current }: { current: number }) {
  return (
    <nav className="zone-card flex w-56 shrink-0 flex-col">
      <CardHead>{tr("Étapes", "Steps")}</CardHead>
      <ol className="flex flex-col gap-1 p-2">
        {STEPS.map((s, i) => {
          const done = i < current;
          const on = i === current;
          return (
            <li
              key={s.id}
              className={cn(
                "flex items-start gap-3 rounded-md px-2 py-2",
                on && "selected",
              )}
            >
              <span
                className={cn(
                  "flex size-6 shrink-0 items-center justify-center rounded-full text-label font-semibold",
                  done && "bg-tint text-on-accent",
                  on && "shadow-[inset_0_0_0_1.5px_var(--tint)] text-tint",
                  !done &&
                    !on &&
                    "shadow-[inset_0_0_0_1px_var(--input)] text-ink-muted",
                )}
              >
                {done ? <Check weight="bold" className="size-3.5" /> : i + 1}
              </span>
              <span className="flex min-w-0 flex-col">
                <span
                  className={cn(
                    "text-body",
                    on ? "font-semibold" : done ? "" : "text-ink-muted",
                  )}
                >
                  {tr(s.fr, s.en)}
                </span>
                {done && (
                  <span className="text-label text-ink-muted">
                    {tr(s.doneFr, s.doneEn)}
                  </span>
                )}
              </span>
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

/** The character being built, filled step by step (the shape of the dashboard's character card). */
export function Building({ upTo }: { upTo: number }) {
  const has = (step: number) => upTo > step;
  const dash = <span className="text-ink-muted">—</span>;
  return (
    <aside className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col overflow-y-auto">
      <CardHead>{tr("Votre personnage", "Your character")}</CardHead>
      <div className="flex flex-col gap-4 px-4 pt-3 pb-4">
        <div className="flex flex-col gap-0.5">
          <span className="text-title break-words">
            {has(4) ? (
              CHAR.name
            ) : (
              <span className="text-ink-muted">
                {tr("Sans nom", "No name yet")}
              </span>
            )}
          </span>
          <span className="text-body text-ink-muted">
            {has(0) ? tr("Barde", "Bard") : "—"}
            {has(1) ? tr(" · Demi-elfe", " · Half-elf") : ""}
            {has(0) ? tr(" · niveau 5", " · level 5") : ""}
          </span>
        </div>
        <div className="grid grid-cols-6 gap-1">
          {ABILITIES.map((a) => (
            <div
              key={a.id}
              className="flex flex-col items-center rounded-md bg-raised py-1"
            >
              <span className="text-label text-ink-muted">
                {tr(a.fr, a.en)}
              </span>
              <span className="text-key tabular-nums">
                {has(2) ? a.score : dash}
              </span>
            </div>
          ))}
        </div>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-body">
          <dt className="text-ink-muted">
            {tr("Points de vie", "Hit points")}
          </dt>
          <dd>{has(3) ? "20" : dash}</dd>
          <dt className="text-ink-muted">{tr("Talents", "Skills")}</dt>
          <dd>{has(3) ? tr("78 / 80 points", "78 / 80 points") : dash}</dd>
          <dt className="text-ink-muted">{tr("Alignement", "Alignment")}</dt>
          <dd>{has(4) ? tr("Neutre bon", "Neutral good") : dash}</dd>
          <dt className="text-ink-muted">{tr("Sexe", "Sex")}</dt>
          <dd>{has(4) ? tr("Féminin", "Female") : dash}</dd>
          <dt className="text-ink-muted">{tr("Langues", "Languages")}</dt>
          <dd>
            {has(4)
              ? tr(
                  "Commun, elfe, gnome, halfelin, orque",
                  "Common, elf, gnome, halfling, orc",
                )
              : dash}
          </dd>
          <dt className="text-ink-muted">
            {tr("Taille, poids", "Height, weight")}
          </dt>
          <dd>{has(4) ? tr("1,70 m · 48 kg", "5′7″ · 105 lb") : dash}</dd>
          <dt className="text-ink-muted">{tr("Âge", "Age")}</dt>
          <dd>{has(4) ? tr("19 ans", "19 years") : dash}</dd>
        </dl>
        {has(4) && (
          <Note kind="warning">
            {tr("1 avertissement : langues", "1 warning: languages")}
          </Note>
        )}
      </div>
    </aside>
  );
}

/**
 * The creation route: a header band (brand, title, leave), then the step list, the step's card
 * (its content scrolls inside, Back / Next at its foot) and the character being built.
 */
export function CreationFrame({
  step,
  title,
  lead,
  children,
  next,
  nextBlocked,
  footNote,
}: {
  step: number;
  title: string;
  lead?: string;
  children: ReactNode;
  next?: string;
  nextBlocked?: boolean;
  footNote?: ReactNode;
}) {
  return (
    <div className={"mx-auto flex h-full max-w-[1680px] flex-col gap-3 p-3"}>
      <header className="zone-card flex items-center gap-4 px-4 py-2">
        <Wordmark />
        <span className="text-title">
          {tr("Nouveau personnage", "New character")}
        </span>
        <span className="flex-1" />
        <Button variant="quiet">
          <X aria-hidden />
          {tr("Quitter", "Leave")}
        </Button>
      </header>
      <div className="flex min-h-0 flex-1 gap-3">
        <StepList current={step} />
        <main className="stage-card flex min-w-0 flex-1 flex-col">
          <div className="card-band flex flex-col gap-0.5 px-5 py-3">
            <span className="text-label text-ink-muted">
              {tr(`Étape ${step + 1} sur 6`, `Step ${step + 1} of 6`)}
            </span>
            <h1 className="text-headline">{title}</h1>
            {lead && <p className="text-body text-ink-muted">{lead}</p>}
          </div>
          <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-5 py-4">
            {children}
          </div>
          <div className="flex items-center gap-3 border-t border-border px-5 py-3">
            {step > 0 && (
              <Button variant="quiet" size="lg">
                <ArrowLeft aria-hidden />
                {tr("Retour", "Back")}
              </Button>
            )}
            <span className="min-w-0 flex-1 text-body text-ink-muted">
              {footNote}
            </span>
            <Button variant="main" size="lg" disabled={nextBlocked}>
              {next ?? tr("Suivant", "Next")}
              {!next && <ArrowRight aria-hidden />}
            </Button>
          </div>
        </main>
        <Building upTo={step} />
      </div>
    </div>
  );
}
