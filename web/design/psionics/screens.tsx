// The screens of the psionics design round 2 (docs/design/psionics/README.md), one per id:
// `?screen=<id>`. Every part of the Powers tab is a template over the catalogue's fields
// (ADR 0043 step 8: "a power's rules are its data"): the section a power sits in comes from its
// `kind` and `discipline`, its row's cost and maintenance tags from `initial_cost` and
// `maintenance`, and the inline form its Use button opens from the shape of `initial_cost`. No
// branch names a power: the two fixtures (data.ts) go through the same code.
import type { ReactNode } from "react";
import {
  ArrowRight,
  CaretDown,
  Check,
  Lightning,
  MagnifyingGlass,
  Minus,
  Moon,
  Plus,
  ShieldCheck,
  StopCircle,
  Warning,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { lt, tr } from "./lang";
import { Dashboard, type Effect } from "./frame";
import {
  ACTIVITY,
  ATTACK_VS_DEFENCE,
  CATALOGUE,
  CONTACT_COST,
  CONTACT_MODIFIERS,
  DISCIPLINES,
  JORAN,
  MODIFIER_WORDS,
  PSP_RECOVERY,
  YSOLDE,
  knownPowers,
  power,
  scoreOf,
  type Discipline,
  type Fixture,
  type Power,
} from "./data";

/* ---------- small controls ---------- */

function Stepper({ value, w = "w-10" }: { value: string; w?: string }) {
  return (
    <span className="flex items-center gap-1">
      <span className="flex size-7 items-center justify-center rounded-md border border-input">
        <Minus className="size-3.5" aria-hidden />
      </span>
      <span
        className={cn(
          "flex h-7 items-center justify-center rounded-md border border-input text-body tabular-nums",
          w,
        )}
      >
        {value}
      </span>
      <span className="flex size-7 items-center justify-center rounded-md border border-input">
        <Plus className="size-3.5" aria-hidden />
      </span>
    </span>
  );
}

function Select({ value }: { value: string }) {
  return (
    <span className="flex h-7 items-center justify-between gap-2 rounded-lg border border-input bg-surface px-2 text-body whitespace-nowrap">
      {value}
      <CaretDown className="size-3.5 text-ink-muted" aria-hidden />
    </span>
  );
}

function Seg({ options, chosen }: { options: string[]; chosen: number }) {
  return (
    <span className="flex gap-1">
      {options.map((o, i) => (
        <span
          key={o}
          className={cn(
            "flex h-7 items-center rounded-md px-2.5 text-body",
            i === chosen ? "chip chip-chosen" : "chip",
          )}
        >
          {o}
        </span>
      ))}
    </span>
  );
}

function Pips({ n, of = 3 }: { n: number; of?: number }) {
  return (
    <span className="flex gap-1" aria-label={`${n}/${of}`}>
      {Array.from({ length: of }, (_, i) => (
        <span
          key={i}
          className={cn("size-2.5 rounded-full border border-tint", i < n && "bg-tint")}
        />
      ))}
    </span>
  );
}

/** A d20 the player types for the DM, with the DM's "success" tick (ADR 0044 Q6). */
function DmRoll({ n, value, ok }: { n: number; value?: string; ok?: boolean }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className="text-label text-ink-muted">{tr(`Jet ${n}`, `Roll ${n}`)}</span>
      <span className="flex h-7 w-10 items-center justify-center rounded-md border border-input bg-surface text-body tabular-nums">
        {value ?? ""}
      </span>
      <span
        className={cn(
          "flex h-7 items-center gap-1 rounded-md px-2 text-label",
          ok ? "chip chip-chosen" : "chip",
        )}
      >
        <Check className="size-3.5" aria-hidden />
        {tr("réussi", "success")}
      </span>
    </span>
  );
}

const signed = (n: number) => (n > 0 ? `+${n}` : n < 0 ? `−${-n}` : "±0");
const psp = (n: number) => tr(`${n} PFP`, `${n} PSP`);

/* ---------- data → words (each a rule on a field, never on an id) ---------- */

/** `initial_cost`'s four shapes. */
function costText(p: Power): string {
  const c = p.initial_cost;
  if (typeof c === "number") return psp(c);
  if (c === "variable") return tr("coût variable", "variable cost");
  if ("table" in c) return tr("coût selon le niveau", "cost by level");
  return c.plus
    ? tr(`au contact +${c.plus} PFP`, `on contact +${c.plus} PSP`)
    : tr("au contact", "on contact");
}

/** `maintenance`: absent, standing (cost 0), a number or variable, every `period` rounds. */
function upkeepText(p: Power): string | undefined {
  const m = p.maintenance;
  if (!m) return undefined;
  if (m.cost === 0) return tr("permanent", "standing");
  const c = m.cost === "variable" ? tr("variable", "variable") : `+${m.cost}`;
  if (m.period === 1) return tr(`${c} / tour`, `${c} / round`);
  return tr(`${c} / ${m.period} tours`, `${c} / ${m.period} rounds`);
}

/** A power that takes an opponent's mind: an attack mode, a cost read from a table, or contact. */
const takesMind = (p: Power) =>
  p.kind === "attack_mode" || typeof p.initial_cost === "object";

const needsContact = (p: Power) =>
  typeof p.initial_cost === "object" && "kind" in p.initial_cost;

/* ---------- the opponent (ADR 0044 step 2): what the DM tells ---------- */

interface Mind {
  name: string;
  open: boolean;
  level: number;
  /** The defence mode the DM says it raised (any catalogue defence mode). */
  defence?: string;
  /** Tangents held on it (1 or 2), or 3 = contact. */
  held?: number;
  /** Contact made in an earlier round (a contact-cost power may be used on it). */
  earlier?: boolean;
}

const ORC: Mind = { name: tr("Orc chaman", "Orc shaman"), open: false, level: 6, defence: "thought_shield", held: 2 };
const GUARD: Mind = { name: tr("Gardien", "Guard"), open: true, level: 7, held: 3, earlier: true };
const BRANN: Mind = { name: "Brann", open: true, level: 5, held: 3, earlier: true };
const NEW_MIND: Mind = { name: tr("Corbeau", "Raven"), open: true, level: 1 };

/* ---------- the round's state the tab shows ---------- */

interface Scene {
  who: Fixture;
  /** 0 = out of combat (ADR 0005). */
  round: number;
  psp: number;
  minds: Mind[];
  mind?: Mind;
  /** The current defence: a raised mode (this round) or a standing one. */
  defence?: { id: string; standing?: boolean };
  /** A power (not a defence mode) checked this round (ADR 0043 step 12). */
  usedThisRound?: string;
  /** The row whose inline form is open, or the defend form. */
  open?: { power: string } | "defend";
}

