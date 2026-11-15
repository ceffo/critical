// The mage school design round's screens (docs/design/mage-school/README.md): where a player finds
// the switch to a school of magic (ADR 0050 step 15), on the dashboard's real classes and buttons.
// Every screen is a proposal until the user approves it. A dashed magenta outline marks what the
// round adds or changes; it is a review mark, not part of the design.
import type { ReactNode } from "react";
import {
  ArrowRight,
  ArrowsLeftRight,
  Backpack,
  BookOpenText,
  CaretDown,
  Check,
  DotsThree,
  Drop,
  Gear,
  HandFist,
  Heart,
  MagicWand,
  Plus,
  Sword,
  TrendUp,
  Warning,
} from "@phosphor-icons/react";
import { BrandIcon } from "@/components/shell/BrandMark";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { lt, tr } from "./lang";
import {
  ABILITY_NAMES,
  BOOK,
  DUAL,
  MAGE,
  RACES,
  SCHOOLS,
  SPECIALISTS,
  meets,
  raceListed,
  type Ability,
  type Fixture,
} from "./data";

/** The review mark around what the round adds or changes. */
function New({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        "rounded-md outline-2 outline-offset-2 outline-[#d6336c] outline-dashed",
        className,
      )}
    >
      {children}
    </span>
  );
}

// ---------------------------------------------------------------------------------------------
// The dashboard around the proposals (ADR 0022 step 2), as the user saw it in the screenshots.

function Header() {
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
          <span className="text-label text-ink-muted">
            {tr("Hors combat · l'initiative sera lancée", "Out of combat · initiative will be rolled")}
          </span>
        </div>
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-1 py-1">
        <span className="flex items-center gap-3">
          <span className="section-label">{tr("Effets", "Effects")}</span>
          <Button variant="ghost" size="xs">
            <Plus aria-hidden />
            {tr("Ajouter", "Add")}
          </Button>
        </span>
        <span className="text-body text-ink-muted">{tr("Aucun effet actif", "No active effect")}</span>
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1">
        <div className="flex items-center gap-2">
          <BrandIcon className="size-6" />
          <span className="font-display text-key font-bold">
            Critical<span className="text-highlight">!</span>
          </span>
        </div>
        <div className="group-band flex flex-col items-center rounded-lg px-3 py-1">
          <span className="section-label">{tr("Dernier résultat", "Last result")}</span>
          <span className="text-body">{tr("Aucun jet pour l'instant.", "No roll yet.")}</span>
        </div>
      </div>
    </header>
  );
}

const entryText = (f: Fixture) => f.entries.map((e) => `${lt(e.name)} ${e.level}`).join(" / ");

/**
 * The character card. `classLine` replaces the class under the name (variant A's tag, variant B's
 * link); `entriesLine` replaces the class-entries summary of a dual-class character.
 */
