// The Skills round's screens (docs/design/skills/README.md). Every layout here is a proposal until the
// user approves it. `?screen=<id>&lang=fr|en`.
import { Fragment, type ReactNode } from "react";
import {
  CaretDown,
  Check,
  MagnifyingGlass,
  PencilSimple,
  Plus,
  Prohibit,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  abil,
  BARD,
  FIGHTER,
  MULTI,
  THIEF,
  THIEF_NEW,
  hasPending,
  type Ability,
  CATALOGUE,
  FAMILY,
  thiefMay,
  WEAPONS,
  classGroups,
  costFor,
  GROUPS,
  groupName,
  percent,
  prof,
  skillName,
  spentNonWeapon,
  spentPoints,
  spentWeapon,
  weapon,
  weaponName,
  type Fixture,
  type Group,
  type NonWeapon,
  type RogueSkill,
  type SkillSet,
  type Weapon,
} from "./data";
import { Abilities, Dashboard, l2 } from "./frame";
import { LANG, tr } from "./lang";

const signed = (n: number) => (n > 0 ? `+${n}` : n < 0 ? `−${-n}` : "0");

/** "SAG 9 −2 → 7": the score in effect, the modifier, the target the check rolls under. */
function Target({ who, p }: { who: Fixture; p: NonWeapon }) {
  if (!p.ability)
    return <span className="text-label text-ink-muted">{tr("sans jet", "no check")}</span>;
  const score = who.abilities[p.ability];
  return (
    <span className="whitespace-nowrap tabular-nums">
      <span className="text-ink-muted">
        {abil(p.ability)} {score}
        {p.modifier !== 0 && ` ${signed(p.modifier)}`} →{" "}
      </span>
      <b className="font-semibold">{score + p.modifier}</b>
    </span>
  );
}

function Card({
  title,
  aside,
  action,
  children,
  className,
}: {
  title: string;
  aside?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("flex min-w-0 flex-col gap-2 rounded-lg bg-raised p-3", className)}>
      <div className="flex items-center gap-x-3">
        <h3 className="section-label">{title}</h3>
        {aside}
        <span className="flex-1" />
        {action}
      </div>
      {children}
    </section>
  );
}

/** A section's Edit. With something to spend it takes a red edge and a red dot instead of the
 * pencil (round 2, the user's choice over a dot by the title: the mark sits on what to click). */
function Edit({ pending = false }: { pending?: boolean }) {
  return (
    <Button size="sm" data-signal={pending ? "edit" : undefined} className={cn(pending && "border-danger")}>
      {pending ? (
        <span aria-hidden className="size-2 rounded-full bg-danger" />
      ) : (
        <PencilSimple aria-hidden />
      )}
      {tr("Modifier", "Edit")}
    </Button>
  );
}

/** The slot line: "Emplacements 7 / 8 · 3 au départ + …". */
function Budget({
  spent,
  available,
  formula,
  unit,
}: {
  spent: number;
  available: number;
  formula: [string, string];
  unit: [string, string];
}) {
  return (
    <p className="text-body break-words text-ink-muted">
      <b className="text-ink tabular-nums">
        {spent} / {available}
      </b>{" "}
      {l2(unit)}
      {l2(formula) && ` · ${l2(formula)}`}
    </p>
  );
}

const SLOTS: [string, string] = ["emplacements", "slots"];
const SLOT: [string, string] = ["empl.", "slot"];
const POINTS: [string, string] = ["points", "points"];

// ---------- read views ----------

function WeaponsRead({ who, action, aside }: { who: Fixture; action?: ReactNode; aside?: ReactNode }) {
  return (
    <Card
      title={tr("Maîtrises d'armes", "Weapon proficiencies")}
      aside={aside}
      action={action}
    >
      <Budget
        spent={spentWeapon(who)}
        available={who.weaponSlots.available}
        formula={who.weaponSlots.formula}
        unit={SLOTS}
      />
      <ul className="flex flex-col gap-1">
        {who.weaponPicks.map((p) => (
          <li
            key={p.code}
            className="flex items-center justify-between gap-2 rounded-md bg-sub-band px-2.5 py-1 text-body"
          >
            <span className="break-words">{weaponName(weapon(p.code))}</span>
            <span className="shrink-0 text-ink-muted">
              {p.spec
                ? tr(`spécialisation · ${p.slots} empl.`, `specialized · ${p.slots} slots`)
                : `${p.slots} ${l2(SLOT)}`}
            </span>
          </li>
        ))}
      </ul>
      <p className="text-label text-ink-muted">
        {tr(
          `Sans maîtrise : ${signed(who.weaponSlots.penalty)} au toucher.`,
          `Not proficient: ${signed(who.weaponSlots.penalty)} to hit.`,
        )}
      </p>
    </Card>
  );
}