/* ---------- the Powers tab ---------- */

function TabBar({ scene }: { scene: Scene }) {
  const used = scene.usedThisRound ? power(scene.usedThisRound) : undefined;
  return (
    <div className="card-band flex items-center gap-1 px-3">
      {[tr("Armes", "Weapons"), tr("Pouvoirs", "Powers"), tr("Objets", "Items"), tr("Compétences", "Skills")].map(
        (t, i) => (
          <span
            key={t}
            className={cn(
              "px-3 py-2 text-body",
              i === 1 ? "font-medium shadow-[inset_0_-2px_0_var(--tint)]" : "text-ink-muted",
            )}
          >
            {t}
          </span>
        ),
      )}
      <span className="ml-auto flex items-center gap-3">
        <span
          className={cn(
            "flex items-center gap-1.5 rounded-md px-2 py-0.5 text-label",
            used ? "bg-raised text-warning" : "bg-raised text-ink-muted",
          )}
        >
          {scene.round === 0
            ? tr("Hors combat", "Out of combat")
            : used
              ? tr(`Pouvoir du tour : ${lt(used.name)}`, `Power this round: ${lt(used.name)}`)
              : tr("Pouvoir du tour : libre", "Power this round: free")}
        </span>
        <span className="flex items-center gap-2 text-label text-ink-muted">
          {tr("Ajustement", "Adjustment")}
          <Stepper value="0" w="w-9" />
        </span>
      </span>
    </div>
  );
}

/** "Ma défense": one chip per known defence mode (kind), raised in one tap, no roll (ADR 0044 step 4). */
function DefenceBand({ scene }: { scene: Scene }) {
  const modes = knownPowers(scene.who).filter((p) => p.kind === "defence_mode");
  return (
    <div className="group-band flex flex-col gap-2 rounded-lg px-3 py-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="section-label mr-1">{tr("Ma défense", "My defence")}</span>
        {modes.map((p) => {
          const standing = p.maintenance?.cost === 0;
          const on = scene.defence?.id === p.id;
          const outOfCombat = scene.round === 0 && !standing;
          return (
            <span
              key={p.id}
              className={cn(
                "flex h-8 items-center gap-1.5 rounded-md px-2.5 text-body",
                on ? "chip chip-chosen" : "chip",
                outOfCombat && "border-dashed text-ink-muted",
              )}
            >
              <ShieldCheck weight={on ? "fill" : "regular"} className="size-4" aria-hidden />
              {lt(p.name)}
              <span className="text-label text-ink-muted">
                {on
                  ? standing
                    ? tr("permanente", "standing")
                    : tr("dressée ce tour", "raised this round")
                  : standing
                    ? tr("permanente · 0", "standing · 0")
                    : outOfCombat
                      ? tr(`${p.initial_cost} PFP · en combat`, `${p.initial_cost} PSP · in combat`)
                      : psp(p.initial_cost as number)}
              </span>
            </span>
          );
        })}
        {modes.length === 0 && (
          <span className="text-body text-ink-muted">{tr("Aucun mode de défense connu.", "No defence mode known.")}</span>
        )}
        <Button
          variant="quiet"
          className="ml-auto"
          aria-expanded={scene.open === "defend"}
          disabled={scene.round === 0}
        >
          <Lightning aria-hidden />
          {tr("Attaque subie", "Incoming attack")}
        </Button>
      </div>
    </div>
  );
}

/** The DM's attack on the character (ADR 0044 step 8): the mode (any catalogue attack mode) and
 * one or two rolls; the current defence answers. */
function defendForm(current?: Power): Form {
  const attack = power("mind_thrust");
  const cell = current ? ATTACK_VS_DEFENCE[`${attack.id}/${current.id}`] : undefined;
  return {
    info: current
      ? tr(`Table 14 : ${signed(cell ?? 0)} pour le MJ`, `Table 14: ${signed(cell ?? 0)} for the DM`)
      : tr("aucune défense : ses réussites l'emportent", "no defence: his successes win"),
    controls: (
      <>
        <span className="flex items-center gap-2 text-body">
          {tr("Le MJ attaque avec", "The DM attacks with")}
          <Select value={lt(attack.name)} />
        </span>
        <DmRoll n={1} value="12" ok />
        <DmRoll n={2} />
        <Launch label={tr("Résoudre", "Resolve")} />
      </>
    ),
  };
}

/** The minds the attacks and the mind-taking powers aim at: shown only when a known power takes one. */
function MindBand({ scene }: { scene: Scene }) {
  const m = scene.mind;
  return (
    <div className="group-band flex flex-col gap-2 rounded-lg px-3 py-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="section-label mr-1">{tr("Cible", "Target")}</span>
        {scene.minds.map((t) => (
          <span
            key={t.name}
            className={cn(
              "flex h-8 items-center gap-2 rounded-md px-2.5 text-body",
              t === m ? "chip chip-chosen" : "chip",
            )}
          >
            {t.name}
            {t.held && t.held < 3 ? <Pips n={t.held} /> : null}
            {t.held === 3 && <span className="text-label text-ink-muted">contact</span>}
          </span>
        ))}
        <Button variant="quiet">
          <Plus aria-hidden />
          {tr("Nouvelle", "New")}
        </Button>
      </div>
      {m && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <span className="flex items-center gap-2 text-body">
            {tr("Esprit", "Mind")}
            <Seg options={[tr("Ouvert", "Open"), tr("Fermé", "Closed")]} chosen={m.open ? 0 : 1} />
          </span>
          <span className="flex items-center gap-2 text-body">
            {tr("Niveau", "Level")}
            <Stepper value={String(m.level)} />
          </span>
          <span className="flex items-center gap-2 text-body">
            {tr("Défense", "Defence")}
            <Select value={m.defence ? lt(power(m.defence).name) : tr("Aucune", "None")} />
          </span>
        </div>
      )}
    </div>
  );
}