function CharacterCard({
  who,
  className: cls,
  classLine,
  entriesLine,
  nameHint,
}: {
  who: Fixture;
  className?: string;
  classLine?: ReactNode;
  entriesLine?: ReactNode;
  /** The name's way into the details shows a caret (the popover proposal). */
  nameHint?: boolean;
}) {
  const several = who.entries.length > 1;
  const abilities: [string, number][] = [
    ...(Object.keys(ABILITY_NAMES) as Ability[]).map((k): [string, number] => [lt(ABILITY_NAMES[k]), who.abilities[k]]),
    [tr("TAC0", "THAC0"), who.thac0],
  ];
  return (
    <aside className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col gap-4 overflow-y-auto px-4 pb-3.5">
      <div className="group-band bleed-x-4 flex min-w-0 items-baseline justify-between gap-2 py-2">
        <div className="flex min-w-0 flex-1 flex-col">
          <h1 className="min-w-0">
            <span className="inline-flex items-center gap-1 text-key text-amb-300">
              {who.name}
              {nameHint && (
                <New>
                  <CaretDown aria-hidden className="size-3.5" />
                </New>
              )}
            </span>
          </h1>
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            {classLine ?? (!several && <p className="text-body text-muted-foreground">{cls ?? lt(who.entries[0].name)}</p>)}
            <span className="inline-flex h-5 shrink-0 items-center rounded-full bg-selected px-2 text-label font-medium">
              {tr("Assigner", "Assign")}
            </span>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {!several && (
            <span className="relative pb-1 text-body text-muted-foreground tabular-nums">
              {tr("Niveau", "Level")} {who.entries[0].level}
              <span aria-hidden className="absolute inset-x-0 bottom-0 h-0.5 rounded-full bg-input">
                <span className="absolute inset-y-0 left-0 w-[40%] rounded-full bg-tint" />
              </span>
            </span>
          )}
          <Button variant="ghost" size="icon-sm" className="shrink-0 shadow-[inset_0_0_0_1.5px_var(--tint)]">
            <ArrowsLeftRight aria-hidden />
          </Button>
        </div>
      </div>
      {several && entriesLine}
      {several && !entriesLine && (
        <span className="flex w-full min-w-0 items-baseline gap-x-1.5 rounded-md bg-raised px-2.5 py-1 text-body font-medium">
          {entryText(who)}
        </span>
      )}
      <div className="grid grid-cols-[repeat(6,minmax(0,1fr))_auto] gap-1">
        {abilities.map(([n, v]) => (
          <div key={n} className="flex min-w-0 flex-col items-center rounded-md bg-raised px-1 py-1">
            <span className="text-label text-muted-foreground">{n}</span>
            <span className="text-key tabular-nums">{v}</span>
          </div>
        ))}
      </div>
      <div className="flex items-stretch gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <span className="section-label">{tr("Points de vie", "Hit points")}</span>
          <span className="text-body text-ink-muted">
            <b className="text-hero leading-none text-ink tabular-nums">{who.hp}</b> / {who.hpMax}
          </span>
          <div className="mt-1 flex h-2 overflow-hidden rounded-full bg-input">
            <div className="w-full bg-success" />
          </div>
        </div>
        <span className="relative flex size-16 shrink-0 items-center justify-center self-center">
          <svg viewBox="0 0 48 56" className="absolute inset-1 size-14" aria-hidden>
            <path d="M24 2 L44 9 V27 C44 40 35 49 24 54 C13 49 4 40 4 27 V9 Z" fill="var(--amb-800)" stroke="var(--tint)" strokeWidth="2" />
          </svg>
          <span className="relative -mt-1 text-key font-bold tabular-nums">{who.ac}</span>
        </span>
      </div>
      <div className="flex flex-col gap-2">
        <span className="section-label">{tr("Modifier les PV", "HP change")}</span>
        <div className="flex gap-2">
          <span className="flex h-7 w-14 items-center rounded-md border border-input px-2 text-body text-ink-muted">{tr("PV", "HP")}</span>
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
        <span className="section-label">{tr("Jets de sauvegarde", "Saving throws")}</span>
        {who.saves.map(([n, v]) => (
          <div key={n.en} className="flex items-center justify-between rounded-md bg-raised px-2.5 py-1 text-body">
            <span>{lt(n)}</span>
            <span className="font-medium tabular-nums">{v}</span>
          </div>
        ))}
      </div>
    </aside>
  );
}

type Activity = "act" | "spells" | "inventory" | "xp";

