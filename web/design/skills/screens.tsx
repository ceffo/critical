// The Skills round's screens (docs/design/skills/README.md). Every layout here is a proposal until the
// user approves it. `?screen=<id>&lang=fr|en`.
import type { ReactNode } from "react";
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
  CATALOGUE,
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
} from "./data";
import { Dashboard, l2 } from "./frame";
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

/** How a section says it has something to spend (round 2: the user found the "1 emplacements à
 * dépenser" pill too wordy, since the budget line already counts). Two proposals:
 * - "dot": a small red dot after the title, no text; nothing once all is spent.
 * - "button": no mark by the title; the section's Modifier button itself turns red-edged with a dot. */
type Signal = "dot" | "button";

function Left({ left, signal }: { left: number; signal: Signal }) {
  if (left <= 0 || signal !== "dot") return null;
  return (
    <span
      role="img"
      aria-label={tr("à dépenser", "to spend")}
      className="size-2.5 shrink-0 rounded-full bg-danger"
    />
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

function Edit({ pending = false }: { pending?: boolean }) {
  return (
    <Button size="sm" className={cn(pending && "border-danger")}>
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
      {l2(unit)} · {l2(formula)}
    </p>
  );
}

const SLOTS: [string, string] = ["emplacements", "slots"];
const SLOT: [string, string] = ["empl.", "slot"];
const POINTS: [string, string] = ["points", "points"];

// ---------- read views ----------

function WeaponsRead({ who, action, signal }: { who: Fixture; action?: ReactNode; signal: Signal }) {
  const left = who.weaponSlots.available - spentWeapon(who);
  return (
    <Card
      title={tr("Maîtrises d'armes", "Weapon proficiencies")}
      aside={<Left left={left} signal={signal} />}
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
  className,
  signal,
}: {
  who: Fixture;
  action?: ReactNode;
  className?: string;
  signal: Signal;
}) {
  const left = who.nonWeaponSlots.available - spentNonWeapon(who);
  return (
    <Card
      title={tr("Compétences et langues", "Proficiencies and languages")}
      aside={<Left left={left} signal={signal} />}
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

function SkillsRead({ set, action, signal }: { set: SkillSet; action?: ReactNode; signal: Signal }) {
  const left = set.available - spentPoints(set);
  return (
    <Card
      title={l2(set.name)}
      aside={<Left left={left} signal={signal} />}
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
}: {
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
      <span className="flex gap-2">
        <Button size="sm">{tr("Annuler", "Cancel")}</Button>
        <Button size="sm" variant="main">
          {tr("Enregistrer", "Save")}
        </Button>
      </span>
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

function ProficiencyEditor({ who, open }: { who: Fixture; open: Group[] }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <EditorBar
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

function PointsEditor({ set, control }: { set: SkillSet; control: "slider" }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <EditorBar
        title={l2(set.name)}
        spent={spentPoints(set)}
        available={set.available}
        formula={set.formula}
        unit={POINTS}
      />
      <SkillTable set={set} edit={control} />
      <p className="text-label text-ink-muted">
        {control === "steppers"
          ? tr(
              "− et + par 5 points ; le nombre se tape aussi. Le total et les points restants suivent à chaque clic.",
              "− and + by 5 points; the number can be typed too. The total and the points left follow each click.",
            )
          : tr(
              "La partie pleine : les points du talent. La partie claire : jusqu'où les points restants le mènent. « / 45 » : sa limite (95 % au plus, ou le plafond par talent d'un voleur).",
              "The solid part: the skill's points. The light part: how far the points left take it. “/ 45”: its limit (95% at most, or a thief's cap per skill).",
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

/** Variant A (chosen by the user, round 2): everything read at a glance; one Edit per section turns
 * the stage into its editor. `signal`: how a section with something to spend says so. */
function OverviewA({ who, signal }: { who: Fixture; signal: Signal }) {
  const edit = (left: number) => <Edit pending={signal === "button" && left > 0} />;
  return (
    <Dashboard activity="skills" who={who}>
      <Stage>
        <div className="grid grid-cols-2 items-start gap-3">
          <ProficienciesRead
            who={who}
            signal={signal}
            action={edit(who.nonWeaponSlots.available - spentNonWeapon(who))}
            className="row-span-2"
          />
          <WeaponsRead who={who} signal={signal} action={edit(who.weaponSlots.available - spentWeapon(who))} />
          {who.sets.map((s) => (
            <SkillsRead key={s.id} set={s} signal={signal} action={edit(s.available - spentPoints(s))} />
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

export const SCREENS: Record<string, () => ReactNode> = {
  "a-overview-bard-dot": () => <OverviewA who={BARD} signal="dot" />,
  "a-overview-bard-button": () => <OverviewA who={BARD} signal="button" />,
  "a-edit-proficiencies-bard": () => (
    <EditorScreen who={BARD}>
      <ProficiencyEditor who={BARD} open={["rogue", "priest"]} />
    </EditorScreen>
  ),
  "points-slider-bard": () => (
    <EditorScreen who={BARD} lit={["dex"]}>
      <PointsEditor set={BARD.sets[0]} control="slider" />
    </EditorScreen>
  ),
};

void LANG;
void Prohibit;