/** One power row: name, kind and cost tags from the data, the score, the action. */
function PowerRow({ scene, p, children }: { scene: Scene; p: Power; children?: ReactNode }) {
  const base = scoreOf(scene.who, p);
  const cell =
    p.kind === "attack_mode" && scene.mind?.defence
      ? ATTACK_VS_DEFENCE[`${p.id}/${scene.mind.defence}`]
      : undefined;
  const score = base + (cell ?? 0);
  const cost = typeof p.initial_cost === "number" ? p.initial_cost : 0;
  const short = cost > scene.psp;
  const contactReady = scene.minds.some((t) => t.held === 3 && t.earlier);
  const blocked = needsContact(p) && !contactReady;
  const upkeep = upkeepText(p);
  const open = typeof scene.open === "object" && scene.open.power === p.id;
  const kindWord = p.kind === "science" ? tr("science", "science") : p.kind === "devotion" ? tr("dévotion", "devotion") : undefined;
  return (
    <li className={cn("flex break-inside-avoid flex-col rounded-md", open ? "bg-selected/60" : "odd:bg-raised/50")}>
      <div className="grid grid-cols-[1fr_auto_auto] items-center gap-3 px-2.5 py-1">
        <span className="flex min-w-0 flex-col">
          <span className="text-body font-medium">{lt(p.name)}</span>
          <span className="text-label text-ink-muted">
            {[kindWord, costText(p), upkeep].filter(Boolean).join(" · ")}
          </span>
        </span>
        <span className="flex flex-col items-end">
          <span className="text-label text-ink-muted">{tr("Score", "Score")}</span>
          <span className="text-body font-semibold tabular-nums">
            {score}
            {cell !== undefined && cell !== 0 && (
              <span className="font-normal text-ink-muted"> ({base}{signed(cell)})</span>
            )}
          </span>
        </span>
        <Button
          variant="quiet"
          className="min-w-24"
          aria-expanded={open}
          disabled={short || blocked || (p.kind === "attack_mode" && !scene.mind)}
        >
          {p.kind === "attack_mode" ? tr("Attaquer", "Attack") : tr("Utiliser", "Use")}
        </Button>
      </div>
      {blocked && (
        <span className="px-2.5 pb-1 text-label text-ink-muted">
          {tr("Il faut un contact établi lors d'un tour précédent.", "Needs a contact made in an earlier round.")}
        </span>
      )}
      {children}
    </li>
  );
}

function Launch({ label }: { label: string }) {
  return (
    <Button variant="main" className="ml-auto">
      {label}
    </Button>
  );
}

/** What the action bar shows: a line of information and the controls (ending with the roll). */
interface Form {
  info?: ReactNode;
  controls: ReactNode;
}

/** The form a row opens, chosen by the kind attack_mode or the shape of `initial_cost`. */
function formOf(scene: Scene, p: Power): Form {
  const c = p.initial_cost;
  if (p.kind === "attack_mode") {
    const m = scene.mind!;
    const cost = psp(c as number);
    return m.defence
      ? {
          info: tr("deux jets, un seul coût", "two checks, one cost"),
          controls: (
            <>
              <span className="text-body">{tr(`Jets du MJ (${lt(power(m.defence).name)})`, `DM's rolls (${lt(power(m.defence).name)})`)}</span>
              <DmRoll n={1} value="8" ok />
              <DmRoll n={2} />
              <Launch label={tr(`Lancer · ${cost}`, `Roll · ${cost}`)} />
            </>
          ),
        }
      : {
          info: tr("aucune défense levée : pas de jet du MJ", "no defence raised: no DM roll"),
          controls: <Launch label={tr(`Lancer · ${cost}`, `Roll · ${cost}`)} />,
        };
  }
  if (c === "variable") {
    return {
      info: tr("coûts d'après le texte du pouvoir", "costs from the power's text"),
      controls: (
        <>
          <span className="flex items-center gap-2 text-body">
            {tr("Coût initial", "Initial cost")}
            <Stepper value="3" />
          </span>
          {p.maintenance?.cost === "variable" && (
            <span className="flex items-center gap-2 text-body">
              {tr("Maintien par tour", "Maintenance a round")}
              <Stepper value="1" />
            </span>
          )}
          <Launch label={tr("Lancer · 3 PFP", "Roll · 3 PSP")} />
        </>
      ),
    };
  }
  if (typeof c === "object" && "table" in c) {
    const m = scene.mind!;
    const row = CONTACT_COST.find((r) => m.level >= r.min_level && m.level <= r.max_level)!;
    // One control per modifier kind of the table: a menu when the kind has several rows, a toggle
    // when it has one.
    const kinds = [...new Set(CONTACT_MODIFIERS.map((r) => r.kind))];
    const chosen: Record<string, string> = { distance: "miles_10", life_order: "mammal" };
    let total = scoreOf(scene.who, p);
    const parts = [`${tr("score", "score")} ${total}`];
    for (const k of kinds) {
      const rows = CONTACT_MODIFIERS.filter((r) => r.kind === k);
      const pick = rows.length > 1 ? rows.find((r) => r.code === chosen[k]) : undefined;
      if (pick) {
        total += pick.modifier;
        parts.push(`${signed(pick.modifier)} ${lt(MODIFIER_WORDS[k]).toLowerCase()}`);
      }
    }
    return {
      info: (
        <>
          {tr(`niveau ${m.level} : ${psp(row.cost)}`, `level ${m.level}: ${psp(row.cost)}`)}{" "}
          {tr(`(table, niv. ${row.min_level}–${row.max_level})`, `(table, lvl ${row.min_level}–${row.max_level})`)} · {parts.join(" ")} = {total}
        </>
      ),
      controls: (
        <>
          {kinds.map((k) => {
            const rows = CONTACT_MODIFIERS.filter((r) => r.kind === k);
            if (rows.length === 1)
              return (
                <span key={k} className="chip flex h-7 items-center gap-1 rounded-md px-2.5 text-body">
                  {lt(MODIFIER_WORDS[rows[0].code])} {signed(rows[0].modifier)}
                </span>
              );
            const pick = rows.find((r) => r.code === chosen[k])!;
            return (
              <span key={k} className="flex items-center gap-2 text-body">
                {lt(MODIFIER_WORDS[k])}
                <Select value={`${lt(MODIFIER_WORDS[pick.code])} ${signed(pick.modifier)}`} />
              </span>
            );
          })}
          <Launch label={tr(`Lancer · ${psp(row.cost)}`, `Roll · ${psp(row.cost)}`)} />
        </>
      ),
    };
  }
  // Contact cost: only the minds holding a contact from an earlier round can be picked.
  const ready = scene.minds.filter((t) => t.held === 3 && t.earlier);
  const plus = typeof c === "object" && "kind" in c ? c.plus ?? 0 : 0;
  return {
    info: (
      <>
        {plus
          ? tr(`contact déjà payé, +${plus} PFP`, `contact already paid, +${plus} PSP`)
          : tr("contact déjà payé : rien de plus", "contact already paid: nothing more")}
        {p.maintenance?.cost ? tr(" · tant qu'il est maintenu, le contact ne coûte rien", " · while it is kept up, the contact costs nothing") : ""}
      </>
    ),
    controls: (
      <>
        <span className="flex items-center gap-2 text-body">
          {tr("Sur", "On")}
          {ready.map((t, i) => (
            <span key={t.name} className={cn("flex h-7 items-center rounded-md px-2.5 text-body", i === 0 ? "chip chip-chosen" : "chip")}>
              {t.name}
            </span>
          ))}
        </span>
        <Launch label={tr(`Lancer · ${psp(plus)}`, `Roll · ${psp(plus)}`)} />
      </>
    ),
  };
}

