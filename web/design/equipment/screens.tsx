// The screens of the equipment design round (docs/design/equipment/README.md), one per id:
// `?screen=<id>`. Each is the whole dashboard with the part the round decides.
import type { ReactNode } from "react";
import {
  ArrowsLeftRight,
  CaretDown,
  Check,
  MagnifyingGlass,
  Plus,
  Minus,
  Sword,
  Sparkle,
  SlidersHorizontal,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import { lt, tr } from "./lang";
import { Dashboard } from "./frame";
import {
  AcBreakdown,
  AcCard,
  CategoryChips,
  GroupHead,
  PossessionsCard,
  ReviewCard,
  Tag,
} from "./pieces";
import {
  AC_ROWS,
  CATALOGUE_ARMOUR,
  POSSESSIONS,
  l,
  type Piece,
} from "./data";

/* ---------- small controls ---------- */

function Seg({ options, chosen }: { options: ReactNode[]; chosen: number }) {
  return (
    <div className="flex flex-wrap gap-1">
      {options.map((o, i) => (
        <span
          key={i}
          className={cn(
            "flex min-h-7 items-center justify-center gap-1 rounded-md px-2.5 text-body [&_svg]:size-4",
            i === chosen ? "chip-chosen" : "chip",
          )}
        >
          {o}
        </span>
      ))}
    </div>
  );
}

function Field({
  label,
  value,
  className,
  width = "w-full",
  hint,
}: {
  label: string;
  value: ReactNode;
  className?: string;
  width?: string;
  hint?: string;
}) {
  return (
    <label className={cn("flex min-w-0 flex-col gap-1", className)}>
      <span className={labelClass}>{label}</span>
      <span className={cn(fieldClass, "flex items-center gap-1", width)}>{value}</span>
      {hint && <span className="text-label text-ink-muted">{hint}</span>}
    </label>
  );
}

function Select({ value, className }: { value: string; className?: string }) {
  return (
    <span className={cn(fieldClass, "flex items-center justify-between gap-2", className)}>
      <span>{value}</span>
      <CaretDown className="size-3.5 shrink-0 text-ink-muted" aria-hidden />
    </span>
  );
}

function Stepper({ value }: { value: string }) {
  return (
    <span className="flex items-center gap-1">
      <Button variant="quiet" size="icon-xs">
        <Minus aria-hidden />
      </Button>
      <span className={cn(fieldClass, "flex w-12 items-center justify-center tabular-nums")}>{value}</span>
      <Button variant="quiet" size="icon-xs">
        <Plus aria-hidden />
      </Button>
    </span>
  );
}

function Popover({
  style,
  title,
  children,
  className,
}: {
  style: React.CSSProperties;
  title: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "absolute z-10 flex flex-col rounded-lg border border-border bg-popover shadow-2xl",
        className,
      )}
      style={style}
    >
      <div className="card-band flex min-h-9 items-center px-4 py-2">
        <h2 className="section-label">{title}</h2>
      </div>
      {children}
    </div>
  );
}

/* ---------- Inventaire ---------- */

const ACWARN_SHIELD = tr(
  "Petit bouclier : un barde ne porte pas de bouclier.",
  "Small shield: a bard carries no shield.",
);
const ACWARN_HANDS = tr(
  "Épée bâtarde +2 à deux mains avec un bouclier.",
  "Bastard sword +2 two-handed with a shield.",
);

function Inventory() {
  return (
    <Dashboard activity="inventory">
      <div className="flex min-h-0 flex-1 gap-3">
        <PossessionsCard />
        <AcCard />
      </div>
    </Dashboard>
  );
}

function Review() {
  return (
    <Dashboard activity="inventory">
      <div className="flex min-h-0 flex-1 gap-3">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
          <ReviewCard />
          <PossessionsCard review="none" />
        </div>
        <AcCard />
      </div>
    </Dashboard>
  );
}

