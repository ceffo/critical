// The screens of the psionics design round (docs/design/psionics/README.md), one per id:
// `?screen=<id>`. Each is the whole dashboard with the part the round decides.
import type { ReactNode } from "react";
import {
  ArrowRight,
  Brain,
  CaretDown,
  Check,
  Link as LinkIcon,
  MagnifyingGlass,
  Minus,
  Moon,
  Plus,
  ShieldCheck,
  Stop,
  Warning,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { lt, tr } from "./lang";
import { Dashboard, type Effect } from "./frame";
import { CHAR, POWERS, power, type Power } from "./data";

/* ---------- small controls ---------- */

function Seg({ options, chosen }: { options: ReactNode[]; chosen: number }) {
  return (
    <div className="flex flex-wrap gap-1">
      {options.map((o, i) => (
        <span
          key={i}
          className={cn(
            "flex min-h-7 items-center justify-center gap-1 rounded-md px-2.5 text-body [&_svg]:size-4",
            i === chosen ? "chip chip-chosen" : "chip",
          )}
        >
          {o}
        </span>
      ))}
    </div>
  );
}

function Stepper({ value }: { value: string }) {
  return (
    <span className="flex items-center gap-1">
      <span className="flex size-7 items-center justify-center rounded-md border border-input">
        <Minus className="size-4" aria-hidden />
      </span>
      <span className="flex h-7 w-12 items-center justify-center rounded-md border border-input text-body tabular-nums">
        {value}
      </span>
      <span className="flex size-7 items-center justify-center rounded-md border border-input">
        <Plus className="size-4" aria-hidden />
      </span>
    </span>
  );
}

function Select({ value, wide }: { value: string; wide?: boolean }) {
  return (
    <span
      className={cn(
        "flex h-7 items-center justify-between gap-2 rounded-lg border border-input bg-surface px-2 text-body",
        wide && "min-w-40",
      )}
    >
      {value}
      <CaretDown className="size-3.5 text-ink-muted" aria-hidden />
    </span>
  );
}

function Pips({ n, of = 3 }: { n: number; of?: number }) {
  return (
    <span className="flex gap-1" aria-label={`${n}/${of}`}>
      {Array.from({ length: of }, (_, i) => (
        <span
          key={i}
          className={cn(
            "size-2.5 rounded-full border border-tint",
            i < n && "bg-tint",
          )}
        />
      ))}
    </span>
  );
}

/* ---------- the opponent (ADR 0044 step 2): what the DM tells ---------- */

interface Target {
  name: string;
  mind: "open" | "closed";
  level: number;
  defence?: Power["id"];
  /** Tangents held (1 or 2), or "contact" for a full contact, or none. */
  held?: 1 | 2 | "contact";
  /** Contact made in an earlier round: a contact-cost power may be used on it. */
  earlier?: boolean;
}

const ORC: Target = {
  name: tr("Orc chaman", "Orc shaman"),
  mind: "closed",
  level: 6,
  defence: "thought_shield",
  held: 2,
};
const GUARD: Target = {
  name: tr("Gardien", "Guard"),
  mind: "open",
  level: 7,
  held: "contact",
  earlier: true,
};

function heldLabel(t: Target) {
  if (t.held === "contact") return tr("contact", "contact");
  if (t.held) return tr(`${t.held} tangente${t.held > 1 ? "s" : ""}`, `${t.held} tangent${t.held > 1 ? "s" : ""}`);
  return "";
}

/**
 * The target bar of the Powers tab, the weapons tab's target bar's counterpart: the minds held (a
 * tangent or contact row) as chips to switch between, then the chosen mind's open / closed, level
 * and raised defence, the three things the DM tells. Attacks and contact take their opponent here.
 */
function TargetBar({
  chosen,
  all,
}: {
  chosen: Target;
  all: Target[];
}) {
  return (
    <div className="flex flex-col gap-2 rounded-lg bg-raised px-3 py-2">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="section-label">{tr("Cible", "Target")}</span>
        {all.map((t) => (
          <span
            key={t.name}
            className={cn(
              "flex min-h-7 items-center gap-2 rounded-md px-2.5 text-body",
              t.name === chosen.name ? "chip chip-chosen" : "chip",
            )}
          >
            {t.name}
            {t.held === "contact" || t.held === undefined ? null : (
              <Pips n={t.held} />
            )}
            {t.held && (
              <span className="text-label text-ink-muted">{heldLabel(t)}</span>
            )}
          </span>
        ))}
        <span className="chip flex min-h-7 items-center gap-1 px-2.5 text-body">
          <Plus className="size-4" aria-hidden />
          {tr("Nouvelle", "New")}
        </span>
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="text-body text-ink-muted">{tr("Esprit", "Mind")}</span>
        <Seg
          options={[tr("Ouvert", "Open"), tr("Fermé", "Closed")]}
          chosen={chosen.mind === "open" ? 0 : 1}
        />
        <span className="text-body text-ink-muted">{tr("Niveau", "Level")}</span>
        <Stepper value={String(chosen.level)} />
        <span className="text-body text-ink-muted">{tr("Défense", "Defence")}</span>
        <Select
          wide
          value={
            chosen.defence
              ? lt(power(chosen.defence).name)
              : tr("Aucune", "None")
          }
        />
      </div>
    </div>
  );
}

/* ---------- the defence of the round ---------- */

/**
 * The defence row: what protects the character now (a mode raised this turn over a standing one),
 * the known modes to raise (one tap, no roll: ADR 0044 step 4) and the way to resolve an attack the DM
 * makes on the character.
 */
function DefenceRow({
  raised,
  incoming,
}: {
  raised?: Power["id"];
  incoming?: boolean;
}) {
  const modes = POWERS.filter((p) => p.kind === "defence");
  const current = raised ?? "mind_blank";
  return (
    <div className="flex flex-col gap-2 rounded-lg bg-raised px-3 py-2">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span className="section-label">{tr("Ma défense", "My defence")}</span>
        {modes.map((m) => {
          const standing = m.cost === 0;
          const on = m.id === current;
          return (
            <span
              key={m.id}
              className={cn(
                "flex min-h-7 items-center gap-2 rounded-md px-2.5 text-body",
                on ? "chip chip-chosen" : "chip",
              )}
            >
              <ShieldCheck weight={on ? "fill" : "regular"} aria-hidden className="size-4" />
              {lt(m.name)}
              <span className="text-label text-ink-muted">
                {on
                  ? standing
                    ? tr("permanente", "standing")
                    : tr("dressée ce tour", "raised this turn")
                  : standing
                    ? tr("0 PFP", "0 PSP")
                    : tr(`${m.cost} PFP`, `${m.cost} PSP`)}
              </span>
            </span>
          );
        })}
        <span className="flex-1" />
        <Button variant="quiet" aria-expanded={incoming}>
          {tr("Attaque subie…", "Incoming attack…")}
        </Button>
      </div>
      {incoming && (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-border pt-2">
          <span className="text-body text-ink-muted">
            {tr("Le MD attaque avec", "The DM attacks with")}
          </span>
          <Select wide value={lt(power("mind_thrust").name)} />
          <span className="text-body text-ink-muted">
            {tr("Jets du MD", "DM's rolls")}
          </span>
          {[
            ["12", true],
            ["", false],
          ].map(([v, ok], i) => (
            <span key={i} className="flex items-center gap-1.5">
              <span className="flex h-7 w-11 items-center justify-center rounded-md border border-input text-body tabular-nums">
                {v}
              </span>
              <span
                className={cn(
                  "flex size-7 items-center justify-center rounded-md border border-input",
                  ok && "border-tint bg-selected",
                )}
              >
                <Check className="size-4" aria-hidden />
              </span>
            </span>
          ))}
          <span className="flex-1" />
          <Button variant="main">{tr("Résoudre", "Resolve")}</Button>
        </div>
      )}
    </div>
  );
}

/* ---------- the powers ---------- */

function costText(p: Power) {
  if (p.cost === "contact") return tr("au contact", "at contact");
  if (p.cost === "variable") return tr("variable", "variable");
  return tr(`${p.cost} PFP`, `${p.cost} PSP`);
}

function PowerRow({
  p,
  verb,
  adjust,
  disabled,
  hint,
  expanded,
  children,
}: {
  p: Power;
  verb: string;
  /** The score's adjustment against the target's defence (Table 14), for an attack. */
  adjust?: number;
  disabled?: boolean;
  hint?: string;
  expanded?: boolean;
  children?: ReactNode;
}) {
  const score = p.score + (adjust ?? 0);
  return (
    <li className="flex flex-col gap-2 rounded-lg px-3 py-1.5 odd:bg-raised/60">
      <div className="grid grid-cols-[1fr_auto_auto] items-center gap-3">
        <div className="flex min-w-0 flex-col">
          <span className="text-body font-medium">{lt(p.name)}</span>
          <span className="text-label text-ink-muted">
            {lt(p.discipline)} · {costText(p)}
            {p.upkeep
              ? tr(` · ${p.upkeep} PFP par tour`, ` · ${p.upkeep} PSP a turn`)
              : ""}
            {hint ? ` · ${hint}` : ""}
          </span>
        </div>
        <span className="flex flex-col items-end">
          <span className="text-label text-ink-muted">
            {tr("Score", "Score")}
          </span>
          <span className="text-body font-semibold tabular-nums">
            {score}
            {adjust ? (
              <span className="ml-1 text-label font-normal text-ink-muted">
                ({p.score}
                {adjust > 0 ? "+" : "−"}
                {Math.abs(adjust)})
              </span>
            ) : null}
          </span>
        </span>
        <Button variant="quiet" disabled={disabled} aria-expanded={expanded}>
          {verb}
        </Button>
      </div>
      {children}
    </li>
  );
}

function GroupHead({ title, note }: { title: string; note?: string }) {
  return (
    <div className="flex items-baseline gap-2 px-3 pt-1">
      <span className="section-label">{title}</span>
      {note && <span className="text-label text-ink-muted">{note}</span>}
    </div>
  );
}

/**
 * An attack's inline form (ADR 0044 step 7): only when the target raised a defence, the DM's roll
 * and whether it succeeded for each of the two contests. Pressing Attaquer opens it; Lancer rolls.
 */
function AttackForm() {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border border-border bg-surface px-3 py-2">
      <span className="text-body text-ink-muted">
        {tr("Jets du MD contre sa défense", "DM's rolls against its defence")}
      </span>
      {[
        ["8", true],
        ["", false],
      ].map(([v, ok], i) => (
        <span key={i} className="flex items-center gap-1.5">
          <span className="text-label text-ink-muted">{i + 1}</span>
          <span className="flex h-7 w-11 items-center justify-center rounded-md border border-input text-body tabular-nums">
            {v}
          </span>
          <span
            className={cn(
              "flex size-7 items-center justify-center rounded-md border border-input",
              ok && "border-tint bg-selected",
            )}
            title={tr("Réussi", "Succeeded")}
          >
            <Check className="size-4" aria-hidden />
          </span>
        </span>
      ))}
      <span className="flex-1" />
      <Button variant="main">{tr("Lancer · 4 PFP", "Roll · 4 PSP")}</Button>
    </div>
  );
}