function Group({ title, note, children }: { title: string; note?: string; children: ReactNode }) {
  return (
    <section className="mb-3">
      <h3 className="mb-1 flex items-baseline gap-2 px-1 break-after-avoid">
        <span className="section-label">{title}</span>
        {note && <span className="text-label text-ink-muted">{note}</span>}
      </h3>
      <ul className="flex flex-col gap-0.5">{children}</ul>
    </section>
  );
}

/** The groups, from the data: the attack modes first (kind), then the sciences and devotions by
 * discipline, in the order the player learned them. Defence modes live in the defence band. */
function groupsOf(who: Fixture) {
  const known = knownPowers(who);
  const attacks = known.filter((p) => p.kind === "attack_mode");
  const rest = known.filter((p) => p.kind === "science" || p.kind === "devotion");
  const order: Discipline[] = [];
  for (const p of rest) if (!order.includes(p.discipline)) order.push(p.discipline);
  return { attacks, disciplines: order.map((d) => [d, rest.filter((p) => p.discipline === d)] as const) };
}

function PowersTab({ scene }: { scene: Scene }) {
  const { attacks, disciplines } = groupsOf(scene.who);
  const showMinds = knownPowers(scene.who).some(takesMind);
  const row = (p: Power) => {
    return (
      <PowerRow key={p.id} scene={scene} p={p} />
    );
  };
  const defenceNote = scene.mind?.defence
    ? tr(`contre ${lt(power(scene.mind.defence).name)} (Table 14)`, `against ${lt(power(scene.mind.defence).name)} (Table 14)`)
    : scene.mind
      ? tr("cible sans défense : pas d'ajustement", "undefended target: no adjustment")
      : tr("choisissez une cible", "pick a target");
  return (
    <>
      <TabBar scene={scene} />
      <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-3">
        <DefenceBand scene={scene} />
        {showMinds && <MindBand scene={scene} />}
        <div className="columns-2 gap-4 pt-1">
          {attacks.length > 0 && (
            <Group title={tr("Modes d'attaque", "Attack modes")} note={defenceNote}>
              {attacks.map(row)}
            </Group>
          )}
          {disciplines.map(([d, ps]) => (
            <Group key={d} title={lt(DISCIPLINES[d])}>
              {ps.map(row)}
            </Group>
          ))}
        </div>
      </div>
      {scene.open && <ActionBar scene={scene} />}
    </>
  );
}

/** The one place every entry before a roll is made: the DM's rolls, a cost the player types, a
 * table's row and modifiers, the mind a contact power works on. Docked under the lists, full width,
 * so opening it never moves a row; the row it belongs to stays marked. */
function ActionBar({ scene }: { scene: Scene }) {
  const open = scene.open!;
  const p = open === "defend" ? undefined : power(open.power);
  const current = scene.defence ? power(scene.defence.id) : undefined;
  const title =
    open === "defend"
      ? tr(`Attaque subie · ${current ? lt(current.name) : "sans défense"}`, `Incoming attack · ${current ? lt(current.name) : "no defence"}`)
      : p && takesMind(p) && !needsContact(p) && scene.mind
        ? `${lt(p.name)} → ${scene.mind.name}`
        : lt(p!.name);
  const form = open === "defend" ? defendForm(current) : formOf(scene, p!);
  return (
    <div className="flex flex-col gap-2 border-t-2 border-tint bg-selected/50 px-3 py-2">
      <div className="flex items-center gap-3">
        <span className="text-body font-semibold">{title}</span>
        {form.info && <span className="text-label text-ink-muted">{form.info}</span>}
        <Button variant="ghost" size="icon-sm" className="ml-auto" aria-label={tr("Fermer", "Close")}>
          <X aria-hidden />
        </Button>
      </div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">{form.controls}</div>
    </div>
  );
}

/* ---------- the right column: result and maintained ---------- */

function ResultFrame({ badge, title, children }: { badge: string; title: string; children: ReactNode }) {
  return (
    <section className="zone-card latest flex min-h-0 min-w-0 flex-col">
      <div className="card-band flex min-h-9 items-center gap-2 px-4 py-2">
        <span className="chip px-1.5 text-label">{badge}</span>
        <span className="text-body font-medium">{title}</span>
      </div>
      <div className="flex flex-col gap-3 overflow-y-auto p-4">{children}</div>
    </section>
  );
}

function Headline({ total, verdict, good, sub }: { total: string; verdict: string; good: boolean; sub: string }) {
  return (
    <div className="flex items-center gap-3">
      <span className="flex size-11 items-center justify-center rounded-lg bg-tint text-key font-bold text-on-accent">
        {total}
      </span>
      <span className="flex flex-col">
        <span className={cn("text-key", good ? "text-success" : "text-danger")}>{verdict}</span>
        <span className="text-label text-ink-muted">{sub}</span>
      </span>
    </div>
  );
}

function Facts({ rows }: { rows: [string, string][] }) {
  return (
    <div className="grid grid-cols-3 gap-2">
      {rows.map(([k, v]) => (
        <span key={k} className="flex flex-col rounded-md bg-raised px-3 py-1.5">
          <span className="text-label text-ink-muted">{k}</span>
          <span className="text-body font-semibold tabular-nums">{v}</span>
        </span>
      ))}
    </div>
  );
}

function Duel({ n, ours, score, theirs, won, skipped }: { n: number; ours?: string; score?: number; theirs?: string; won?: boolean; skipped?: string }) {
  return (
    <div
      className={cn(
        "grid grid-cols-[auto_1fr_auto] items-center gap-3 rounded-lg px-3 py-2",
        skipped ? "border border-dashed border-border" : "bg-raised",
      )}
    >
      <span className="text-label text-ink-muted">{tr(`Duel ${n}`, `Contest ${n}`)}</span>
      {skipped ? (
        <span className="text-body text-ink-muted">{skipped}</span>
      ) : (
        <span className="text-body">
          {tr("Vous", "You")} <b className="tabular-nums">{ours}</b>{" "}
          <span className="text-ink-muted">({tr("score", "score")} {score})</span> · {tr("MJ", "DM")}{" "}
          <b className="tabular-nums">{theirs}</b>
        </span>
      )}
      {!skipped && (
        <span className={cn("text-body font-semibold", won ? "text-success" : "text-danger")}>
          {won ? tr("Gagné", "Won") : tr("Perdu", "Lost")}
        </span>
      )}
    </div>
  );
}