function Warnings() {
  const shield: Piece = {
    id: "shield",
    name: l("Petit bouclier", "Small shield"),
    category: "shield",
    kind: l("Petit · CA −1", "Small · AC −1"),
    stats: l("", ""),
    state: "worn",
  };
  const pieces = POSSESSIONS.map((p) =>
    p.id === "bastard" ? { ...p, state: "wielded" as const } : p,
  ).map((p) => ({ ...p, review: false }));
  pieces.splice(1, 0, shield);
  const rows = [
    ...AC_ROWS.slice(0, 2),
    { term: l("Bouclier", "Shield"), source: l("Petit bouclier", "Small shield"), value: "−1" },
    ...AC_ROWS.slice(3),
  ];
  return (
    <Dashboard activity="inventory" card={{ acWarning: true }}>
      <div className="flex min-h-0 flex-1 gap-3">
        <PossessionsCard
          pieces={pieces}
          review="none"
          hoverId=""
          warnings={{
            shield: tr("Un barde ne porte pas de bouclier.", "A bard carries no shield."),
            bastard: tr("À deux mains, avec un bouclier porté.", "Two-handed, with a shield worn."),
          }}
        />
        <AcCard total="−3" rows={rows} warnings={[ACWARN_SHIELD, ACWARN_HANDS]} />
      </div>
    </Dashboard>
  );
}

function AcOverride() {
  return (
    <Dashboard activity="inventory" card={{ acOverride: true }}>
      <div className="flex min-h-0 flex-1 gap-3">
        <PossessionsCard review="none" hoverId="" />
        <AcCard override />
      </div>
    </Dashboard>
  );
}

/* ---------- Adding and editing ---------- */

function AddFrame({
  tab,
  children,
}: {
  tab: number;
  children: ReactNode;
}) {
  return (
    <Dashboard activity="inventory">
      <div className="flex min-h-0 flex-1 gap-3">
        <section className="stage-card flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="card-band flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2">
            <h2 className="section-label">{tr("Ajouter", "Add")}</h2>
            <Seg options={[tr("Catalogue", "Catalogue"), tr("Saisir", "Type in")]} chosen={tab} />
            <span className="flex-1" />
            <Button variant="quiet">
              <X aria-hidden />
              {tr("Fermer", "Close")}
            </Button>
          </div>
          {children}
        </section>
        <AcCard />
      </div>
    </Dashboard>
  );
}