/** The contact power's inline form (ADR 0044 step 9): the cost by level and the three modifiers. */
function ContactForm() {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border border-border bg-surface px-3 py-2">
      <span className="text-body text-ink-muted">{tr("Distance", "Distance")}</span>
      <Select value={tr("10 milles", "10 miles")} />
      <span className="text-body text-ink-muted">{tr("Ordre de vie", "Life order")}</span>
      <Select value={tr("Mammifère", "Mammal")} />
      <span className="chip flex min-h-7 items-center gap-1.5 px-2.5 text-body chip-chosen">
        <Check className="size-4" aria-hidden />
        {tr("Résiste", "Resists")}
      </span>
      <span className="basis-full text-label text-ink-muted">
        {tr(
          "Coût 3 PFP (niveau 4, tableau 29) · score 17 −2 distance −0 vie −2 résiste = 13",
          "Cost 3 PSP (level 4, table 29) · score 17 −2 distance −0 life −2 resists = 13",
        )}
      </span>
    </div>
  );
}

function PowersTab({
  target,
  targets,
  raised,
  incoming,
  contact,
  attackOpen,
}: {
  target: Target;
  targets: Target[];
  raised?: Power["id"];
  incoming?: boolean;
  contact?: boolean;
  /** The id of the attack whose inline form is open. */
  attackOpen?: Power["id"];
}) {
  const adjust = target.defence ? -2 : 0;
  const noContact = !target.earlier;
  return (
    <div className="flex min-h-0 flex-col gap-3 overflow-y-auto p-4">
      <TargetBar chosen={target} all={targets} />
      <DefenceRow raised={raised} incoming={incoming} />
      <ul className="flex flex-col gap-0.5">
        <GroupHead
          title={tr("Attaque", "Attack")}
          note={
            target.defence
              ? tr(
                  `contre ${lt(power(target.defence).name)} : −2 au score (tableau 14)`,
                  `against ${lt(power(target.defence).name)}: −2 to the score (table 14)`,
                )
              : tr("aucune défense : pas de modificateur", "no defence: no modifier")
          }
        />
        {POWERS.filter((p) => p.kind === "attack").map((p) => (
          <PowerRow
            key={p.id}
            p={p}
            verb={tr("Attaquer", "Attack")}
            adjust={adjust}
            expanded={attackOpen === p.id}
          >
            {attackOpen === p.id && <AttackForm />}
          </PowerRow>
        ))}
        <GroupHead title={tr("Pouvoirs", "Powers")} />
        <PowerRow
          p={power("contact")}
          verb={tr("Contacter", "Contact")}
          expanded={contact}
        >
          {contact && <ContactForm />}
        </PowerRow>
        <PowerRow
          p={power("mind_link")}
          verb={tr("Utiliser", "Use")}
          disabled={noContact}
          hint={
            noContact
              ? tr("contact d'un tour précédent requis", "contact from an earlier turn needed")
              : tr(`sur ${target.name}`, `on ${target.name}`)
          }
        />
        {POWERS.filter(
          (p) => (p.kind === "science" || p.kind === "devotion") && p.id !== "contact" && p.id !== "mind_link",
        ).map((p) => (
          <PowerRow key={p.id} p={p} verb={tr("Utiliser", "Use")} />
        ))}
      </ul>
    </div>
  );
}