function Rail({ active }: { active: Activity }) {
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

function Footer() {
  return (
    <footer className="zone-card flex min-w-0 shrink-0 items-center gap-3 px-3 py-2">
      <Gear className="size-5 text-ink-muted" aria-hidden />
      <span className="section-label">{tr("Dés", "Dice")}</span>
    </footer>
  );
}

/** The Act stage as in the screenshots: the weapon in hand. */
function ActStage() {
  return (
    <section className="zone-card flex min-h-0 flex-1 flex-col gap-3 p-4">
      <div className="flex gap-6 text-body">
        <span className="font-medium">{tr("Armes", "Weapons")}</span>
        <span className="text-muted-foreground">{tr("Sorts", "Spells")} 0</span>
        <span className="text-muted-foreground">{tr("Objets", "Items")} 3</span>
        <span className="text-muted-foreground">{tr("Compétences", "Skills")}</span>
      </div>
      <div className="selected flex flex-col gap-2 rounded-xl p-4">
        <span className="section-label">{tr("Arme en main", "Weapon in hand")}</span>
        <span className="flex items-center justify-between gap-3">
          <span className="text-key">{tr("Épée bâtarde +2", "Bastard sword +2")}</span>
          <Button variant="main">
            <Sword aria-hidden />
            {tr("Attaquer", "Attack")}
          </Button>
        </span>
      </div>
    </section>
  );
}

function Journal() {
  return (
    <section className="zone-card flex h-[28%] shrink-0 flex-col gap-2 p-4">
      <span className="section-label">{tr("Journal", "Journal")}</span>
      <span className="text-body text-ink-muted">{tr("Rien d'enregistré pour l'instant.", "Nothing recorded yet.")}</span>
    </section>
  );
}

function Dashboard({
  who = MAGE,
  activity = "act",
  card,
  stage,
  overlay,
}: {
  who?: Fixture;
  activity?: Activity;
  card?: ReactNode;
  stage?: ReactNode;
  overlay?: ReactNode;
}) {
  return (
    <div className="relative flex h-full flex-col gap-3 overflow-hidden p-3">
      <Header />
      <div className="flex min-h-0 flex-1 gap-3">
        {card ?? <CharacterCard who={who} />}
        <Rail active={activity} />
        <main className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
          {stage ?? <ActStage />}
          <Journal />
        </main>
      </div>
      <Footer />
      {overlay}
    </div>
  );
}

// ---------------------------------------------------------------------------------------------
// Variant A: a tag on the class line, while the mage has no school.

function SchoolTag() {
  return (
    <New>
      <span className="inline-flex h-5 shrink-0 items-center gap-1 rounded-full border border-dashed border-tint px-2 text-label font-medium text-foreground">
        <MagicWand aria-hidden className="size-3.5" />
        {tr("Choisir une école", "Pick a school")}
      </span>
    </New>
  );
}

function ATag() {
  return (
    <Dashboard
      card={
        <CharacterCard
          who={MAGE}
          classLine={
            <>
              <p className="text-body text-muted-foreground">{tr("Magicien", "Mage")}</p>
              <SchoolTag />
            </>
          }
        />
      }
    />
  );
}

function ATagDual() {
  return (
    <Dashboard
      who={DUAL}
      card={<CharacterCard who={DUAL} classLine={<SchoolTag />} />}
    />
  );
}

/** After the switch: the class reads the specialist, its school beside it, and the tag is gone. */
function ATagAfter() {
  return (
    <Dashboard
      card={
        <CharacterCard
          who={MAGE}
          classLine={
            <New>
              <p className="text-body text-muted-foreground">
                {tr("Transmutateur", "Transmuter")} · {lt(SCHOOLS.alteration)}
              </p>
            </New>
          }
        />
      }
    />
  );
}

// ---------------------------------------------------------------------------------------------
// Variant B: the class itself is a link, like the level beside it.

/** The class as a link (dotted underline and caret, like the level beside it) opening the picker. */
function ClassLink({ children }: { children: ReactNode }) {
  return (
    <New>
      <span className="inline-flex items-center gap-0.5 text-body text-foreground underline decoration-tint decoration-dotted underline-offset-4">
        {children}
        <CaretDown aria-hidden className="size-3" />
      </span>
    </New>
  );
}

/** The tooltip, portalled to the page as the app's are. */
function Tip({ top, children }: { top: string; children: ReactNode }) {
  return (
    <span
      style={{ top }}
      className="absolute left-8 z-10 w-max max-w-72 rounded-md bg-ink px-2 py-1 text-label text-surface shadow-float"
    >
      {children}
    </span>
  );
}

function BClassLink() {
  return (
    <Dashboard
      card={<CharacterCard who={MAGE} classLine={<ClassLink>{tr("Magicien", "Mage")}</ClassLink>} />}
      overlay={
        <Tip top="11.9rem">
          {tr("Généraliste, sans école de magie · choisir une école", "Generalist, no school of magic · pick a school")}
        </Tip>
      }
    />
  );
}

/** A fighter who took Mage: the mage entry's name in the entries line is the link. */
function BClassLinkDual() {
  return (
    <Dashboard
      who={DUAL}
      card={
        <CharacterCard
          who={DUAL}
          entriesLine={
            <span className="flex w-full min-w-0 flex-wrap items-baseline gap-x-1.5 rounded-md bg-raised px-2.5 py-1 text-body font-medium">
              {tr("Guerrier", "Fighter")} 6 /
              <ClassLink>{tr("Magicien", "Mage")}</ClassLink>1
            </span>
          }
        />
      }
      overlay={
        <Tip top="14rem">
          {tr("Généraliste, sans école de magie · choisir une école", "Generalist, no school of magic · pick a school")}
        </Tip>
      }
    />
  );
}

/** After the switch: the link reads the specialist and its school; it still opens the picker. */
function BClassLinkAfter() {
  return (
    <Dashboard
      card={
        <CharacterCard
          who={MAGE}
          classLine={
            <ClassLink>
              {tr("Transmutateur", "Transmuter")} · {lt(SCHOOLS.alteration)}
            </ClassLink>
          }
        />
      }
    />
  );
}

// ---------------------------------------------------------------------------------------------
// Variant C: in the Spells activity, where a mage thinks about schools.

function SpellsStage() {
  return (
    <section className="zone-card flex min-h-0 flex-1 flex-col gap-3 p-4">
      <div className="flex gap-6 text-body">
        <span className="font-medium">{tr("Grimoire", "Spellbook")}</span>
        <span className="text-muted-foreground">{tr("Préparés", "Prepared")}</span>
        <span className="text-muted-foreground">{tr("Catalogue", "Catalogue")}</span>
      </div>
      <New className="block">
        <div className="group-band flex items-center justify-between gap-3 rounded-lg px-3 py-2">
          <span className="flex min-w-0 flex-col">
            <span className="text-body font-medium">{tr("Magicien généraliste", "Generalist mage")}</span>
            <span className="text-label text-ink-muted">
              {tr(
                "Aucune école de magie. Un spécialiste a un sort de plus par niveau de sort dans son école.",
                "No school of magic. A specialist has one more spell per spell level, of his school.",
              )}
            </span>
          </span>
          <Button variant="quiet" className="shrink-0">
            <MagicWand aria-hidden />
            {tr("Choisir une école", "Pick a school")}
          </Button>
        </div>
      </New>
      <div className="flex min-h-0 flex-col gap-1 overflow-y-auto">
        {BOOK.map(([n, lvl, s]) => (
          <div key={n.en} className="grid grid-cols-[2rem_minmax(0,1fr)_auto] items-center gap-2 rounded-md bg-raised px-2.5 py-1 text-body">
            <span className="text-ink-muted tabular-nums">{lvl}</span>
            <span>{lt(n)}</span>
            <span className="text-label text-ink-muted">{lt(SCHOOLS[s])}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function CSpells() {
  return <Dashboard activity="spells" stage={<SpellsStage />} />;
}

// ---------------------------------------------------------------------------------------------
// Common to every variant: the details popover says what its button changes.

function Popover() {
  const rows: [string, ReactNode][] = [
    [tr("Nom", "Name"), MAGE.name],
    [
      tr("Classe", "Class"),
      <span className="flex flex-wrap items-center gap-2">
        {tr("Magicien", "Mage")}
        <New>
          <Button variant="quiet" size="xs">
            <MagicWand aria-hidden />
            {tr("École", "School")}
          </Button>
        </New>
      </span>,
    ],
    [tr("Niveau", "Level"), "6"],
    [tr("Race (règles)", "Race (rules)"), lt(RACES.elf)],
    [tr("Alignement", "Alignment"), tr("Neutre bon", "Neutral good")],
    [tr("Sexe", "Sex"), tr("Homme", "Male")],
    [tr("Langues", "Languages"), lt(RACES.elf)],
  ];
  return (
    <div className="absolute top-[8.5rem] left-[7.5rem] z-20 flex w-96 flex-col gap-3 rounded-xl bg-popover p-3 text-popover-foreground shadow-float">
      <span className="section-label">{tr("Détails du personnage", "Character details")}</span>
      <div className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-4 gap-y-1.5 text-body">
        {rows.map(([k, v]) => (
          <div key={k} className="contents">
            <span className="text-muted-foreground">{k}</span>
            <span className="min-w-0">{v}</span>
          </div>
        ))}
      </div>
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="ghost">{tr("Prendre une nouvelle classe", "Take a class")}</Button>
        <Button variant="ghost">{tr("Renseigner", "Fill in")}</Button>
        <Button variant="quiet">{tr("Modifier", "Edit")}</Button>
      </div>
    </div>
  );
}

function PopoverScreen() {
  return <Dashboard card={<CharacterCard who={MAGE} nameHint />} overlay={<Popover />} />;
}

// ---------------------------------------------------------------------------------------------
// Common to every variant: the picker names the schools and shows what each one costs.

function SchoolPicker({ picked }: { picked: string }) {
  const who = MAGE;
  const names = (s: typeof SPECIALISTS[number]["opposed"]) => s.map((c) => lt(SCHOOLS[c])).join(", ");
  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-ground/70">
      <div className="flex max-h-[calc(100%-4rem)] w-[min(56rem,calc(100%-4rem))] flex-col gap-3 rounded-xl bg-popover p-4 text-popover-foreground shadow-float">
        <span className="section-label">{tr("Choisir une école de magie", "Pick a school of magic")}</span>
        <p className="text-body text-ink-muted">
          {tr(
            "Magicien devient le spécialiste de l'école choisie. Niveau, PX, points de vie et grimoire restent tels quels ; les sorts des écoles opposées ne peuvent plus être préparés.",
            "Mage becomes the specialist of the school you pick. Level, XP, hit points and spellbook stay as they are; spells of the opposed schools can no longer be prepared.",
          )}
        </p>
        <div role="radiogroup" className="grid min-h-0 grid-cols-2 gap-2 overflow-y-auto">
          {SPECIALISTS.map((s) => {
            const ok = meets(who, s);
            const on = s.id === picked;
            const [ab, min] = s.minimum;
            return (
              <div
                key={s.id}
                role="radio"
                aria-checked={on}
                aria-disabled={!ok}
                className={cn(
                  "flex min-w-0 flex-col gap-0.5 rounded-lg border px-3 py-2 text-body",
                  on ? "selected border-transparent shadow-[inset_0_0_0_1.5px_var(--tint)]" : "border-input",
                  !ok && "border-dashed text-ink-muted",
                )}
              >
                <span className="flex items-baseline justify-between gap-2">
                  <span className="text-key">{lt(SCHOOLS[s.school])}</span>
                  <span className={cn("text-label", ok ? "text-ink-muted" : "")}>{lt(s.name)}</span>
                </span>
                <span className="text-label">
                  <span className="text-ink-muted">{tr("Opposées : ", "Opposed: ")}</span>
                  {names(s.opposed)}
                </span>
                <span className="flex flex-wrap items-center gap-x-3 text-label">
                  <span className={cn("inline-flex items-center gap-1 tabular-nums", !ok && "font-semibold text-danger")}>
                    {ok ? <Check aria-hidden className="size-3.5 text-success" /> : <Warning aria-hidden className="size-3.5" />}
                    {lt(ABILITY_NAMES[ab])} {min}
                    {!ok && tr(` requis (${who.abilities[ab]})`, ` needed (${who.abilities[ab]})`)}
                  </span>
                  {ok && !raceListed(who, s) && (
                    <span className="inline-flex items-center gap-1 text-warning">
                      <Warning aria-hidden className="size-3.5" />
                      {tr(`${lt(RACES[who.race])} hors table 22 : permis`, `${lt(RACES[who.race])} not in Table 22: allowed`)}
                    </span>
                  )}
                </span>
              </div>
            );
          })}
        </div>
        <div className="flex items-center justify-end gap-2">
          <Button variant="ghost">{tr("Annuler", "Cancel")}</Button>
          <Button variant="main">{tr("Suivant", "Next")}</Button>
        </div>
      </div>
    </div>
  );
}

function PickerScreen() {
  return <Dashboard overlay={<SchoolPicker picked="transmuter" />} />;
}

export const SCREENS: Record<string, () => ReactNode> = {
  "a-tag": ATag,
  "a-tag-dual": ATagDual,
  "a-tag-after": ATagAfter,
  "b-class-link": BClassLink,
  "b-class-link-dual": BClassLinkDual,
  "b-class-link-after": BClassLinkAfter,
  "c-spells": CSpells,
  popover: PopoverScreen,
  picker: PickerScreen,
};