function AddCatalogue() {
  const chosen = 8;
  return (
    <AddFrame tab={0}>
      <div className="flex min-h-0 flex-1 flex-col gap-3 p-4 min-[1800px]:flex-row">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-3">
          <div className="flex flex-wrap items-center gap-3">
            <Seg
              options={[tr("Armes", "Weapons"), tr("Munitions", "Ammunition"), tr("Armures", "Armour"), tr("Boucliers", "Shields")]}
              chosen={2}
            />
            <span className={cn(fieldClass, "flex w-56 items-center gap-2 text-ink-muted")}>
              <MagnifyingGlass className="size-4" aria-hidden />
              {tr("Chercher", "Search")}
            </span>
          </div>
          <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
            <div className="group-heading grid grid-cols-[1fr_3rem_8rem_8rem] gap-3 px-3 text-label font-medium">
              <span>{tr("Nom", "Name")}</span>
              <span className="text-right">{tr("CA", "AC")}</span>
              <span>{tr("Talents", "Skills")}</span>
              <span>{tr("Type d'armure", "Armour type")}</span>
            </div>
            <ul className="flex flex-col gap-1 pt-1">
              {CATALOGUE_ARMOUR.map((a, i) => (
                <li
                  key={a.name.en}
                  className={cn(
                    "grid shrink-0 grid-cols-[1fr_3rem_8rem_8rem] items-center gap-3 rounded-md px-3 py-1 text-body",
                    i === chosen ? "selected" : "bg-raised",
                  )}
                >
                  <span className="flex flex-wrap items-center gap-x-2">
                    {lt(a.name)}
                    {a.warn && <Tag tone="warning">{tr("Pas pour un barde", "Not for a bard")}</Tag>}
                  </span>
                  <span className="text-right tabular-nums">{a.ac}</span>
                  <span className={a.warn ? "text-ink-muted" : ""}>{lt(a.skill)}</span>
                  <span>{lt(a.row)}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>
        <div className="flex shrink-0 flex-col gap-3 rounded-lg bg-raised p-4 min-[1800px]:w-[26rem]">
          <span className="section-label">{tr("Votre exemplaire", "Your copy")}</span>
          <div className="flex flex-wrap gap-3">
            <Field label={tr("Nom", "Name")} value={tr("Cotte de mailles elfique", "Elven chain mail")} className="min-w-[14rem] flex-1" />
            <label className="flex flex-col gap-1">
              <span className={labelClass}>{tr("Magie", "Magic")}</span>
              <Stepper value="+1" />
            </label>
          </div>
          <p className="text-body text-ink-muted">
            {tr(
              "CA 5 · talents : mailles elfiques · type : mailles. Tout se corrige ensuite dans l'éditeur.",
              "AC 5 · skills: elven chain · type: chain. Everything can be changed later in the editor.",
            )}
          </p>
          <div className="flex flex-wrap items-center justify-end gap-3">
            <label className="flex items-center gap-2 text-body">
              <span className="flex size-4 items-center justify-center rounded-sm bg-tint text-on-accent">
                <Check weight="bold" className="size-3" aria-hidden />
              </span>
              {tr("Équiper tout de suite", "Equip it now")}
            </label>
            <Button variant="main" size="lg">
              <Plus aria-hidden />
              {tr("Ajouter", "Add")}
            </Button>
          </div>
        </div>
      </div>
    </AddFrame>
  );
}

function PowersBlock({ templates }: { templates?: boolean }) {
  return (
    <div className="flex flex-col gap-2">
      <GroupHead title={tr("Pouvoirs", "Powers")} note={tr("actifs quand l'objet est équipé", "on while equipped")} />
      <ul className="flex flex-col gap-1">
        {[
          [tr("CA −2", "AC −2"), tr("sans armure magique", "without magic armour")],
          [tr("Sauvegardes +2", "Saves +2"), ""],
        ].map(([e, c]) => (
          <li key={e} className="flex items-center gap-3 rounded-md bg-raised px-3 py-1.5 text-body">
            <Sparkle className="size-4 text-ink-muted" aria-hidden />
            <span className="font-medium">{e}</span>
            {c && <Tag tone="muted">{c}</Tag>}
            <span className="flex-1" />
            <Button variant="ghost" size="icon-sm" title={tr("Supprimer", "Delete")}>
              <X aria-hidden />
            </Button>
          </li>
        ))}
      </ul>
      <div className="flex flex-wrap gap-2">
        <Button variant="quiet">
          <Plus aria-hidden />
          {tr("Effet", "Effect")}
        </Button>
        {templates && (
          <Button variant="quiet" aria-expanded>
            {tr("Modèles", "Templates")}
            <CaretDown aria-hidden />
          </Button>
        )}
      </div>
    </div>
  );
}

function EditorFrame({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Dashboard activity="inventory">
      <div className="flex min-h-0 flex-1 gap-3">
        <section className="stage-card flex min-h-0 min-w-0 flex-1 flex-col">
          <div className="card-band flex min-h-9 items-center gap-3 px-4 py-2">
            <h2 className="section-label">{title}</h2>
          </div>
          <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-4">{children}</div>
          <div className="flex items-center justify-end gap-2 border-t border-border px-4 py-3">
            <Button variant="quiet">{tr("Annuler", "Cancel")}</Button>
            <Button variant="main">{tr("Enregistrer", "Save")}</Button>
          </div>
        </section>
        <AcCard />
      </div>
    </Dashboard>
  );
}

function EditArmour() {
  return (
    <EditorFrame title={tr("Modifier · Cotte de mailles", "Edit · Chain mail")}>
      <div className="grid gap-3 min-[1800px]:grid-cols-[2fr_1fr_1fr]">
        <Field label={tr("Nom", "Name")} value={tr("Cotte de mailles", "Chain mail")} />
        <Field label={tr("Catégorie", "Category")} value={tr("Armure", "Armour")} hint={tr("du catalogue : Cotte de mailles", "from the catalogue: Chain mail")} />
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("Magie (+N)", "Magic (+N)")}</span>
          <Stepper value="+0" />
        </label>
      </div>
      <div className="grid grid-cols-3 gap-3">
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("CA de base", "Base AC")}</span>
          <Stepper value="5" />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <span className={labelClass}>{tr("Colonne des talents", "Skills column")}</span>
          <Select value={tr("Métal", "Metal")} />
        </label>
        <label className="flex min-w-0 flex-col gap-1">
          <span className={labelClass}>{tr("Type d'armure", "Armour type")}</span>
          <Select value={tr("Mailles", "Chain")} />
        </label>
      </div>
      <Field label={tr("Note", "Note")} value={<span className="text-ink-muted">{tr("Facultative", "Optional")}</span>} />
      <PowersBlockEmpty />
    </EditorFrame>
  );
}

function PowersBlockEmpty() {
  return (
    <div className="flex flex-col gap-2">
      <GroupHead title={tr("Autres pouvoirs", "Other powers")} note={tr("la magie +N est déjà un pouvoir", "the +N magic is already a power")} />
      <p className="text-body text-ink-muted">{tr("Aucun.", "None.")}</p>
      <div>
        <Button variant="quiet">
          <Plus aria-hidden />
          {tr("Effet", "Effect")}
        </Button>
      </div>
    </div>
  );
}

function EditMagic() {
  return (
    <EditorFrame title={tr("Modifier · Anneau de protection +2", "Edit · Ring of protection +2")}>
      <div className="grid gap-3 min-[1800px]:grid-cols-[2fr_1fr_1fr]">
        <Field label={tr("Nom", "Name")} value={tr("Anneau de protection +2", "Ring of protection +2")} />
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("Catégorie", "Category")}</span>
          <Select value={tr("Objet magique", "Magic item")} />
        </label>
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("Type", "Kind")}</span>
          <Select value={tr("Anneau", "Ring")} />
        </label>
      </div>
      <div className="flex flex-wrap items-end gap-6">
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("Décompte", "Count")}</span>
          <Seg options={[tr("Aucun", "None"), tr("Stock", "Stock"), tr("Charges", "Charges"), tr("Quotidien", "Daily")]} chosen={0} />
        </label>
      </div>
      <div className="relative">
        <PowersBlock templates />
        <div className="absolute bottom-10 left-28 z-10 flex w-[22rem] flex-col gap-1 rounded-lg border border-border bg-popover p-1 shadow-2xl">
          {[
            [tr("Anneau de protection +N", "Ring of protection +N"), tr("CA −N sans armure magique, sauvegardes +N", "AC −N without magic armour, saves +N")],
            [tr("Cape de protection +N", "Cloak of protection +N"), tr("CA −N, sauvegardes +N", "AC −N, saves +N")],
            [tr("Bracelets de défense CA N", "Bracers of defense AC N"), tr("CA N comme une armure, sans armure", "AC N as if armoured, without armour")],
          ].map(([n, d]) => (
            <span key={n} className="flex flex-col rounded-md px-3 py-1.5 hover:bg-raised-hover">
              <span className="text-body font-medium">{n}</span>
              <span className="text-label text-ink-muted">{d}</span>
            </span>
          ))}
        </div>
      </div>
    </EditorFrame>
  );
}