function PowerResult({ who, p, on, d20, cost, kept }: { who: Fixture; p: Power; on?: string; d20: number; cost: number; kept?: boolean }) {
  const s = scoreOf(who, p);
  return (
    <ResultFrame badge={tr("Pouvoir", "Power")} title={on ? `${lt(p.name)} · ${on}` : lt(p.name)}>
      <Headline
        total={String(d20)}
        verdict={d20 <= s ? tr("Réussi", "Success") : tr("Échec", "Failed")}
        good={d20 <= s}
        sub={
          tr(`d20 ${d20} contre ${s}`, `d20 ${d20} vs ${s}`) +
          (kept ? tr(" · maintenu", " · kept up") : "")
        }
      />
      <Facts rows={[["d20", String(d20)], [tr("Score", "Score"), String(s)], [tr("Coût", "Cost"), psp(cost)]]} />
    </ResultFrame>
  );
}

function AttackResult({ who }: { who: Fixture }) {
  const p = power("ego_whip");
  return (
    <ResultFrame badge={tr("Attaque", "Attack")} title={`${lt(p.name)} · ${ORC.name}`}>
      <Headline total="11" verdict={tr("Contact établi", "Contact made")} good sub={tr("troisième tangente · 4 PFP", "third tangent · 4 PSP")} />
      <Duel n={1} ours="11" score={scoreOf(who, p)} theirs="8" won />
      <Duel n={2} skipped={tr("Non lancé : le contact est fait, plus d'attaque ce tour.", "Not rolled: contact is made, no more attacks this round.")} />
      <div className="flex items-center gap-3 rounded-lg bg-raised px-3 py-2 text-body">
        <span className="text-ink-muted">{tr("Tangentes", "Tangents")}</span>
        <Pips n={3} />
        <ArrowRight className="size-4 text-ink-muted" aria-hidden />
        <b>{tr("contact, 1 PFP par tour", "contact, 1 PSP a round")}</b>
      </div>
    </ResultFrame>
  );
}

function DefendResult({ who }: { who: Fixture }) {
  const d = power("thought_shield");
  const a = power("mind_thrust");
  return (
    <ResultFrame badge={tr("Défense", "Defence")} title={`${lt(d.name)} ${tr("contre", "against")} ${lt(a.name)}`}>
      <Headline total="9" verdict={tr("Défense battue", "Defence beaten")} good={false} sub={tr("le MJ compte la tangente, rien à noter ici", "the DM counts the tangent, nothing to track here")} />
      <Duel n={1} ours="9" score={scoreOf(who, d)} theirs="12" won={false} />
      <div className="flex items-center justify-between gap-3 rounded-lg bg-raised px-3 py-2">
        <span className="text-body">{tr("Garder ce jet pour les autres duels du tour ?", "Keep this check for the round's other contests?")}</span>
        <Button variant="quiet">{tr("Garder", "Keep")}</Button>
      </div>
    </ResultFrame>
  );
}

function NoResult() {
  return (
    <ResultFrame badge={tr("Pouvoir", "Power")} title={tr("Aucun", "None")}>
      <span className="text-body text-ink-muted">{tr("Le résultat du dernier pouvoir apparaît ici.", "The last power's result shows here.")}</span>
    </ResultFrame>
  );
}

/** A buff row that costs PSPs: a maintained power, a tangent row or a contact row (ADR 0044 step 3). */
interface Kept {
  label: string;
  target?: string;
  cost: number;
  period?: number;
  note?: string;
  pips?: number;
}

const keptOf = (p: Power, target?: string, cost?: number, note?: string): Kept => ({
  label: lt(p.name),
  target,
  cost: cost ?? (typeof p.maintenance?.cost === "number" ? p.maintenance.cost : 0),
  period: p.maintenance?.period,
  note: note ?? (p.maintenance?.cost === 0 ? tr("permanent", "standing") : undefined),
});
const tangentRow = (m: Mind): Kept => ({ label: tr("Tangentes", "Tangents"), target: m.name, cost: 1, pips: m.held });
const contactRow = (m: Mind, note?: string): Kept => ({ label: "Contact", target: m.name, cost: note ? 0 : 1, pips: 3, note });

/** The PSPs a Next Round charges (rows every round; a longer period is charged when it falls due). */
const perRound = (rows: Kept[]) => rows.filter((r) => (r.period ?? 1) === 1).reduce((s, r) => s + r.cost, 0);

function KeptCard({ rows, psp: now }: { rows: Kept[]; psp: number }) {
  const total = perRound(rows);
  const after = now - total;
  return (
    <section className="zone-card flex min-h-0 min-w-0 flex-col">
      <div className="card-band flex min-h-9 items-center justify-between gap-2 px-4 py-2">
        <h2 className="section-label">{tr("Maintenus", "Kept up")}</h2>
        <span className="text-body tabular-nums">{tr(`${total} PFP par tour`, `${total} PSP a round`)}</span>
      </div>
      <ul className="flex flex-col gap-0.5 overflow-y-auto p-2">
        {rows.length === 0 && <li className="px-2 py-1 text-body text-ink-muted">{tr("Rien n'est maintenu.", "Nothing is kept up.")}</li>}
        {rows.map((r) => (
          <li key={r.label + (r.target ?? "")} className="grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-md px-2 py-1 odd:bg-raised/60">
            <span className="flex min-w-0 flex-col">
              <span className="text-body">
                {r.label}
                {r.target ? ` · ${r.target}` : ""}
              </span>
              <span className="flex items-center gap-2 text-label text-ink-muted">
                {r.pips ? <Pips n={r.pips} /> : null}
                {r.note}
              </span>
            </span>
            <span className="text-body text-ink-muted tabular-nums">
              {r.cost === 0
                ? tr("gratuit", "free")
                : (r.period ?? 1) === 1
                  ? psp(r.cost)
                  : tr(`${r.cost} PFP / ${r.period} tours`, `${r.cost} PSP / ${r.period} rounds`)}
            </span>
            <Button variant="quiet">
              <StopCircle aria-hidden />
              {tr("Arrêter", "Stop")}
            </Button>
          </li>
        ))}
      </ul>
      <div className={cn("mt-auto flex items-center gap-2 border-t border-border px-4 py-2 text-body", after < 0 && "text-danger")}>
        {after < 0 && <Warning weight="fill" className="size-4" aria-hidden />}
        {after >= 0
          ? tr(`Tour suivant : ${now} → ${after} PFP`, `Next round: ${now} → ${after} PSP`)
          : tr(`Tour suivant : il manque ${-after} PFP, un pouvoir s'arrête`, `Next round: ${-after} PSP short, a power ends`)}
      </div>
    </section>
  );
}