/* ---------- the result card and what is under way ---------- */

function Contest({
  n,
  ours,
  score,
  theirs,
  won,
  skipped,
}: {
  n: number;
  ours?: string;
  score?: number;
  theirs?: string;
  won?: boolean;
  skipped?: string;
}) {
  return (
    <div
      className={cn(
        "grid grid-cols-[auto_1fr_auto] items-center gap-3 rounded-lg px-3 py-2",
        skipped ? "border border-dashed border-border" : "bg-raised",
      )}
    >
      <span className="text-label text-ink-muted">
        {tr(`Lutte ${n}`, `Contest ${n}`)}
      </span>
      {skipped ? (
        <span className="text-body text-ink-muted">{skipped}</span>
      ) : (
        <span className="text-body">
          {tr("Votre jet", "Your roll")} <b className="tabular-nums">{ours}</b>{" "}
          <span className="text-ink-muted">
            ({tr("score", "score")} {score})
          </span>{" "}
          · {tr("MD", "DM")} <b className="tabular-nums">{theirs}</b>
        </span>
      )}
      {!skipped && (
        <span
          className={cn(
            "text-body font-semibold",
            won ? "text-success" : "text-danger",
          )}
        >
          {won ? tr("Gagnée", "Won") : tr("Perdue", "Lost")}
        </span>
      )}
    </div>
  );
}