function AddHomebrew() {
  return (
    <AddFrame tab={1}>
      <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-4">
        <Field label={tr("Nom", "Name")} value={tr("Bottes elfiques", "Elven boots")} />
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("Catégorie", "Category")}</span>
          <Seg
            options={[tr("Arme", "Weapon"), tr("Munition", "Ammunition"), tr("Armure", "Armour"), tr("Bouclier", "Shield"), tr("Objet magique", "Magic item"), tr("Équipement", "Gear")]}
            chosen={4}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("Type", "Kind")}</span>
          <Seg
            options={[tr("Potion", "Potion"), tr("Parchemin", "Scroll"), tr("Anneau", "Ring"), tr("Sceptre", "Rod"), tr("Bâton", "Staff"), tr("Baguette", "Wand"), tr("Divers", "Misc")]}
            chosen={6}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className={labelClass}>{tr("Décompte", "Count")}</span>
          <Seg options={[tr("Aucun", "None"), tr("Stock", "Stock"), tr("Charges", "Charges"), tr("Quotidien", "Daily")]} chosen={0} />
        </label>
        <Field label={tr("Note", "Note")} value={tr("Déplacement silencieux", "Move silently")} />
        <div className="flex flex-wrap items-center justify-end gap-3">
          <label className="flex items-center gap-2 text-body">
            <span className="flex size-4 items-center justify-center rounded-sm bg-tint text-on-accent">
              <Check weight="bold" className="size-3" aria-hidden />
            </span>
            {tr("Équiper tout de suite", "Equip it now")}
          </label>
          <Button variant="main" size="lg">
            <Plus aria-hidden />
            {tr("Ajouter", "Add")}
          </Button>
        </div>
      </div>
    </AddFrame>
  );
}