function Journal({ rows }: { rows: [string, string][] }) {
  return (
    <section className="zone-card flex min-h-0 flex-[2] flex-col">
      <div className="card-band flex min-h-9 items-center gap-3 px-4 py-2">
        <h2 className="section-label">Journal</h2>
      </div>
      <ul className="flex flex-col gap-0.5 overflow-y-auto p-2">
        {rows.map(([n, t], i) => (
          <li key={i} className="grid grid-cols-[3rem_1fr_auto] gap-3 rounded-md px-2 py-0.5 text-body">
            <span className="text-right font-semibold text-highlight tabular-nums">{n}</span>
            <span>{t}</span>
            <span className="text-ink-muted tabular-nums">20:4{i}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ActStage({ scene, result, kept, journal }: { scene: Scene; result: ReactNode; kept: Kept[]; journal: [string, string][] }) {
  return (
    <>
      <div className="grid min-h-0 flex-[5] grid-cols-[minmax(34rem,1.7fr)_minmax(18rem,1fr)] gap-3">
        <section className="stage-card flex min-h-0 min-w-0 flex-col">
          <PowersTab scene={scene} />
        </section>
        <div className="flex min-h-0 min-w-0 flex-col gap-3">
          {result}
          <KeptCard rows={kept} psp={scene.psp} />
        </div>
      </div>
      <Journal rows={journal} />
    </>
  );
}

const fx = (rows: Kept[]): Effect[] =>
  rows.map((r) => ({
    name: r.label + (r.target ? ` · ${r.target}` : ""),
    width: 100,
    note: r.pips && r.pips < 3 ? tr(`${r.pips} sur 3`, `${r.pips} of 3`) : tr("jusqu'à arrêt", "until stopped"),
  }));

function Act({ scene, result, kept, journal, last, overlay, restOpen }: { scene: Scene; result: ReactNode; kept: Kept[]; journal: [string, string][]; last?: ReactNode; overlay?: ReactNode; restOpen?: boolean }) {
  return (
    <Dashboard
      activity="act"
      who={scene.who}
      psp={scene.psp}
      effects={fx(kept)}
      upkeep={perRound(kept)}
      round={scene.round}
      last={last}
      overlay={overlay}
      restOpen={restOpen}
    >
      <ActStage scene={scene} result={result} kept={kept} journal={journal} />
    </Dashboard>
  );
}

/* ---------- the table's scenes ---------- */

const MINDLINK = power("mindlink");
const JORAN_KEPT: Kept[] = [
  keptOf(MINDLINK, GUARD.name),
  contactRow(GUARD, tr("gratuit tant qu'un pouvoir l'utilise", "free while a power uses it")),
  tangentRow(ORC),
  keptOf(power("mind_blank")),
];

/** Joran, round 3: an attack mode's row open on the orc (thought shield): the DM's two rolls. */
function ActAttack() {
  return (
    <Act
      scene={{ who: JORAN, round: 3, psp: 71, minds: [ORC, GUARD], mind: ORC, defence: { id: "mind_blank", standing: true }, open: { power: "ego_whip" } }}
      result={<PowerResult who={JORAN} p={MINDLINK} on={GUARD.name} d20={9} cost={0} kept />}
      kept={JORAN_KEPT}
      journal={[
        ["9", tr("Lien spirituel sur Gardien : réussi, 0 PFP, maintenu", "Mindlink on Guard: success, 0 PSP, kept up")],
        ["", tr("Tour 3 : maintien payé, 9 PFP", "Round 3: maintenance paid, 9 PSP")],
      ]}
      last={<><b className="text-key">9</b> {lt(MINDLINK.name)} · <span className="text-success">{tr("réussi", "success")}</span></>}
    />
  );
}

/** The attack's answer: the third tangent is contact, the second check not rolled. */
function ActAttackResult() {
  const kept = [JORAN_KEPT[0], JORAN_KEPT[1], contactRow(ORC), JORAN_KEPT[3]];
  return (
    <Act
      scene={{ who: JORAN, round: 3, psp: 67, minds: [{ ...ORC, held: 3 }, GUARD], mind: { ...ORC, held: 3 }, defence: { id: "mind_blank", standing: true }, usedThisRound: "ego_whip" }}
      result={<AttackResult who={JORAN} />}
      kept={kept}
      journal={[
        ["11", tr("Cinglement de l'ego sur Orc chaman : duel 1 gagné (11 contre 8), 3e tangente : contact, 4 PFP", "Ego Whip on Orc shaman: contest 1 won (11 vs 8), 3rd tangent: contact, 4 PSP")],
        ["9", tr("Lien spirituel sur Gardien : réussi, maintenu", "Mindlink on Guard: success, kept up")],
      ]}
      last={<><b className="text-key">11</b> {lt(power("ego_whip").name)} · <span className="text-success">contact</span></>}
    />
  );
}

/** Being attacked: thought shield raised this round, the DM's mode and roll typed, then Resolve. */
function ActDefend() {
  const kept = JORAN_KEPT;
  return (
    <Act
      scene={{ who: JORAN, round: 3, psp: 70, minds: [ORC, GUARD], mind: ORC, defence: { id: "thought_shield" }, open: "defend" }}
      result={<DefendResult who={JORAN} />}
      kept={kept}
      journal={[
        ["9", tr("Bouclier de la pensée contre Pression mentale : duel 1 perdu (9 contre 12)", "Thought Shield against Mind Thrust: contest 1 lost (9 vs 12)")],
        ["", tr("Bouclier de la pensée dressé, 1 PFP", "Thought Shield raised, 1 PSP")],
      ]}
      last={<><b className="text-key">9</b> {lt(power("thought_shield").name)} · <span className="text-danger">{tr("battue", "beaten")}</span></>}
    />
  );
}

/** A power whose cost is read from a rule table: the table's row and one control per modifier kind. */
function ActTableCost() {
  return (
    <Act
      scene={{ who: JORAN, round: 3, psp: 71, minds: [ORC, GUARD, NEW_MIND], mind: NEW_MIND, defence: { id: "mind_blank", standing: true }, open: { power: "contact" } }}
      result={<PowerResult who={JORAN} p={MINDLINK} on={GUARD.name} d20={9} cost={0} kept />}
      kept={JORAN_KEPT}
      journal={[["9", tr("Lien spirituel sur Gardien : réussi, maintenu", "Mindlink on Guard: success, kept up")]]}
    />
  );
}

/** A variable cost: the player types the cost (and the maintenance when it is variable too). */
function ActVariable() {
  return (
    <Act
      scene={{ who: JORAN, round: 3, psp: 71, minds: [ORC, GUARD], mind: ORC, defence: { id: "mind_blank", standing: true }, open: { power: "telekinesis" } }}
      result={<PowerResult who={JORAN} p={MINDLINK} on={GUARD.name} d20={9} cost={0} kept />}
      kept={JORAN_KEPT}
      journal={[["9", tr("Lien spirituel sur Gardien : réussi, maintenu", "Mindlink on Guard: success, kept up")]]}
    />
  );
}

/** Out of combat: a contact-cost power on an ally contacted earlier; paid defences wait for a round. */
function ActCalm() {
  const kept = [contactRow(BRANN), keptOf(power("mind_blank"))];
  return (
    <Act
      scene={{ who: JORAN, round: 0, psp: 96, minds: [BRANN], mind: BRANN, defence: { id: "mind_blank", standing: true }, open: { power: "mindlink" } }}
      result={<NoResult />}
      kept={kept}
      journal={[["16", tr("Contact sur Brann : réussi, 3 PFP, maintenu", "Contact on Brann: success, 3 PSP, kept up")]]}
    />
  );
}

/** The second fixture: no attack mode, no power that takes a mind, other disciplines. Same code. */
const YSOLDE_KEPT: Kept[] = [keptOf(power("biofeedback")), keptOf(power("flesh_armor"))];
function ActOther() {
  return (
    <Act
      scene={{ who: YSOLDE, round: 2, psp: 38, minds: [], defence: { id: "intellect_fortress" }, usedThisRound: "flesh_armor" }}
      result={<PowerResult who={YSOLDE} p={power("flesh_armor")} d20={6} cost={8} kept />}
      kept={YSOLDE_KEPT}
      journal={[
        ["6", tr("Armure charnelle : réussi, 8 PFP, maintenu", "Flesh Armor: success, 8 PSP, kept up")],
        ["", tr("Forteresse intellectuelle dressée, 4 PFP", "Intellect Fortress raised, 4 PSP")],
        ["", tr("Tour 2 : maintien payé, 3 PFP", "Round 2: maintenance paid, 3 PSP")],
      ]}
      last={<><b className="text-key">6</b> {lt(power("flesh_armor").name)} · <span className="text-success">{tr("réussi", "success")}</span></>}
    />
  );
}

/* ---------- rest ---------- */

/** Rest: Table 6's rows (a row gated by a proficiency shows when it is known), hours, the preview.
 * While a row that costs PSPs is kept up, recovery is refused (ADR 0043 step 6): the popover lists
 * them and offers to stop them in the same action; a row that costs nothing stays up. */
function RestPopover({ who, now, kept, activity }: { who: Fixture; now: number; kept: Kept[]; activity: string }) {
  const rows = PSP_RECOVERY.filter((r) => !r.requires_proficiency || who.proficiencies.includes(r.requires_proficiency));
  const hours = 3;
  const rate = rows.find((r) => r.activity === activity)!.per_hour;
  const gain = Math.min(hours * rate, who.pspMax - now);
  const paying = kept.filter((r) => r.cost > 0);
  const free = kept.filter((r) => r.cost === 0);
  const sleep = PSP_RECOVERY.find((r) => r.activity === "sleeping")!.per_hour * 8;
  return (
    <div className="zone-card absolute top-[17.5rem] left-[clamp(292px,calc(21vw+12px),396px)] z-10 flex w-[30rem] flex-col gap-3 p-4 shadow-xl">
      <div className="flex items-center justify-between">
        <span className="section-label">{tr("Récupérer des PFP", "Recover PSPs")}</span>
        <X className="size-4 text-ink-muted" aria-hidden />
      </div>
      <div className="grid auto-cols-fr grid-flow-col gap-1.5">
        {rows.map((r) => (
          <span
            key={r.activity}
            className={cn(
              "flex flex-col items-start rounded-md px-2.5 py-1.5",
              r.activity === activity ? "chip chip-chosen" : "chip",
            )}
          >
            <span className="text-body">{lt(ACTIVITY[r.activity])}</span>
            <span className="text-label text-ink-muted">{tr(`${r.per_hour} / heure`, `${r.per_hour} / hour`)}</span>
          </span>
        ))}
      </div>
      <div className="flex items-center gap-3">
        <Stepper value={`${hours} h`} w="w-14" />
        {[1, 4, 8].map((h) => (
          <span key={h} className="chip flex h-7 items-center rounded-md px-2.5 text-body">
            {h} h
          </span>
        ))}
        <span className="ml-auto text-body">
          <b className="tabular-nums">+{gain}</b> · {now} → <b className="tabular-nums">{now + gain}</b>
        </span>
      </div>
      {paying.length > 0 && (
        <div className="flex flex-col gap-1 rounded-md border border-warning/60 px-3 py-2">
          <span className="flex items-center gap-2 text-body text-warning">
            <Warning weight="fill" className="size-4" aria-hidden />
            {tr("On ne récupère que sans rien dépenser. Ceux-ci s'arrêtent :", "PSPs come back only while nothing is spent. These stop:")}
          </span>
          {paying.map((r) => (
            <span key={r.label + r.target} className="text-body">
              {r.label}
              {r.target ? ` · ${r.target}` : ""} <span className="text-ink-muted">({psp(r.cost)})</span>
            </span>
          ))}
          {free.length > 0 && (
            <span className="text-label text-ink-muted">
              {tr(`Reste levé, gratuit : ${free.map((r) => r.label).join(", ")}`, `Stays up, free: ${free.map((r) => r.label).join(", ")}`)}
            </span>
          )}
        </div>
      )}
      <Button variant="main">
        <Moon aria-hidden />
        {paying.length > 0
          ? paying.length === 1
            ? tr(`Arrêter ${paying[0].label} et récupérer ${hours} h`, `Stop ${paying[0].label} and recover ${hours} h`)
            : tr(`Arrêter ${paying.length} pouvoirs et récupérer ${hours} h`, `Stop ${paying.length} powers and recover ${hours} h`)
          : tr(`Récupérer ${hours} h`, `Recover ${hours} h`)}
      </Button>
      <div className="flex items-center justify-between gap-3 border-t border-border pt-3">
        <span className="text-label text-ink-muted">
          {tr(
            `Repos long : 8 h de sommeil, +${sleep} PFP (au plus le maximum), sorts et pouvoirs maintenus remis à zéro.`,
            `Long rest: 8 h of sleep, +${sleep} PSP (up to the maximum), spells and kept-up powers reset.`,
          )}
        </span>
        <Button variant="quiet">{tr("Repos long", "Long rest")}</Button>
      </div>
    </div>
  );
}

function Rest({ who, scene, kept, activity }: { who: Fixture; scene: Scene; kept: Kept[]; activity: string }) {
  return (
    <Act
      scene={scene}
      result={<NoResult />}
      kept={kept}
      journal={[]}
      restOpen
      overlay={<RestPopover who={who} now={scene.psp} kept={kept} activity={activity} />}
    />
  );
}

/** Joran (no Rejuvenation): four rows; a paying power kept up. */
function RestJoran() {
  const kept = [keptOf(MINDLINK, BRANN.name), contactRow(BRANN, tr("gratuit tant qu'un pouvoir l'utilise", "free while a power uses it")), keptOf(power("mind_blank"))];
  return <Rest who={JORAN} scene={{ who: JORAN, round: 0, psp: 71, minds: [BRANN], mind: BRANN, defence: { id: "mind_blank", standing: true } }} kept={kept} activity="resting" />;
}

/** Ysolde (Rejuvenation known): the proficiency's row shows; nothing kept up. */
function RestYsolde() {
  return <Rest who={YSOLDE} scene={{ who: YSOLDE, round: 0, psp: 38, minds: [] }} kept={[]} activity="rejuvenating" />;
}

/* ---------- browsing: known powers under Spells ---------- */

function LookupPowers() {
  const who = JORAN;
  const KIND_WORD: Record<Power["kind"], string> = {
    science: tr("Science", "Science"),
    devotion: tr("Dévotion", "Devotion"),
    defence_mode: tr("Mode de défense", "Defence mode"),
    attack_mode: tr("Mode d'attaque", "Attack mode"),
  };
  const list = CATALOGUE.filter((p) => p.discipline === "telepathy");
  const known = knownPowers(who);
  const count = (k: Power["kind"]) => known.filter((p) => p.kind === k).length;
  return (
    <Dashboard activity="spells" who={who} effects={[]} upkeep={0} round={0}>
      <div className="grid min-h-0 flex-1 grid-cols-[1.3fr_1fr] gap-3">
        <section className="stage-card flex min-h-0 min-w-0 flex-col">
          <div className="card-band flex items-center gap-1 px-3">
            {[tr("Sorts", "Spells"), tr("Pouvoirs", "Powers")].map((t, i) => (
              <span key={t} className={cn("px-3 py-2 text-body", i === 1 ? "font-medium shadow-[inset_0_-2px_0_var(--tint)]" : "text-ink-muted")}>
                {t}
              </span>
            ))}
          </div>
          <div className="flex min-h-0 flex-col gap-3 overflow-y-auto p-3">
            <span className="flex h-8 items-center gap-2 rounded-lg border border-input px-3 text-body text-ink-muted">
              <MagnifyingGlass className="size-4" aria-hidden />
              {tr("Rechercher des pouvoirs...", "Search powers...")}
            </span>
            <div className="flex flex-wrap gap-1.5">
              {(Object.keys(DISCIPLINES) as Discipline[]).map((d) => (
                <span key={d} className={cn("rounded-md px-2.5 py-1 text-body", d === "telepathy" ? "chip chip-chosen" : "chip")}>
                  {lt(DISCIPLINES[d])}
                </span>
              ))}
            </div>
            <ul className="flex flex-col gap-0.5">
              {list.map((p) => (
                <li key={p.id} className="grid grid-cols-[1fr_auto] items-center gap-3 rounded-md px-2.5 py-1 odd:bg-raised/50">
                  <span className="flex flex-col">
                    <span className="text-body font-medium">{lt(p.name)}</span>
                    <span className="text-label text-ink-muted">
                      {[KIND_WORD[p.kind], costText(p), upkeepText(p), `${tr("score", "score")} ${scoreOf(who, p)}`].filter(Boolean).join(" · ")}
                    </span>
                  </span>
                  {who.known.includes(p.id) ? (
                    <span className="flex items-center gap-1 text-label text-success">
                      <Check className="size-3.5" aria-hidden />
                      {tr("Connu", "Known")}
                    </span>
                  ) : (
                    <Button variant="quiet" size="sm">
                      <Plus aria-hidden />
                      {tr("Ajouter", "Add")}
                    </Button>
                  )}
                </li>
              ))}
            </ul>
          </div>
        </section>
        <section className="zone-card flex min-h-0 min-w-0 flex-col">
          <div className="card-band flex min-h-9 items-center px-4 py-2">
            <h2 className="section-label">{tr("Pouvoirs connus", "Known powers")}</h2>
          </div>
          <div className="flex min-h-0 flex-col gap-3 overflow-y-auto p-3">
            <div className="grid grid-cols-4 gap-2">
              {[
                [tr("Disciplines", "Disciplines"), `3 / 3`],
                [tr("Sciences", "Sciences"), `${count("science")} / 4`],
                [tr("Dévotions", "Devotions"), `${count("devotion")} / 10`],
                [tr("Modes de défense", "Defence modes"), `${count("defence_mode")} / 3`],
              ].map(([k, v]) => (
                <span key={k} className="flex flex-col rounded-md bg-raised px-3 py-1.5">
                  <span className="text-label text-ink-muted">{k}</span>
                  <span className="text-body font-semibold tabular-nums">{v}</span>
                </span>
              ))}
            </div>
            <ul className="flex flex-col gap-0.5">
              {known.map((p) => (
                <li key={p.id} className="grid grid-cols-[1fr_auto_auto] items-center gap-2 rounded-md px-2.5 py-1 odd:bg-raised/50">
                  <span className="flex flex-col">
                    <span className="text-body">{lt(p.name)}</span>
                    <span className="text-label text-ink-muted">
                      {lt(DISCIPLINES[p.discipline])} · {KIND_WORD[p.kind]}
                    </span>
                  </span>
                  {p.kind === "science" || p.kind === "devotion" ? (
                    <Button variant="quiet" size="sm">
                      {tr("Réapprendre", "Relearn")}
                    </Button>
                  ) : (
                    <span />
                  )}
                  <Button variant="ghost" size="icon-sm" aria-label={tr("Retirer", "Remove")}>
                    <X aria-hidden />
                  </Button>
                </li>
              ))}
            </ul>
          </div>
        </section>
      </div>
    </Dashboard>
  );
}


export const SCREENS: Record<string, () => ReactNode> = {
  "act-attack": ActAttack,
  "act-attack-result": ActAttackResult,
  "act-defend": ActDefend,
  "act-table-cost": ActTableCost,
  "act-variable": ActVariable,
  "act-calm": ActCalm,
  "act-other": ActOther,
  "rest-kept": RestJoran,
  "rest-proficiency": RestYsolde,
  "lookup-powers": LookupPowers,
};