function ResultFrame({
  badge,
  title,
  children,
}: {
  badge: string;
  title: string;
  children: ReactNode;
}) {
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

function Headline({
  total,
  verdict,
  good,
  sub,
}: {
  total: string;
  verdict: string;
  good: boolean;
  sub: string;
}) {
  return (
    <div className="flex items-center gap-3">
      <span className="flex size-11 items-center justify-center rounded-lg bg-tint text-key font-bold text-on-accent">
        {total}
      </span>
      <span className="flex flex-col">
        <span className={cn("text-key", good ? "text-success" : "text-danger")}>
          {verdict}
        </span>
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

function PowerResult() {
  return (
    <ResultFrame
      badge={tr("Pouvoir", "Power")}
      title={`${lt(power("mind_link").name)} · ${tr("Gardien", "Guard")}`}
    >
      <Headline
        total="9"
        verdict={tr("Réussi", "Success")}
        good
        sub={tr("jet 9 contre score 12 · entretenu", "roll 9 vs score 12 · maintained")}
      />
      <Facts
        rows={[
          ["d20", "9"],
          [tr("Score", "Score"), "12"],
          [tr("Coût", "Cost"), tr("8 PFP", "8 PSP")],
        ]}
      />
    </ResultFrame>
  );
}

function AttackResult() {
  return (
    <ResultFrame
      badge={tr("Attaque", "Attack")}
      title={`${lt(power("ego_whip").name)} · ${ORC.name}`}
    >
      <Headline
        total="3"
        verdict={tr("Contact établi", "Contact made")}
        good
        sub={tr("troisième tangente · 4 PFP", "third tangent · 4 PSP")}
      />
      <Contest n={1} ours="11" score={12} theirs="8" won />
      <Contest
        n={2}
        skipped={tr(
          "Non lancée : le contact est fait, plus d'attaque ce tour.",
          "Not rolled: contact is made, no more attacks this turn.",
        )}
      />
      <div className="flex items-center gap-3 rounded-lg bg-raised px-3 py-2 text-body">
        <span className="text-ink-muted">{tr("Tangentes", "Tangents")}</span>
        <Pips n={3} />
        <ArrowRight className="size-4 text-ink-muted" aria-hidden />
        <b>{tr("contact, 1 PFP par tour", "contact, 1 PSP a turn")}</b>
      </div>
    </ResultFrame>
  );
}

function DefendResult() {
  return (
    <ResultFrame
      badge={tr("Défense", "Defence")}
      title={`${lt(power("thought_shield").name)} ${tr("contre", "against")} ${lt(power("mind_thrust").name)}`}
    >
      <Headline
        total="9"
        verdict={tr("Défense battue", "Defence beaten")}
        good={false}
        sub={tr(
          "le MD compte la tangente, rien à noter ici",
          "the DM counts the tangent, nothing to track here",
        )}
      />
      <Contest n={1} ours="9" score={14} theirs="12" won={false} />
      <Contest
        n={2}
        skipped={tr("Pas de second jet du MD.", "No second roll from the DM.")}
      />
      <div className="flex items-center justify-between gap-3 rounded-lg bg-raised px-3 py-2">
        <span className="text-body">
          {tr(
            "Garder ce jet pour les autres lutte de ce tour ?",
            "Keep this roll for this turn's other contests?",
          )}
        </span>
        <Button variant="quiet">{tr("Garder", "Keep")}</Button>
      </div>
    </ResultFrame>
  );
}

interface UnderWayRow {
  name: string;
  target?: string;
  cost: number;
  note?: string;
  pips?: number;
  free?: boolean;
}

function UnderWayCard({ rows, psp }: { rows: UnderWayRow[]; psp: number }) {
  const total = rows.reduce((s, r) => s + r.cost, 0);
  const afford = psp - total >= 0;
  return (
    <section className="zone-card flex min-h-0 min-w-0 flex-col">
      <div className="card-band flex min-h-9 items-center justify-between gap-2 px-4 py-2">
        <h2 className="section-label">{tr("Entretenu", "Maintained")}</h2>
        <span className="text-body tabular-nums">
          {tr(`${total} PFP par tour`, `${total} PSP a turn`)}
        </span>
      </div>
      <ul className="flex flex-col gap-0.5 overflow-y-auto p-2">
        {rows.length === 0 && (
          <li className="px-2 py-1 text-body text-ink-muted">
            {tr("Rien n'est entretenu.", "Nothing is maintained.")}
          </li>
        )}
        {rows.map((r) => (
          <li
            key={r.name + (r.target ?? "")}
            className="grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-md px-2 py-1 odd:bg-raised/60"
          >
            <span className="flex min-w-0 flex-col">
              <span className="text-body">
                {r.name}
                {r.target ? ` · ${r.target}` : ""}
              </span>
              <span className="flex items-center gap-2 text-label text-ink-muted">
                {r.pips ? <Pips n={r.pips} /> : null}
                {r.note}
              </span>
            </span>
            <span className="text-body text-ink-muted tabular-nums">
              {r.free
                ? tr("gratuit", "free")
                : tr(`${r.cost} PFP`, `${r.cost} PSP`)}
            </span>
            <Button variant="quiet" aria-label={tr("Arrêter", "Stop")}>
              <Stop aria-hidden />
              {tr("Arrêter", "Stop")}
            </Button>
          </li>
        ))}
      </ul>
      <div
        className={cn(
          "mt-auto flex items-center gap-2 border-t border-border px-4 py-2 text-body",
          !afford && "text-danger",
        )}
      >
        {!afford && <Warning weight="fill" className="size-4" aria-hidden />}
        {afford
          ? tr(
              `Tour suivant : ${psp} → ${psp - total} PFP`,
              `Next turn: ${psp} → ${psp - total} PSP`,
            )
          : tr(
              `Tour suivant : il manque ${total - psp} PFP, un pouvoir s'arrête`,
              `Next turn: ${total - psp} PSP short, a power ends`,
            )}
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
          <li
            key={i}
            className="grid grid-cols-[3rem_1fr_auto] gap-3 rounded-md px-2 py-0.5 text-body"
          >
            <span className="text-right font-semibold text-highlight tabular-nums">
              {n}
            </span>
            <span>{t}</span>
            <span className="text-ink-muted tabular-nums">20:4{i}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ActTabs() {
  return (
    <div className="card-band flex items-center gap-1 px-3">
      {[
        tr("Armes", "Weapons"),
        tr("Pouvoirs", "Powers"),
        tr("Objets", "Items"),
        tr("Compétences", "Skills"),
      ].map((t, i) => (
        <span
          key={t}
          className={cn(
            "px-3 py-2 text-body",
            i === 1
              ? "font-medium shadow-[inset_0_-2px_0_var(--tint)]"
              : "text-ink-muted",
          )}
        >
          {t}
        </span>
      ))}
    </div>
  );
}

function ActStage({
  tab,
  result,
  underway,
  journal,
}: {
  tab: ReactNode;
  result: ReactNode;
  underway: ReactNode;
  journal: [string, string][];
}) {
  return (
    <>
      <div className="grid min-h-0 flex-[5] grid-cols-[minmax(28rem,1.5fr)_minmax(18rem,1fr)] gap-3">
        <section className="stage-card flex min-h-0 min-w-0 flex-col">
          <ActTabs />
          {tab}
        </section>
        <div className="flex min-h-0 min-w-0 flex-col gap-3">
          {result}
          {underway}
        </div>
      </div>
      <Journal rows={journal} />
    </>
  );
}

/* ---------- the table's scene: round 3, an orc shaman and a guard ---------- */

const LINK_ROW: UnderWayRow = {
  name: lt(power("mind_link").name),
  target: GUARD.name,
  cost: 8,
  note: tr("contact du Gardien gratuit", "Guard's contact is free"),
};
const BLANK_ROW: UnderWayRow = {
  name: lt(power("mind_blank").name),
  cost: 0,
  note: tr("permanent", "standing"),
};
const TANGENT_ROW: UnderWayRow = {
  name: tr("Tangentes", "Tangents"),
  target: ORC.name,
  cost: 1,
  pips: 2,
  note: tr("une seule cible", "one mind only"),
};
const CONTACT_ROW: UnderWayRow = {
  name: "Contact",
  target: ORC.name,
  cost: 1,
  pips: 3,
  note: tr("contact complet", "full contact"),
};

const FX_BASE: Effect[] = [
  { name: lt(power("mind_link").name) + " · " + GUARD.name, width: 100, note: tr("jusqu'à retrait", "until stopped") },
  { name: tr("Tangentes · Orc chaman", "Tangents · Orc shaman"), width: 100, note: tr("2 sur 3", "2 of 3") },
];

const J = (rows: [string, string][]): [string, string][] => rows;

function ActPowers() {
  return (
    <Dashboard
      activity="act"
      effects={FX_BASE}
      upkeep={9}
      last={
        <>
          <b className="text-key">9</b> {tr("Lien spirituel", "Mind link")} ·{" "}
          <span className="text-success">{tr("réussi", "success")}</span>
        </>
      }
    >
      <ActStage
        tab={<PowersTab target={ORC} targets={[ORC, GUARD]} attackOpen="ego_whip" />}
        result={<PowerResult />}
        underway={
          <UnderWayCard rows={[LINK_ROW, TANGENT_ROW, BLANK_ROW]} psp={71} />
        }
        journal={J([
          ["9", tr("Lien spirituel sur Gardien : réussi, 8 PFP, entretenu", "Mind link on Guard: success, 8 PSP, maintained")],
          ["", tr("Contact établi : Gardien", "Contact made: Guard")],
          ["", tr("Tour 3 : entretien payé, 9 PFP", "Turn 3: upkeep paid, 9 PSP")],
        ])}
      />
    </Dashboard>
  );
}

function ActAttack() {
  return (
    <Dashboard
      activity="act"
      psp={67}
      effects={[FX_BASE[0], { name: tr("Contact · Orc chaman", "Contact · Orc shaman"), width: 100, note: tr("jusqu'à retrait", "until stopped") }]}
      upkeep={9}
      last={
        <>
          <b className="text-key">11</b> {tr("Fouet de l'ego", "Ego whip")} ·{" "}
          <span className="text-success">{tr("contact", "contact")}</span>
        </>
      }
    >
      <ActStage
        tab={
          <PowersTab
            target={{ ...ORC, held: "contact" }}
            targets={[{ ...ORC, held: "contact" }, GUARD]}
          />
        }
        result={<AttackResult />}
        underway={
          <UnderWayCard rows={[LINK_ROW, CONTACT_ROW, BLANK_ROW]} psp={67} />
        }
        journal={J([
          ["11", tr("Fouet de l'ego sur Orc chaman : lutte 1 gagnée (11 contre 8), 3e tangente : contact, 4 PFP", "Ego whip on Orc shaman: contest 1 won (11 vs 8), 3rd tangent: contact, 4 PSP")],
          ["", tr("Lien spirituel sur Gardien : réussi, 8 PFP", "Mind link on Guard: success, 8 PSP")],
        ])}
      />
    </Dashboard>
  );
}

function ActDefend() {
  return (
    <Dashboard
      activity="act"
      psp={70}
      effects={[...FX_BASE, { name: lt(power("thought_shield").name), width: 20, note: tr("ce tour", "this turn") }]}
      upkeep={9}
      last={
        <>
          <b className="text-key">9</b> {tr("Bouclier de pensée", "Thought shield")} ·{" "}
          <span className="text-danger">{tr("battu", "beaten")}</span>
        </>
      }
    >
      <ActStage
        tab={
          <PowersTab
            target={ORC}
            targets={[ORC, GUARD]}
            raised="thought_shield"
            incoming
          />
        }
        result={<DefendResult />}
        underway={
          <UnderWayCard rows={[LINK_ROW, TANGENT_ROW, BLANK_ROW]} psp={70} />
        }
        journal={J([
          ["9", tr("Bouclier de pensée contre Pression mentale : lutte 1 perdue (9 contre 12)", "Thought shield vs Mind thrust: contest 1 lost (9 vs 12)")],
          ["", tr("Bouclier de pensée dressé, 1 PFP", "Thought shield raised, 1 PSP")],
        ])}
      />
    </Dashboard>
  );
}

function ActContact() {
  const priest: Target = {
    name: tr("Prêtre", "Priest"),
    mind: "open",
    level: 4,
  };
  return (
    <Dashboard
      activity="act"
      psp={71}
      effects={[FX_BASE[0]]}
      upkeep={8}
    >
      <ActStage
        tab={<PowersTab target={priest} targets={[priest, GUARD]} contact />}
        result={<PowerResult />}
        underway={<UnderWayCard rows={[LINK_ROW, BLANK_ROW]} psp={71} />}
        journal={J([
          ["9", tr("Lien spirituel sur Gardien : réussi, 8 PFP", "Mind link on Guard: success, 8 PSP")],
        ])}
      />
    </Dashboard>
  );
}

function ActSingle() {
  const adv: Target = {
    name: tr("Adversaire", "Opponent"),
    mind: "closed",
    level: 0,
  };
  return (
    <Dashboard activity="act" single effects={[]} upkeep={0} psp={109} round={0}>
      <ActStage
        tab={<PowersTab target={adv} targets={[adv]} />}
        result={
          <ResultFrame badge={tr("Pouvoir", "Power")} title={tr("Aucun", "None")}>
            <span className="text-body text-ink-muted">
              {tr(
                "Le résultat du dernier pouvoir apparaît ici.",
                "The last power's result appears here.",
              )}
            </span>
          </ResultFrame>
        }
        underway={<UnderWayCard rows={[]} psp={109} />}
        journal={J([])}
      />
    </Dashboard>
  );
}

/* ---------- resting ---------- */

function RestPopover({ blocked }: { blocked?: boolean }) {
  const acts = [
    tr("Effort", "Exertion"),
    tr("Marche", "Walking"),
    tr("Repos", "Resting"),
    tr("Sommeil", "Sleeping"),
    tr("Méditation", "Meditating"),
  ];
  return (
    <div
      className="fixed z-20 flex w-[27rem] flex-col gap-4 rounded-lg border border-border bg-popover p-4 shadow-2xl"
      style={{ left: "calc(clamp(280px, 21vw, 384px) + 28px)", top: 340 }}
    >
      <div className="flex flex-col gap-2">
        <span className="section-label">
          {tr("Récupérer des PFP", "Recover PSPs")}
        </span>
        <div className="flex flex-wrap items-center gap-3">
          <Stepper value="3" />
          <span className="text-body text-ink-muted">{tr("heures de", "hours of")}</span>
          <Seg options={acts} chosen={2} />
        </div>
        <span className="text-body">
          {tr("+18 PFP (6 par heure)", "+18 PSP (6 an hour)")} ·{" "}
          <b className="tabular-nums">71 → 89</b>
        </span>
        <span className="text-label text-ink-muted">
          {tr(
            "Méditation : 12 par heure, avec la compétence Régénération. Jamais plus que ce qui a été perdu.",
            "Meditating: 12 an hour, with the Rejuvenation proficiency. Never more than was lost.",
          )}
        </span>
        {blocked && (
          <span className="flex items-start gap-2 rounded-md bg-danger/15 px-3 py-2 text-body text-danger">
            <Warning weight="fill" className="mt-0.5 size-4 shrink-0" aria-hidden />
            {tr(
              "Lien spirituel est entretenu : on ne récupère que les heures sans dépense. Arrêtez-le d'abord.",
              "Mind link is maintained: you only recover in hours with no PSPs spent. Stop it first.",
            )}
          </span>
        )}
        <div className="flex gap-2">
          <Button variant="main" disabled={blocked}>
            <Moon aria-hidden />
            {tr("Récupérer", "Recover")}
          </Button>
          {blocked && (
            <Button variant="quiet">
              <Stop aria-hidden />
              {tr("Arrêter Lien spirituel", "Stop Mind link")}
            </Button>
          )}
        </div>
      </div>
      <div className="flex flex-col gap-2 border-t border-border pt-3">
        <span className="section-label">{tr("Repos long", "Long rest")}</span>
        <span className="text-body">
          {tr(
            "8 h de sommeil : +96 PFP (au plus ce qui manque), les pouvoirs entretenus prennent fin.",
            "8 h of sleep: +96 PSP (at most what is missing); maintained powers end.",
          )}
        </span>
        <Button variant="quiet" className="self-start">
          {tr("Repos long", "Long rest")}
        </Button>
      </div>
    </div>
  );
}

function ActRest({ blocked }: { blocked?: boolean }) {
  return (
    <Dashboard
      activity="act"
      effects={blocked ? [FX_BASE[0]] : []}
      upkeep={blocked ? 8 : 0}
      round={0}
      restOpen
      overlay={<RestPopover blocked={blocked} />}
    >
      <ActStage
        tab={
          <PowersTab
            target={{ name: tr("Adversaire", "Opponent"), mind: "closed", level: 0 }}
            targets={[{ name: tr("Adversaire", "Opponent"), mind: "closed", level: 0 }]}
          />
        }
        result={<PowerResult />}
        underway={
          <UnderWayCard rows={blocked ? [LINK_ROW, BLANK_ROW] : [BLANK_ROW]} psp={71} />
        }
        journal={J([])}
      />
    </Dashboard>
  );
}

/* ---------- browsing: Spells > Powers (the research side) ---------- */

function LookupPowers() {
  const disciplines = [
    tr("Clairsentience", "Clairsentience"),
    tr("Psychokinésie", "Psychokinesis"),
    tr("Psychométabolisme", "Psychometabolism"),
    tr("Psychotranslation", "Psychoportation"),
    tr("Télépathie", "Telepathy"),
    tr("Métapsionisme", "Metapsionics"),
  ];
  const counts: [string, string][] = [
    [tr("Disciplines", "Disciplines"), "3 / 4"],
    [tr("Sciences", "Sciences"), "3 / 5"],
    [tr("Dévotions", "Devotions"), "4 / 12"],
    [tr("Modes de défense", "Defence modes"), "3 / 3"],
  ];
  const known = POWERS.slice(0, 10);
  return (
    <Dashboard
      activity="spells"
      effects={FX_BASE}
      upkeep={9}
    >
      <div className="grid min-h-0 flex-1 grid-cols-[minmax(24rem,1fr)_minmax(24rem,1fr)] gap-3">
        <section className="stage-card flex min-h-0 min-w-0 flex-col">
          <div className="card-band flex items-center gap-1 px-3">
            {[tr("Sorts", "Spells"), tr("Pouvoirs", "Powers")].map((t, i) => (
              <span
                key={t}
                className={cn(
                  "px-3 py-2 text-body",
                  i === 1
                    ? "font-medium shadow-[inset_0_-2px_0_var(--tint)]"
                    : "text-ink-muted",
                )}
              >
                {t}
              </span>
            ))}
          </div>
          <div className="flex flex-col gap-3 p-4">
            <span className="flex h-8 items-center gap-2 rounded-lg border border-input bg-surface px-2 text-body text-ink-muted">
              <MagnifyingGlass className="size-4" aria-hidden />
              {tr("Rechercher des pouvoirs…", "Search powers…")}
            </span>
            <div className="flex flex-wrap gap-1">
              {disciplines.map((d, i) => (
                <span
                  key={d}
                  className={cn(
                    "px-2.5 py-1 text-body",
                    i === 4 ? "chip chip-chosen" : "chip",
                  )}
                >
                  {d}
                </span>
              ))}
            </div>
          </div>
          <ul className="flex min-h-0 flex-col gap-0.5 overflow-y-auto px-4 pb-4">
            {POWERS.filter((p) => p.discipline.en === "Telepathy").map((p) => {
              const have = known.some((k) => k.id === p.id);
              return (
                <li
                  key={p.id}
                  className="grid grid-cols-[1fr_auto] items-center gap-3 rounded-lg px-3 py-1.5 odd:bg-raised/60"
                >
                  <span className="flex flex-col">
                    <span className="text-body font-medium">{lt(p.name)}</span>
                    <span className="text-label text-ink-muted">
                      {costText(p)} · {tr("score", "score")} {p.score}
                    </span>
                  </span>
                  {have ? (
                    <span className="flex items-center gap-1 text-label text-success">
                      <Check className="size-4" aria-hidden />
                      {tr("Connu", "Known")}
                    </span>
                  ) : (
                    <Button variant="quiet">
                      <Plus aria-hidden />
                      {tr("Ajouter", "Add")}
                    </Button>
                  )}
                </li>
              );
            })}
          </ul>
        </section>
        <section className="zone-card flex min-h-0 min-w-0 flex-col">
          <div className="card-band flex min-h-9 items-center gap-3 px-4 py-2">
            <h2 className="section-label">
              {tr("Pouvoirs connus", "Known powers")}
            </h2>
          </div>
          <div className="grid grid-cols-4 gap-2 p-3">
            {counts.map(([k, v]) => (
              <span key={k} className="flex flex-col rounded-md bg-raised px-3 py-1.5">
                <span className="text-label text-ink-muted">{k}</span>
                <span className="text-body font-semibold tabular-nums">{v}</span>
              </span>
            ))}
          </div>
          <ul className="flex min-h-0 flex-col gap-0.5 overflow-y-auto px-3 pb-3">
            {known.map((p) => (
              <li
                key={p.id}
                className="grid grid-cols-[1fr_auto_auto] items-center gap-3 rounded-lg px-3 py-1.5 odd:bg-raised/60"
              >
                <span className="flex flex-col">
                  <span className="text-body font-medium">{lt(p.name)}</span>
                  <span className="text-label text-ink-muted">
                    {lt(p.discipline)}
                    {p.id === "mind_thrust" ? ` · ${tr("primaire", "primary")}` : ""}
                  </span>
                </span>
                {(p.kind === "science" || p.kind === "devotion") && (
                  <Button variant="quiet">{tr("Réapprendre", "Relearn")}</Button>
                )}
                <Button variant="quiet" size="icon" aria-label={tr("Retirer", "Remove")}>
                  <X aria-hidden />
                </Button>
              </li>
            ))}
          </ul>
        </section>
      </div>
    </Dashboard>
  );
}

export const SCREENS: Record<string, () => ReactNode> = {
  "act-powers": ActPowers,
  "act-attack": ActAttack,
  "act-defend": ActDefend,
  "act-contact": ActContact,
  "act-single": ActSingle,
  "rest": () => <ActRest />,
  "rest-blocked": () => <ActRest blocked />,
  "lookup-powers": LookupPowers,
};

void Brain;
void LinkIcon;
void CHAR;