/* ---------- Act: weapons, AC breakdown, initiative ---------- */

function Journal({ cast }: { cast?: boolean }) {
  const rows = [
    ...(cast
      ? [
          ["4", tr("Initiative du tour 2 corrigée : 6 → 4 (Projectile magique +1)", "Turn 2 initiative corrected: 6 → 4 (Magic missile +1)")],
          ["", tr("Lancé : Projectile magique", "Cast: Magic missile")],
        ]
      : []),
    ["6", tr("Épée longue +2 contre CA 5 : attaque réussie, 6 dégâts", "Long sword +2 vs AC 5: hit, 6 damage")],
    ["6", tr("Initiative 6 pour le tour 2 : d10 5, Épée longue +2 +3, Hâte −2", "Initiative 6 for turn 2: d10 5, Long sword +2 +3, Haste −2")],
    ["", tr("Équipé : Anneau de protection +2", "Equipped: Ring of protection +2")],
    ["", tr("Équipé : Épée longue +2", "Equipped: Long sword +2")],
  ];
  return (
    <section className="zone-card flex min-h-0 flex-1 flex-col">
      <div className="card-band flex min-h-9 items-center gap-3 px-4 py-2">
        <h2 className="section-label">{tr("Journal", "Journal")}</h2>
      </div>
      <ul className="flex flex-col gap-0.5 overflow-hidden p-2">
        {rows.map(([n, t], i) => (
          <li key={i} className="grid grid-cols-[3rem_1fr_auto] gap-3 rounded-md px-2 py-0.5 text-body">
            <span className="text-right font-semibold text-highlight tabular-nums">{n}</span>
            <span>{t}</span>
            <span className="text-ink-muted tabular-nums">20:{39 - i}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ActTabs({ tab }: { tab: number }) {
  return (
    <div className="card-band flex items-center gap-1 px-3">
      {[tr("Armes", "Weapons"), tr("Sorts", "Spells"), tr("Objets", "Items"), tr("Compétences", "Skills")].map((t, i) => (
        <span
          key={t}
          className={cn(
            "px-3 py-2 text-body",
            i === tab ? "font-medium shadow-[inset_0_-2px_0_var(--tint)]" : "text-ink-muted",
          )}
        >
          {t}
        </span>
      ))}
    </div>
  );
}

function ResultCard() {
  return (
    <section className="zone-card latest flex min-h-0 min-w-0 flex-col">
      <div className="card-band flex min-h-9 items-center gap-2 px-4 py-2">
        <span className="chip px-1.5 text-label">{tr("Attaque", "Attack")}</span>
        <span className="text-body font-medium">{tr("Épée longue +2", "Long sword +2")}</span>
      </div>
      <div className="flex flex-col gap-3 p-4">
        <div className="flex items-center gap-3">
          <span className="flex size-11 items-center justify-center rounded-lg bg-tint text-key font-bold text-on-accent">15</span>
          <span className="flex flex-col">
            <span className="text-key">{tr("Touché", "Hit")}</span>
            <span className="text-label text-ink-muted">{tr("contre CA 5 · P/M", "vs AC 5 · S/M")}</span>
          </span>
        </div>
        <div className="grid grid-cols-3 gap-2">
          {[["d20", "15"], [tr("CA cible", "Target AC"), "5"], [tr("Marge", "Margin"), "+9"]].map(([k, v]) => (
            <span key={k} className="flex flex-col rounded-md bg-raised px-3 py-1.5">
              <span className="text-label text-ink-muted">{k}</span>
              <span className="text-body font-semibold tabular-nums">{v}</span>
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}

function WeaponsTab({ menu }: { menu?: boolean }) {
  return (
    <div className="flex flex-col gap-3 p-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg bg-raised px-3 py-1.5">
        <span className="section-label">{tr("Cible", "Target")}</span>
        <span className="text-body text-ink-muted">{tr("CA", "AC")}</span>
        <Stepper value="5" />
        <span className="text-body">{tr("Inconnue", "Unknown")}</span>
        <Seg options={[tr("P/M", "S/M"), "G"]} chosen={0} />
        <span className="flex-1" />
        <SlidersHorizontal className="size-4 text-ink-muted" aria-hidden />
      </div>
      <div className="flex items-center gap-4 rounded-lg border-2 border-tint bg-selected px-4 py-3">
        <div className="flex min-w-0 flex-1 flex-col">
          <span className="section-label">{tr("Arme en main", "Weapon in hand")}</span>
          <span className="text-title">{tr("Épée longue +2", "Long sword +2")}</span>
          <span className="text-body text-ink-muted">
            {tr("Mêlée · vitesse 3 · ", "Melee · speed 3 · ")}
            <b className="text-ink">1d8</b> {tr("P/M", "S/M")} · 1d12 G · <b className="text-ink">+2 / +2</b>
          </span>
        </div>
        <Button variant="main" className="h-auto flex-col items-start gap-0 px-4 py-1.5">
          <span className="flex items-center gap-2 text-key">
            <Sword aria-hidden />
            {tr("Attaquer", "Attack")}
          </span>
          <span className="text-label">{tr("6 contre CA 5", "needs 6 vs AC 5")}</span>
        </Button>
      </div>
      <div className="relative flex flex-wrap items-center gap-2">
        <Button variant="quiet">
          <ArrowsLeftRight aria-hidden />
          {tr("Arc court +2", "Short bow +2")}
        </Button>
        <Button variant="quiet" aria-expanded={menu}>
          {tr("Autre arme…", "Other weapon…")}
          <CaretDown aria-hidden />
        </Button>
        {menu && (
          <div className="absolute top-9 left-28 z-10 flex w-[21rem] flex-col gap-0.5 rounded-lg border border-border bg-popover p-1 shadow-2xl">
            <span className="px-2 pt-1 section-label">{tr("Équipées", "Equipped")}</span>
            {[[tr("Dague +3", "Dagger +3"), "1d4 · " + tr("vitesse 0", "speed 0")]].map(([n, s]) => (
              <span key={n} className="flex justify-between gap-3 rounded-md px-2 py-1 text-body hover:bg-raised-hover">
                {n}
                <span className="text-ink-muted">{s}</span>
              </span>
            ))}
            <span className="px-2 pt-2 section-label">{tr("Non équipées", "Not equipped")}</span>
            {[
              [tr("Épée bâtarde +2", "Bastard sword +2"), "2d4 · " + tr("vitesse 6", "speed 6")],
              [tr("Masse d'armes +1", "Mace +1"), "1d6"],
            ].map(([n, s], i) => (
              <span key={n} className={cn("flex justify-between gap-3 rounded-md px-2 py-1 text-body", i === 0 && "bg-raised-hover")}>
                {n}
                <span className="text-ink-muted">{s}</span>
              </span>
            ))}
            <span className="px-2 pt-1 pb-1 text-label text-ink-muted">
              {tr("En choisir une l'équipe.", "Picking one equips it.")}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}

function ActStage({ children, tab = 0, cast }: { children: ReactNode; tab?: number; cast?: boolean }) {
  return (
    <>
      <div className="grid min-h-0 shrink-0 grid-cols-[minmax(28rem,1.35fr)_minmax(18rem,1fr)] gap-3">
        <section className="stage-card flex min-w-0 flex-col">
          <ActTabs tab={tab} />
          {children}
        </section>
        <ResultCard />
      </div>
      <Journal cast={cast} />
    </>
  );
}

function ActWeapons() {
  return (
    <Dashboard activity="act">
      <ActStage>
        <WeaponsTab />
      </ActStage>
    </Dashboard>
  );
}

function ActWeaponsMenu() {
  return (
    <Dashboard activity="act">
      <ActStage>
        <WeaponsTab menu />
      </ActStage>
    </Dashboard>
  );
}

function AcPopover() {
  return (
    <Dashboard
      activity="act"
      card={{ acOpen: true }}
      overlay={
        <Popover
          title={tr("Classe d'armure −2", "Armour class −2")}
          style={{ left: "calc(clamp(280px, 21vw, 384px) + 6px)", top: 214 }}
          className="w-[23rem]"
        >
          <div className="flex flex-col gap-3 p-4">
            <AcBreakdown rows={AC_ROWS} total="−2" />
            <div className="flex justify-between gap-2">
              <Button variant="quiet" size="sm">{tr("Inventaire", "Inventory")}</Button>
              <Button variant="quiet" size="sm">{tr("Forcer", "Override")}</Button>
            </div>
          </div>
        </Popover>
      }
    >
      <ActStage>
        <WeaponsTab />
      </ActStage>
    </Dashboard>
  );
}

function Initiative() {
  return (
    <Dashboard activity="act" initTip="weapon">
      <ActStage>
        <WeaponsTab />
      </ActStage>
    </Dashboard>
  );
}

function InitiativeCast() {
  return (
    <Dashboard activity="act" initTip="cast">
      <ActStage tab={1} cast>
        <div className="flex flex-col gap-2 p-4">
          <GroupHead title={tr("Niveau 1", "Level 1")} />
          {[tr("Projectile magique", "Magic missile"), tr("Mains brûlantes", "Burning hands")].map((n, i) => (
            <div key={n} className="flex items-center justify-between rounded-md bg-raised px-3 py-1.5 text-body">
              {n}
              <Button variant={i === 0 ? "main" : "quiet"} size="sm">
                {tr("Lancer", "Cast")}
              </Button>
            </div>
          ))}
        </div>
      </ActStage>
    </Dashboard>
  );
}

function Skills() {
  const skills = [
    [tr("Crochetage", "Pick pockets"), "45 %"],
    [tr("Détection des bruits", "Detect noise"), "60 %"],
    [tr("Escalade", "Climb walls"), "80 %"],
    [tr("Lecture des langues", "Read languages"), "55 %"],
  ];
  return (
    <Dashboard activity="act">
      <ActStage tab={3}>
        <div className="flex flex-col gap-2 p-4">
          <GroupHead title={tr("Talents de barde", "Bard skills")} />
          <p className="flex flex-wrap items-center gap-2 px-3 text-body">
            <span className="text-ink-muted">{tr("Armure :", "Armour:")}</span>
            <span className="font-medium">{tr("Cotte de mailles (équipée)", "Chain mail (equipped)")}</span>
            <Tag tone="muted">{tr("métal : −5 % à chaque talent", "metal: −5% to every skill")}</Tag>
          </p>
          <ul className="grid grid-cols-2 gap-1">
            {skills.map(([n, v]) => (
              <li key={n} className="flex justify-between rounded-md bg-raised px-3 py-1 text-body">
                {n}
                <b className="tabular-nums">{v}</b>
              </li>
            ))}
          </ul>
        </div>
      </ActStage>
    </Dashboard>
  );
}

export const SCREENS: Record<string, () => ReactNode> = {
  inventory: Inventory,
  review: Review,
  "add-catalogue": AddCatalogue,
  "add-homebrew": AddHomebrew,
  "edit-armour": EditArmour,
  "edit-magic": EditMagic,
  warnings: Warnings,
  "ac-popover": AcPopover,
  "ac-override": AcOverride,
  "act-weapons": ActWeapons,
  "act-weapons-menu": ActWeaponsMenu,
  initiative: Initiative,
  "initiative-cast": InitiativeCast,
  skills: Skills,
};