function ProficienciesRead({
  who,
  action,
  aside,
  className,
}: {
  who: Fixture;
  action?: ReactNode;
  aside?: ReactNode;
  className?: string;
}) {
  return (
    <Card
      title={tr("Compétences et langues", "Proficiencies and languages")}
      aside={aside}
      action={action}
      className={className}
    >
      <Budget
        spent={spentNonWeapon(who)}
        available={who.nonWeaponSlots.available}
        formula={who.nonWeaponSlots.formula}
        unit={SLOTS}
      />
      <ul className="flex flex-col gap-1">
        {who.picks.map((pk) => {
          const p = prof(pk.id);
          return (
            <li
              key={pk.id}
              className="grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-md bg-sub-band px-2.5 py-1 text-body"
            >
              <span className="break-words">{skillName(pk.id)}</span>
              <Target who={who} p={p} />
              <span className="w-[4.5rem] text-right text-label text-ink-muted">
                {pk.free ? tr("gratuite", "free") : `${pk.spent} ${l2(SLOT)}`}
              </span>
            </li>
          );
        })}
      </ul>
      <div className="flex flex-col gap-1 pt-1">
        <span className="text-label text-ink-muted">{tr("Langues", "Languages")}</span>
        <div className="flex flex-wrap gap-1.5">
          {who.languages.map((lg) => (
            <span key={lg.code[1]} className="chip px-2 py-0.5 text-body">
              {l2(lg.code)}
              <span className="ml-1.5 text-label text-ink-muted">
                {lg.native ? tr("de naissance", "native") : `1 ${l2(SLOT)}`}
              </span>
            </span>
          ))}
        </div>
      </div>
    </Card>
  );
}

/** The breakdown table: one column per term, the total last (decided in triage). With `edit`,
 * the Points column holds the control. */
function SkillTable({
  set,
  edit,
  className,
}: {
  set: SkillSet;
  edit?: "slider";
  className?: string;
}) {
  const th = "px-2 py-1 text-right font-medium text-label text-ink-muted";
  const td = "px-2 py-1.5 text-right tabular-nums";
  return (
    <table className={cn("w-full border-separate border-spacing-y-1 text-body", className)}>
      <thead>
        <tr>
          <th className={cn(th, "text-left")}>{tr("Talent", "Skill")}</th>
          <th className={th}>{tr("Base", "Base")}</th>
          <th className={cn(th, edit && "text-center")}>{tr("Points", "Points")}</th>
          <th className={th}>{tr("Race", "Race")}</th>
          <th className={th}>{tr("Dextérité", "Dexterity")}</th>
          <th className={th}>{tr("Armure", "Armour")}</th>
          <th className={th}>{tr("Total", "Total")}</th>
        </tr>
      </thead>
      <tbody>
        {set.skills.map((s) => (
          <tr key={s.id} className="bg-sub-band">
            <td className="rounded-l-md px-2.5 py-1.5 text-left">
              <span className="underline decoration-dotted underline-offset-2">
                {skillName(s.id)}
              </span>
            </td>
            <td className={td}>{s.base}</td>
            <td className={cn(td, edit && "w-[1%] text-center")}>
              {edit === "slider" ? (
                <Slider skill={s} set={set} />
              ) : (
                s.points
              )}
            </td>
            <td className={td}>{signed(s.race)}</td>
            <td className={td}>{signed(s.dex)}</td>
            <td className={td}>{signed(s.armour)}</td>
            <td className={cn(td, "rounded-r-md text-key font-semibold whitespace-nowrap")}>
              {percent(s)} %
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function SkillsRead({ set, action, aside }: { set: SkillSet; action?: ReactNode; aside?: ReactNode }) {
  return (
    <Card
      title={l2(set.name)}
      aside={aside}
      action={action}
    >
      <Budget
        spent={spentPoints(set)}
        available={set.available}
        formula={set.formula}
        unit={POINTS}
      />
      <SkillTable set={set} />
      <p className="text-label text-ink-muted">
        {tr(`Armure portée : ${set.armour[0]}`, `Armour worn: ${set.armour[1]}`)}
        {" · "}
        {set.perSkillCap > 0
          ? tr(
              `au plus ${set.perSkillCap} points par talent (${l2(set.capFormula!)}), 95 % au plus`,
              `at most ${set.perSkillCap} points per skill (${l2(set.capFormula!)}), 95% at most`,
            )
          : tr("95 % au plus par talent", "95% at most per skill")}
      </p>
    </Card>
  );
}

// ---------- controls ----------

function Slider({ skill, set }: { skill: RogueSkill; set: SkillSet }) {
  const left = set.available - spentPoints(set);
  const to95 = 95 - (percent(skill) - skill.points);
  const limit = Math.min(set.perSkillCap > 0 ? set.perSkillCap : to95, to95);
  const reach = Math.min(limit, skill.points + left);
  const scale = Math.max(limit, 1);
  return (
    <span className="inline-flex items-center gap-2">
      <span className="relative block h-7 w-56">
        <span className="absolute inset-x-0 top-3 h-1.5 rounded-full bg-input" />
        <span
          className="absolute top-3 h-1.5 rounded-full bg-tint/35"
          style={{ left: 0, width: `${(reach / scale) * 100}%` }}
        />
        <span
          className="absolute top-3 h-1.5 rounded-full bg-tint"
          style={{ left: 0, width: `${(skill.points / scale) * 100}%` }}
        />
        <span
          className="absolute top-1.5 size-4 -translate-x-1/2 rounded-full border-2 border-tint bg-surface"
          style={{ left: `${(skill.points / scale) * 100}%` }}
        />
      </span>
      <span className="flex h-7 w-11 items-center justify-center rounded-md border border-input tabular-nums">
        {skill.points}
      </span>
      <span className="w-12 text-left text-label text-ink-muted">
        / {limit}
      </span>
    </span>
  );
}

// ---------- editors ----------

/** The editor's top band, stuck while the list scrolls: what is left, Cancel and Save. */
function EditorBar({
  title,
  spent,
  available,
  formula,
  unit,
  note,
  actions = true,
}: {
  actions?: boolean;
  title: string;
  spent: number;
  available: number;
  formula: [string, string];
  unit: [string, string];
  note?: ReactNode;
}) {
  const left = available - spent;
  return (
    <div className="group-band flex flex-wrap items-center gap-x-5 gap-y-1 rounded-lg px-3 py-2">
      <h3 className="text-title">{title}</h3>
      <span className="flex items-baseline gap-1.5">
        <b
          className={cn(
            "text-headline tabular-nums",
            left > 0 ? "text-danger" : left < 0 ? "text-warning" : "text-ink",
          )}
        >
          {left}
        </b>
        <span className="text-body text-ink-muted">
          {tr(`${unit[0]} restants sur ${available}`, `${unit[1]} left of ${available}`)}
        </span>
      </span>
      <span className="min-w-0 flex-1 text-label break-words text-ink-muted">
        {l2(formula)} · {tr(`dépensés ${spent}`, `spent ${spent}`)}
        {note}
      </span>
      {actions && <span className="flex gap-2">
        <Button size="sm">{tr("Annuler", "Cancel")}</Button>
        <Button size="sm" variant="main">
          {tr("Enregistrer", "Save")}
        </Button>
      </span>}
    </div>
  );
}

function PickRow({ who, pick }: { who: Fixture; pick: { id: string; spent: number; free?: boolean } }) {
  const p = prof(pick.id);
  return (
    <li className="grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-md bg-sub-band px-2.5 py-1 text-body">
      <span className="break-words">{skillName(pick.id)}</span>
      <Target who={who} p={p} />
      {pick.free ? (
        <span className="flex w-[7.25rem] items-center justify-end gap-2 text-label text-ink-muted">
          {tr("gratuite (barde)", "free (bard)")}
        </span>
      ) : (
        <span className="flex w-[7.25rem] items-center justify-end gap-1">
          <span className="text-label text-ink-muted">
            {pick.spent} {l2(SLOT)}
          </span>
          <Button size="icon-xs" variant="quiet" aria-label={tr("Retirer", "Remove")}>
            <X aria-hidden />
          </Button>
        </span>
      )}
    </li>
  );
}

/** The catalogue by the book's groups: the class's groups first (normal cost), then the others
 * (one slot more). A proficiency in several groups is listed under each; held ones are ticked. */
function Catalogue({ who, open }: { who: Fixture; open: Group[] }) {
  const own = new Set(who.classIds.flatMap(classGroups));
  const held = new Set(who.picks.map((p) => p.id));
  const order = [...GROUPS.filter((g) => own.has(g)), ...GROUPS.filter((g) => !own.has(g))];
  return (
    <div className="flex min-h-0 flex-col gap-2">
      <div className="flex items-center gap-2 rounded-md border border-input px-2.5 py-1.5 text-body text-ink-muted">
        <MagnifyingGlass aria-hidden className="size-4" />
        {tr("Chercher une compétence", "Search a proficiency")}
      </div>
      <div className="flex min-h-0 flex-col gap-2 overflow-y-auto pr-1">
        {order.map((g) => {
          const mine = own.has(g);
          const rows = CATALOGUE.filter((p) => p.groups.includes(g));
          const isOpen = open.includes(g);
          return (
            <section key={g} className="flex flex-col gap-1">
              <div
                className={cn(
                  "flex flex-wrap items-center gap-x-2 rounded-md px-2.5 py-1.5",
                  mine ? "bg-selected" : "bg-sub-band",
                )}
              >
                <CaretDown
                  aria-hidden
                  className={cn("size-3.5", !isOpen && "-rotate-90")}
                />
                <span className="text-body font-semibold">{groupName(g)}</span>
                <span className="text-label text-ink-muted">
                  {mine
                    ? tr("groupe de la classe · coût normal", "class group · normal cost")
                    : tr("hors classe · +1 emplacement", "other class · +1 slot")}
                </span>
                <span className="flex-1" />
                <span className="text-label text-ink-muted">{rows.length}</span>
              </div>
              {isOpen &&
                rows.map((p) => {
                  const has = held.has(p.id);
                  return (
                    <div
                      key={p.id}
                      className={cn(
                        "grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-md px-2.5 py-0.5 text-body",
                        has && "text-ink-muted",
                      )}
                    >
                      <span className="break-words">{skillName(p.id)}</span>
                      <Target who={who} p={p} />
                      <span className="flex w-[6.5rem] items-center justify-end gap-1.5">
                        <span className="text-label text-ink-muted">
                          {costFor(who, p)} {l2(SLOT)}
                        </span>
                        {has ? (
                          <span className="flex size-6 items-center justify-center"><Check aria-label={tr("Déjà prise", "Held")} className="size-4 text-success" /></span>
                        ) : (
                          <Button size="icon-xs" variant="quiet" aria-label={tr("Ajouter", "Add")}>
                            <Plus aria-hidden />
                          </Button>
                        )}
                      </span>
                    </div>
                  );
                })}
            </section>
          );
        })}
      </div>
    </div>
  );
}

function ProficiencyEditor({ who, open, actions }: { who: Fixture; open: Group[]; actions?: boolean }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <EditorBar
        actions={actions}
        title={tr("Compétences et langues", "Proficiencies and languages")}
        spent={spentNonWeapon(who)}
        available={who.nonWeaponSlots.available}
        formula={who.nonWeaponSlots.formula}
        unit={SLOTS}
      />
      <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] gap-4">
        <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
          <section className="flex flex-col gap-1">
            <span className="section-label">{tr("Vos compétences", "Your proficiencies")}</span>
            <ul className="flex flex-col gap-1">
              {who.picks.map((p) => (
                <PickRow key={p.id} who={who} pick={p} />
              ))}
            </ul>
          </section>
          <section className="flex flex-col gap-1.5">
            <span className="section-label">{tr("Langues", "Languages")}</span>
            <p className="text-label text-ink-muted">
              {tr(
                "Chaque langue au-delà de celles de naissance prend 1 emplacement de compétence.",
                "Each language beyond the native ones takes 1 proficiency slot.",
              )}
            </p>
            <div className="flex flex-wrap gap-1.5">
              {who.languages.map((lg) => (
                <span key={lg.code[1]} className="chip flex items-center gap-1.5 px-2 py-0.5 text-body">
                  {l2(lg.code)}
                  <span className="text-label text-ink-muted">
                    {lg.native ? tr("de naissance", "native") : `1 ${l2(SLOT)}`}
                  </span>
                  {lg.paid && <X aria-hidden className="size-3.5 text-ink-muted" />}
                </span>
              ))}
              <Button size="xs">
                <Plus aria-hidden />
                {tr("Langue", "Language")}
              </Button>
            </div>
          </section>
        </div>
        <Catalogue who={who} open={open} />
      </div>
    </div>
  );
}

/** Which weapons a fixture's classes may take: a thief's list alone, any weapon with a warrior class. */
const mayUse = (who: Fixture) => (w: Weapon) =>
  who.classIds.some((c) => c === "fighter") || who.classIds.every((c) => c !== "thief") || thiefMay(w);
/** Only a single-class fighter may specialize (PHB). */
const canSpecialize = (who: Fixture) => who.classIds.length === 1 && who.classIds[0] === "fighter";

/** The weapons editor (decision 5): held weapons on the left; on the right a search and the weapons
 * by family. Within a family the class's weapons come first, then the ones it may not take, greyed
 * with the reason (the user's choice, round 2, over one "not allowed" group at the end). A family
 * with none of the class's weapons is a quiet band. */
function WeaponEditor({ who, open, actions }: { who: Fixture; open: Weapon["family"][]; actions?: boolean }) {
  const left = who.weaponSlots.available - spentWeapon(who);
  const ok = mayUse(who);
  const held = new Set(who.weaponPicks.map((p) => p.code));
  const families = Object.keys(FAMILY) as Weapon["family"][];
  const cls = l2(who.classes[0].name).toLowerCase();
  const row = (w: Weapon, allowed: boolean) => (
    <div
      key={w.code}
      className={cn(
        "grid grid-cols-[1fr_auto] items-center gap-3 rounded-md px-2.5 py-0.5 text-body",
        (!allowed || held.has(w.code)) && "text-ink-muted",
      )}
    >
      <span className="break-words">{weaponName(w)}</span>
      <span className="flex items-center justify-end gap-1.5">
        {!allowed ? (
          <span className="flex items-center gap-1.5 text-label">
            <Prohibit aria-hidden className="size-3.5" />
            {tr(`hors de la liste du ${cls}`, `not on the ${cls}'s list`)}
          </span>
        ) : (
          <>
            <span className="text-label text-ink-muted">1 {l2(SLOT)}</span>
            {held.has(w.code) ? (
              <span className="flex size-6 items-center justify-center">
                <Check aria-label={tr("Déjà prise", "Held")} className="size-4 text-success" />
              </span>
            ) : (
              <Button size="icon-xs" variant="quiet" disabled={left <= 0} aria-label={tr("Ajouter", "Add")}>
                <Plus aria-hidden />
              </Button>
            )}
          </>
        )}
      </span>
    </div>
  );
  const band = (key: string, title: string, note: string, count: number, isOpen: boolean, mine: boolean) => (
    <div
      key={key}
      className={cn(
        "flex flex-wrap items-center gap-x-2 rounded-md px-2.5 py-1.5",
        mine ? "bg-selected" : "bg-sub-band",
      )}
    >
      <CaretDown aria-hidden className={cn("size-3.5", !isOpen && "-rotate-90")} />
      <span className="text-body font-semibold">{title}</span>
      {note && <span className="text-label text-ink-muted">{note}</span>}
      <span className="flex-1" />
      <span className="text-label text-ink-muted">{count}</span>
    </div>
  );
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <EditorBar
        actions={actions}
        title={tr("Maîtrises d'armes", "Weapon proficiencies")}
        spent={spentWeapon(who)}
        available={who.weaponSlots.available}
        formula={who.weaponSlots.formula}
        unit={SLOTS}
      />
      <div className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] gap-4">
        <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
          <section className="flex flex-col gap-1">
            <span className="section-label">{tr("Vos maîtrises", "Your proficiencies")}</span>
            <ul className="flex flex-col gap-1">
              {who.weaponPicks.map((p) => (
                <li
                  key={p.code}
                  className="grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-md bg-sub-band px-2.5 py-1 text-body"
                >
                  <span className="break-words">{weaponName(weapon(p.code))}</span>
                  {canSpecialize(who) ? (
                    <Button size="xs" aria-pressed={!!p.spec}>
                      {p.spec
                        ? tr("Spécialisée", "Specialized")
                        : tr("Se spécialiser +1 empl.", "Specialize +1 slot")}
                    </Button>
                  ) : (
                    <span />
                  )}
                  <span className="flex w-[5.5rem] items-center justify-end gap-1">
                    <span className="text-label text-ink-muted">
                      {p.slots} {l2(SLOT)}
                    </span>
                    <Button size="icon-xs" variant="quiet" aria-label={tr("Retirer", "Remove")}>
                      <X aria-hidden />
                    </Button>
                  </span>
                </li>
              ))}
            </ul>
          </section>
          <p className="text-label text-ink-muted">
            {tr(
              `Sans maîtrise : ${signed(who.weaponSlots.penalty)} au toucher.`,
              `Not proficient: ${signed(who.weaponSlots.penalty)} to hit.`,
            )}{" "}
            {canSpecialize(who)
              ? tr(
                  "Spécialisation : +1 au toucher, +2 aux dégâts, une attaque de plus tous les 2 rounds.",
                  "Specialization: +1 to hit, +2 damage, one more attack every 2 rounds.",
                )
              : who.classIds.length > 1
                ? tr(
                    "Multi-classé : pas de spécialisation (guerrier mono-classé seulement).",
                    "Multi-class: no specialization (single-class fighters only).",
                  )
                : null}
          </p>
        </div>
        <div className="flex min-h-0 flex-col gap-2">
          <div className="flex items-center gap-2 rounded-md border border-input px-2.5 py-1.5 text-body text-ink-muted">
            <MagnifyingGlass aria-hidden className="size-4" />
            {tr("Chercher une arme", "Search a weapon")}
          </div>
          <div className="flex min-h-0 flex-col gap-2 overflow-y-auto pr-1">
            {families.map((f) => {
              const mine = WEAPONS.filter((w) => w.family === f && ok(w));
              const not = WEAPONS.filter((w) => w.family === f && !ok(w));
              const isOpen = open.includes(f);
              const note = !not.length
                ? ""
                : !mine.length
                  ? tr(`aucune sur la liste du ${cls}`, `none on the ${cls}'s list`)
                  : tr(`${not.length} hors de la liste du ${cls}`, `${not.length} not on the ${cls}'s list`);
              return (
                <section key={f} className="flex flex-col gap-1">
                  {band(f, l2(FAMILY[f]), note, mine.length + not.length, isOpen, mine.length > 0)}
                  {isOpen && [...mine.map((w) => row(w, true)), ...not.map((w) => row(w, false))]}
                </section>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

function PointsEditor({ set, control, actions }: { set: SkillSet; control: "slider"; actions?: boolean }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <EditorBar
        actions={actions}
        title={l2(set.name)}
        spent={spentPoints(set)}
        available={set.available}
        formula={set.formula}
        unit={POINTS}
      />
      <SkillTable set={set} edit={control} />
      <p className="text-label text-ink-muted">
        {tr(
              "La partie pleine : les points du talent. La partie claire : jusqu'où les points restants le mènent. « / 30 » : sa limite (95 % au plus, ou le plafond par talent d'un voleur).",
              "The solid part: the skill's points. The light part: how far the points left take it. “/ 30”: its limit (95% at most, or a thief's cap per skill).",
        )}
      </p>
    </div>
  );
}

// ---------- screens ----------

function Stage({ children }: { children: ReactNode }) {
  return (
    <div className="zone-card flex min-h-0 min-w-0 flex-1 flex-col gap-3 overflow-y-auto p-3">
      {children}
    </div>
  );
}

/** Layout A (the user's choice, round 2): everything read at a glance; one Edit per section turns
 * the stage into its editor. Proficiencies on the left; weapons, then each skill set, on the right. */
function OverviewA({ who }: { who: Fixture }) {
  const pending = (left: number) => <Edit pending={left > 0} />;
  return (
    <Dashboard activity="skills" who={who}>
      <Stage>
        <div className="grid grid-cols-2 items-start gap-3">
          <ProficienciesRead
            who={who}
            action={pending(who.nonWeaponSlots.available - spentNonWeapon(who))}
            className="row-span-2"
          />
          <WeaponsRead who={who} action={pending(who.weaponSlots.available - spentWeapon(who))} />
          {who.sets.map((s) => (
            <SkillsRead key={s.id} set={s} action={pending(s.available - spentPoints(s))} />
          ))}
        </div>
      </Stage>
    </Dashboard>
  );
}

/** Multi-class (decision 8, the user's choice in round 2 over the formula spelled out by class and a
 * heading per class): a short class tag by each title; the budget line keeps the total and the
 * detailed formula moves to the editor's top band. */
const MULTI_TAGGED: Fixture = {
  ...MULTI,
  weaponSlots: { ...MULTI.weaponSlots, formula: ["", ""] },
  nonWeaponSlots: { ...MULTI.nonWeaponSlots, formula: ["", ""] },
  sets: MULTI.sets.map((s) => ({ ...s, formula: ["", ""] as [string, string] })),
};

function ClassTag({ children }: { children: ReactNode }) {
  return <span className="chip px-2 py-0.5 text-label text-ink-muted">{children}</span>;
}

function MultiTagged() {
  const who = MULTI_TAGGED;
  const tag = (fr: string, en: string) => <ClassTag>{tr(fr, en)}</ClassTag>;
  return (
    <Dashboard activity="skills" who={who}>
      <Stage>
        <div className="grid grid-cols-2 items-start gap-3">
          <ProficienciesRead
            who={who}
            aside={tag("Guerrier 4 + Voleur 5", "Fighter 4 + Thief 5")}
            action={<Edit pending />}
            className="row-span-2"
          />
          <WeaponsRead who={who} aside={tag("Guerrier 4", "Fighter 4")} action={<Edit pending />} />
          {who.sets.map((s) => (
            <SkillsRead key={s.id} set={s} aside={tag("Voleur 5", "Thief 5")} action={<Edit pending />} />
          ))}
        </div>
      </Stage>
    </Dashboard>
  );
}

function EditorScreen({ who, children, lit }: { who: Fixture; children: ReactNode; lit?: Parameters<typeof Dashboard>[0]["lit"] }) {
  return (
    <Dashboard activity="skills" who={who} lit={lit}>
      <Stage>{children}</Stage>
    </Dashboard>
  );
}

// ---------- creation (decision 6) ----------

type Part = "weapons" | "proficiencies" | "points";

/** Creation (decision 6, the user's choice in round 2 over one joined step): "Compétences" holds
 * weapons and proficiencies as two tabs, then "Talents de voleur" is its own step; "Points de vie"
 * keeps hit points alone. The parts of a step as tabs, each with what is left in it (a red number while some is left). */
function PartTabs({ who, parts, current }: { who: Fixture; parts: Part[]; current: Part }) {
  const set = who.sets[0];
  const info: Record<Part, [string, number]> = {
    weapons: [tr("Maîtrises d'armes", "Weapon proficiencies"), who.weaponSlots.available - spentWeapon(who)],
    proficiencies: [tr("Compétences et langues", "Proficiencies and languages"), who.nonWeaponSlots.available - spentNonWeapon(who)],
    points: [set ? l2(set.name) : "", set ? set.available - spentPoints(set) : 0],
  };
  return (
    <div className="flex shrink-0 gap-1 border-b border-border">
      {parts.map((p) => (
        <span
          key={p}
          className={cn(
            "flex items-center gap-2 px-3 py-2 text-body",
            p === current ? "border-b-2 border-tint font-medium text-ink" : "text-ink-muted",
          )}
        >
          {info[p][0]}
          <b className={cn("tabular-nums", info[p][1] > 0 ? "text-danger" : "text-ink-muted")}>
            {info[p][1]}
          </b>
        </span>
      ))}
    </div>
  );
}

/** The creation screen as the app draws it (CreationFlow): steps on the left, the step in the
 * middle, the character's summary on the right with the scores and what is left to spend. */
function CreationScreen({
  who,
  steps,
  current,
  lead,
  lit,
  children,
}: {
  who: Fixture;
  steps: [string, string][];
  current: number;
  lead: [string, string];
  lit?: Ability[];
  children: ReactNode;
}) {
  const set = who.sets[0];
  const left: [string, number][] = [
    [tr("Maîtrises d'armes", "Weapon proficiencies"), who.weaponSlots.available - spentWeapon(who)],
    [tr("Compétences", "Proficiencies"), who.nonWeaponSlots.available - spentNonWeapon(who)],
    ...(set ? [[l2(set.name), set.available - spentPoints(set)] as [string, number]] : []),
  ];
  return (
    <div className="mx-auto flex h-full max-w-[1680px] flex-col gap-3 overflow-hidden p-3">
      <header className="zone-card flex shrink-0 items-center justify-between gap-4 px-4 py-3">
        <h1 className="text-headline">{tr("Nouveau personnage", "New character")}</h1>
        <Button>
          <X aria-hidden />
          {tr("Quitter", "Leave")}
        </Button>
      </header>
      <div className="flex min-h-0 flex-1 gap-3">
        <nav className="zone-card flex w-[clamp(14rem,15vw,16rem)] shrink-0 flex-col">
          <ol className="flex min-h-0 flex-col gap-1 overflow-y-auto p-2">
            {steps.map((st, i) => (
              <li
                key={st[1]}
                className={cn(
                  "flex items-center gap-2 rounded-lg px-2.5 py-2 text-body",
                  i === current ? "selected font-medium" : i < current ? "text-ink" : "text-ink-muted",
                )}
              >
                <span className="flex size-5 shrink-0 items-center justify-center rounded-full border border-input text-label tabular-nums">
                  {i < current ? <Check aria-hidden className="size-3 text-success" /> : i + 1}
                </span>
                <span className="break-words">{l2(st)}</span>
              </li>
            ))}
          </ol>
        </nav>
        <main className="stage-card flex min-w-0 flex-1 flex-col">
          <div className="card-band flex shrink-0 flex-col gap-0.5 px-5 py-3">
            <span className="text-label text-ink-muted">
              {tr(`Étape ${current + 1} sur ${steps.length}`, `Step ${current + 1} of ${steps.length}`)}
            </span>
            <h2 className="text-headline">{l2(steps[current])}</h2>
            <p className="text-body text-ink-muted">{l2(lead)}</p>
          </div>
          <div className="relative flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4">{children}</div>
          <div className="flex shrink-0 items-center gap-3 border-t border-border px-5 py-3">
            <Button>{tr("Précédent", "Back")}</Button>
            <span className="flex-1" />
            <Button variant="main">{tr("Suivant", "Next")}</Button>
          </div>
        </main>
        <aside className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col gap-4 overflow-y-auto px-4 pt-3 pb-4">
          <div className="flex flex-col gap-0.5">
            <span className="text-title break-words">{who.name}</span>
            <span className="text-body break-words text-ink-muted">
              {l2(who.race)} · {who.classes.map((c) => `${l2(c.name)} ${c.level}`).join(" / ")}
            </span>
          </div>
          <Abilities who={who} lit={lit} thac0={false} />
          <section className="flex flex-col gap-1">
            <span className="section-label">{tr("À dépenser", "To spend")}</span>
            <dl className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-1 text-body">
              {left.map(([k, v]) => (
                <Fragment key={k}>
                  <dt className="break-words text-ink-muted">{k}</dt>
                  <dd className={cn("text-right font-semibold tabular-nums", v > 0 ? "text-danger" : "text-ink-muted")}>
                    {v}
                  </dd>
                </Fragment>
              ))}
            </dl>
          </section>
        </aside>
      </div>
    </div>
  );
}

const STEP = {
  class: ["Classe", "Class"],
  race: ["Race", "Race"],
  scores: ["Caractéristiques", "Ability scores"],
  hp: ["Points de vie", "Hit points"],
  profs: ["Compétences", "Proficiencies"],
  talents: ["Talents de voleur", "Thief skills"],
  identity: ["Identité", "Identity"],
  review: ["Récapitulatif", "Review"],
} satisfies Record<string, [string, string]>;
const SEPARATE: [string, string][] = [STEP.class, STEP.race, STEP.scores, STEP.hp, STEP.profs, STEP.talents, STEP.identity, STEP.review];

// ---------- Act · Skills ----------

/** Act · Skills: rolls only (decided in triage). One Manage button opens the Skills activity; it
 * takes the same red edge as Edit while something is left to spend. */
function ActSkills({ who }: { who: Fixture }) {
  const set = who.sets[0];
  const tabs: [string, string][] = [["Armes", "Weapons"], ["Sorts", "Spells"], ["Objets", "Items"], ["Compétences", "Skills"]];
  return (
    <Dashboard activity="act" who={who}>
      <div className="flex min-h-0 flex-1 gap-3">
        <section className="zone-card flex min-w-0 flex-[1.4] flex-col overflow-hidden">
          <div className="flex shrink-0 items-center gap-1 border-b border-border px-3">
            {tabs.map((t, i) => (
              <span
                key={t[1]}
                className={cn(
                  "px-3 py-2.5 text-body",
                  i === 3 ? "border-b-2 border-tint font-medium text-ink" : "text-ink-muted",
                )}
              >
                {l2(t)}
              </span>
            ))}
            <span className="flex-1" />
            <Button size="sm" className={cn(hasPending(who) && "border-danger")}>
              {hasPending(who) ? (
                <span aria-hidden className="size-2 rounded-full bg-danger" />
              ) : (
                <PencilSimple aria-hidden />
              )}
              {tr("Gérer", "Manage")}
            </Button>
          </div>
          <div className="grid min-h-0 flex-1 grid-cols-2 items-start gap-3 overflow-y-auto p-3">
            <section className="flex flex-col gap-1 rounded-lg bg-raised p-3">
              <h3 className="section-label pb-1">{tr("Compétences", "Proficiencies")}</h3>
              {who.picks.map((pk) => (
                <span
                  key={pk.id}
                  className="grid grid-cols-[1fr_auto] items-center gap-3 rounded-md bg-sub-band px-2.5 py-1.5 text-body"
                >
                  <span className="break-words">{skillName(pk.id)}</span>
                  <Target who={who} p={prof(pk.id)} />
                </span>
              ))}
            </section>
            {set && (
              <section className="flex flex-col gap-1 rounded-lg bg-raised p-3">
                <h3 className="section-label pb-1">{l2(set.name)}</h3>
                {set.skills.map((k) => (
                  <span
                    key={k.id}
                    className="grid grid-cols-[1fr_auto] items-center gap-3 rounded-md bg-sub-band px-2.5 py-1.5 text-body"
                  >
                    <span className="break-words underline decoration-dotted underline-offset-2">{skillName(k.id)}</span>
                    <b className="text-key font-semibold tabular-nums">{percent(k)} %</b>
                  </span>
                ))}
              </section>
            )}
          </div>
        </section>
        <section className="zone-card flex min-w-0 flex-1 flex-col gap-2 self-start p-3">
          <h3 className="section-label">{tr("Détail", "Detail")}</h3>
          <p className="text-body text-ink-muted">
            {tr(
              "Cliquez une compétence ou un talent pour le lancer ; le détail du jet s'affiche ici.",
              "Click a proficiency or a skill to roll it; the roll's detail shows here.",
            )}
          </p>
        </section>
      </div>
    </Dashboard>
  );
}


export const SCREENS: Record<string, () => ReactNode> = {
  "a-overview-bard": () => <OverviewA who={BARD} />,
  "a-overview-thief": () => <OverviewA who={THIEF} />,
  "a-overview-fighter": () => <OverviewA who={FIGHTER} />,
  "a-overview-multi": () => <MultiTagged />,
  "a-edit-proficiencies-bard": () => (
    <EditorScreen who={BARD}>
      <ProficiencyEditor who={BARD} open={["rogue", "priest"]} />
    </EditorScreen>
  ),
  "a-edit-weapons-thief": () => (
    <EditorScreen who={THIEF}>
      <WeaponEditor who={THIEF} open={["sword", "blunt"]} />
    </EditorScreen>
  ),
  "a-edit-weapons-fighter": () => (
    <EditorScreen who={FIGHTER}>
      <WeaponEditor who={FIGHTER} open={["sword", "polearm"]} />
    </EditorScreen>
  ),
  "creation-separate-proficiencies-thief": () => (
    <CreationScreen
      who={THIEF_NEW}
      steps={SEPARATE}
      current={4}
      lead={["Les maîtrises d'armes et les compétences, avec les mêmes éditeurs qu'en jeu.", "Weapon proficiencies and proficiencies, with the same editors as in play."]}
    >
      <PartTabs who={THIEF_NEW} parts={["weapons", "proficiencies"]} current="proficiencies" />
      <ProficiencyEditor who={THIEF_NEW} open={["rogue"]} actions={false} />
    </CreationScreen>
  ),
  "creation-separate-points-thief": () => (
    <CreationScreen
      who={THIEF_NEW}
      steps={SEPARATE}
      current={5}
      lit={["dex"]}
      lead={["Les points de talent, avec les curseurs du jeu.", "The skill points, with the sliders used in play."]}
    >
      <PointsEditor set={THIEF_NEW.sets[0]} control="slider" actions={false} />
    </CreationScreen>
  ),
  "act-skills-thief": () => <ActSkills who={THIEF} />,
  "act-skills-fighter": () => <ActSkills who={FIGHTER} />,
  "points-slider-bard": () => (
    <EditorScreen who={BARD} lit={["dex"]}>
      <PointsEditor set={BARD.sets[0]} control="slider" />
    </EditorScreen>
  ),
};

void LANG;
