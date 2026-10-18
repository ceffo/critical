// The onboarding design round's screens (French first; `?lang=en` for the no-cut check in English).
import type { ReactNode } from "react";
import {
  Archive,
  ArrowLeft,
  ArrowRight,
  ArrowsLeftRight,
  Check,
  DiceFive,
  Desktop,
  Detective,
  GearSix,
  HandsPraying,
  Keyboard,
  Lock,
  MagicWand,
  Moon,
  MusicNotes,
  PencilSimple,
  Play,
  Prohibit,
  ShieldCheck,
  SignOut,
  Sun,
  Sword,
  UserPlus,
  Warning,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { cn } from "@/lib/utils";
import { tr } from "./lang";
import {
  ABILITIES,
  ALIGNMENTS,
  BARD_SKILLS,
  CHAR,
  CLASSES,
  HP_ROWS,
  LANGUAGES,
  METHODS,
  RACES,
  ROLLS,
  ROSTER,
  STEPS,
  total,
} from "./data";
import {
  CardHead,
  CreationFrame,
  Die,
  Field,
  Group,
  LangSwitch,
  Logout,
  Note,
  Seg,
  Wordmark,
} from "./kit";

const CLASS_ICONS: Record<string, ReactNode> = {
  bard: <MusicNotes aria-hidden />,
  fighter: <Sword aria-hidden />,
  mage: <MagicWand aria-hidden />,
  cleric: <HandsPraying aria-hidden />,
  thief: <Detective aria-hidden />,
};

/** A selectable card (class, race): the chosen tone with the tint outline, or plain. */
function Choice({
  on,
  off,
  children,
  className,
}: {
  on?: boolean;
  off?: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-1 rounded-lg p-3",
        on ? "chip-chosen" : "bg-raised",
        off && "opacity-60",
        className,
      )}
    >
      {children}
    </div>
  );
}

/** One fact of a class or race card: the label left, the value right of it. */
function Row({ k, v }: { k: string; v: ReactNode }) {
  return (
    <div className="grid grid-cols-[7.5rem_minmax(0,1fr)] items-baseline gap-2">
      <span className={labelClass}>{k}</span>
      <span className="text-body break-words">{v}</span>
    </div>
  );
}

// --- Intro ---------------------------------------------------------------------------------

export function Intro() {
  return (
    <div className="flex h-full items-center justify-center overflow-y-auto p-3">
      <section className="stage-card flex w-full max-w-[44rem] flex-col">
        <div className="flex flex-col gap-5 px-8 pt-8 pb-6">
          <Wordmark size="hero" />
          <h1 className="text-headline">
            {tr("Bienvenue à la table", "Welcome to the table")}
          </h1>
          <p className="text-title font-normal">
            {tr(
              "Critical! tient la fiche de votre personnage AD&D 2e pendant la partie : points de vie, sorts, effets en cours et jets de dés, chaque résultat avec son calcul.",
              "Critical! keeps your AD&D 2e character's sheet during play: hit points, spells, running effects and dice rolls, each result with its working.",
            )}
          </p>
          <div className="flex flex-col gap-2">
            <p className="text-body text-ink-muted">
              {tr(
                "Commencez par créer votre personnage, en six étapes :",
                "Start by creating your character, in six steps:",
              )}
            </p>
            <ol className="grid grid-cols-2 gap-x-6 gap-y-1.5">
              {STEPS.map((s, i) => (
                <li key={s.id} className="flex items-center gap-2 text-body">
                  <span className="flex size-6 shrink-0 items-center justify-center rounded-full text-label font-semibold text-tint shadow-[inset_0_0_0_1.5px_var(--tint)]">
                    {i + 1}
                  </span>
                  {tr(s.fr, s.en)}
                </li>
              ))}
            </ol>
          </div>
          <p className="text-body text-ink-muted">
            {tr(
              "À chaque valeur, tirez les dés ici ou recopiez votre fiche papier. Tout reste modifiable ensuite, et le MD a toujours le dernier mot.",
              "For each value, roll the dice here or copy your paper sheet. Everything stays editable later, and the DM always has the last word.",
            )}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3 border-t border-border px-8 py-4">
          <LangSwitch />
          <Logout />
          <span className="flex-1" />
          <Button variant="main" size="lg">
            {tr("Commencer", "Start")}
            <ArrowRight aria-hidden />
          </Button>
        </div>
      </section>
    </div>
  );
}

// --- Step 1: class ------------------------------------------------------------------------

export function StepClass() {
  return (
    <CreationFrame
      step={0}
      title={tr("Choisissez une classe", "Choose a class")}
      lead={tr(
        "La classe fixe le dé de vie, les talents et les alignements permis.",
        "The class sets the hit die, the skills and the alignments allowed.",
      )}
    >
      <div className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-3">
        {CLASSES.map((c) => (
          <Choice key={c.id} on={c.id === "bard"}>
            <div className="flex items-center gap-2 [&_svg]:size-5">
              {CLASS_ICONS[c.id]}
              <span className="text-key">{tr(c.fr, c.en)}</span>
              <span className="ml-auto text-label text-ink-muted">{tr(c.groupFr, c.groupEn)}</span>
            </div>
            <Row k={tr("Dé de vie", "Hit die")} v={c.hd} />
            <Row k={tr("Minimums", "Minimums")} v={tr(c.reqFr, c.reqEn)} />
            <Row k={tr("Alignement", "Alignment")} v={tr(c.alFr, c.alEn)} />
            <Row k={tr("Races", "Races")} v={tr(c.racesFr, c.racesEn)} />
          </Choice>
        ))}
      </div>
      <Group title={tr("Expérience de départ", "Starting experience")}>
        <div className="flex flex-wrap items-end gap-6">
          <Field label={tr("XP de départ", "Starting XP")} value="2 500" />
          <div className="flex flex-col">
            <span className={labelClass}>{tr("Niveau", "Level")}</span>
            <span className="text-key">3</span>
          </div>
          <p className="max-w-[36rem] text-body text-ink-muted">
            {tr(
              "0 pour un personnage débutant. Le niveau, le TAC0 et les jets de protection en découlent.",
              "0 for a beginning character. The level, THAC0 and saving throws follow from it.",
            )}
          </p>
        </div>
      </Group>
    </CreationFrame>
  );
}

// --- Step 2: race -------------------------------------------------------------------------

export function StepRace() {
  return (
    <CreationFrame
      step={1}
      title={tr("Choisissez une race", "Choose a race")}
      lead={tr(
        "Seules les races permises au barde sont proposées ; les autres restent visibles.",
        "Only the races a bard allows can be picked; the others stay visible.",
      )}
    >
      <div className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-3">
        {RACES.map((r) => (
          <Choice key={r.id} on={r.id === "half_elf"} off={!r.ok}>
            <div className="flex items-center gap-2">
              <span className="text-key">{tr(r.fr, r.en)}</span>
              {!r.ok && (
                <span className="ml-auto flex items-center gap-1 text-label text-danger">
                  <Prohibit aria-hidden weight="bold" className="size-3.5" />
                  {tr("Pas barde", "No bard")}
                </span>
              )}
            </div>
            <Row k={tr("Ajustements", "Adjustments")} v={tr(r.adjFr, r.adjEn)} />
            <Row k={tr("Langue maternelle", "Native language")} v={tr(r.langFr, r.langEn)} />
            <Row k={tr("Taille", "Height")} v={tr(r.hFr, r.hEn)} />
            <Row k={tr("Âge de départ", "Starting age")} v={tr(r.ageFr, r.ageEn)} />
          </Choice>
        ))}
      </div>
    </CreationFrame>
  );
}

// --- Step 3: ability scores ---------------------------------------------------------------

export function StepScores() {
  return (
    <CreationFrame
      step={2}
      title={tr("Caractéristiques", "Ability scores")}
      lead={tr(
        "Tirez-les selon une méthode du Guide du maître, ou saisissez celles de votre fiche.",
        "Roll them by a Dungeon Master's Guide method, or type your sheet's.",
      )}
    >
      <div className="flex flex-wrap items-center gap-4">
        <Seg
          chosen={0}
          options={[
            <>
              <DiceFive aria-hidden />
              {tr("Tirer", "Roll")}
            </>,
            <>
              <Keyboard aria-hidden />
              {tr("Saisir", "Type")}
            </>,
          ]}
        />
      </div>
      <Group title={tr("Méthode", "Method")}>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(12rem,1fr))] gap-2">
          {METHODS.map((m) => (
            <div
              key={m.id}
              className={cn(
                "flex items-baseline gap-2 rounded-md px-3 py-2",
                m.id === "V" ? "chip-chosen" : "chip",
              )}
            >
              <span className="text-title">{m.id}</span>
              <span className="text-body">{tr(m.fr, m.en)}</span>
            </div>
          ))}
        </div>
      </Group>
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-5">
        <Group
          title={tr("Tirage · méthode V", "Roll · method V")}
          right={
            <Button variant="quiet" size="sm">
              <DiceFive aria-hidden />
              {tr("Relancer", "Reroll")}
            </Button>
          }
        >
          <ol className="flex flex-col gap-1.5">
            {ROLLS.map((r, i) => {
              const ab = ABILITIES.find((a) => a.id === r.to)!;
              return (
                <li key={i} className="flex items-center gap-2 rounded-md bg-raised px-2 py-1">
                  <span className="w-5 text-label text-ink-muted">{i + 1}</span>
                  <span className="flex gap-1">
                    {r.dice.map((d, j) => (
                      <Die key={j} v={d} dropped={j === r.drop} />
                    ))}
                  </span>
                  <span className="text-body text-ink-muted">=</span>
                  <span className="w-6 text-key tabular-nums">{total(r)}</span>
                  <ArrowRight aria-hidden className="size-3.5 text-ink-muted" />
                  <span className="text-body">{tr(ab.nameFr, ab.nameEn)}</span>
                </li>
              );
            })}
          </ol>
          <p className="text-label text-ink-muted">
            {tr(
              "Le dé barré est le plus bas des quatre, écarté. Cliquez un total puis une caractéristique pour l'y placer.",
              "The struck die is the lowest of the four, dropped. Click a total, then an ability, to place it.",
            )}
          </p>
        </Group>
        <Group title={tr("Répartition", "Arrangement")}>
          <table className="w-full text-body">
            <thead>
              <tr className="text-left text-label text-ink-muted">
                <th className="pb-1 font-medium">{tr("Caractéristique", "Ability")}</th>
                <th className="pb-1 font-medium">{tr("Tirage", "Roll")}</th>
                <th className="pb-1 font-medium">{tr("Race", "Race")}</th>
                <th className="pb-1 font-medium">{tr("Score", "Score")}</th>
                <th className="pb-1 font-medium">{tr("Barde", "Bard")}</th>
              </tr>
            </thead>
            <tbody>
              {ABILITIES.map((a) => (
                <tr key={a.id} className="border-t border-border">
                  <td className="py-1.5">{tr(a.nameFr, a.nameEn)}</td>
                  <td>
                    <span className="chip-chosen inline-flex h-6 min-w-8 items-center justify-center px-1.5 tabular-nums">
                      {a.score}
                    </span>
                  </td>
                  <td className="text-ink-muted">+0</td>
                  <td className="text-key tabular-nums">{a.score}</td>
                  <td>
                    {a.min > 0 && (
                      <span className="flex items-center gap-1 text-success">
                        <Check aria-hidden weight="bold" className="size-3.5" />
                        {a.min}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Group>
      </div>
    </CreationFrame>
  );
}

// --- Step 4: hit points and class skills -------------------------------------------------

export function StepHp() {
  const sum = HP_ROWS.reduce((s, r) => s + r.die, 0);
  const pts = BARD_SKILLS.reduce((s, r) => s + r.points, 0);
  return (
    <CreationFrame
      step={3}
      title={tr("Points de vie et talents", "Hit points and skills")}
      lead={tr(
        "Ce que la classe demande en plus : les PV du niveau 1 au niveau 3, et les points des talents de barde.",
        "What the class adds: the hit points from level 1 to level 3, and the bard skill points.",
      )}
    >
      <div className="grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-5">
        <Group
          title={tr("Points de vie", "Hit points")}
          right={
            <Button variant="quiet" size="sm">
              <DiceFive aria-hidden />
              {tr("Relancer", "Reroll")}
            </Button>
          }
        >
          <table className="w-full text-body">
            <thead>
              <tr className="text-left text-label text-ink-muted">
                <th className="pb-1 font-medium">{tr("Niveau", "Level")}</th>
                <th className="pb-1 font-medium">{tr("Dé", "Die")}</th>
                <th className="pb-1 font-medium">CON</th>
                <th className="pb-1 font-medium">{tr("Gain", "Gain")}</th>
              </tr>
            </thead>
            <tbody>
              {HP_ROWS.map((r) => (
                <tr key={r.level} className="border-t border-border">
                  <td className="py-1.5">{r.level}</td>
                  <td>
                    <span className="flex items-center gap-1.5">
                      <span className="text-ink-muted">d6</span>
                      <Die v={r.die} />
                    </span>
                  </td>
                  <td className="text-ink-muted">+0</td>
                  <td className="tabular-nums">+{r.die}</td>
                </tr>
              ))}
              <tr className="border-t border-border">
                <td className="py-1.5 font-medium" colSpan={3}>
                  {tr("Total", "Total")}
                </td>
                <td className="text-key tabular-nums">{sum}</td>
              </tr>
            </tbody>
          </table>
          <Field
            label={tr("PV max", "Max HP")}
            value={String(sum)}
            hint={tr("Le tirage remplit la case ; vous pouvez la changer.", "The roll fills the box; you may change it.")}
          />
        </Group>
        <Group
          title={tr("Talents de barde", "Bard skills")}
          right={
            <span className="text-body tabular-nums">
              {tr(`${pts} / 50 points`, `${pts} / 50 points`)}
            </span>
          }
        >
          <table className="w-full text-body">
            <thead>
              <tr className="text-left text-label text-ink-muted">
                <th className="pb-1 font-medium">{tr("Talent", "Skill")}</th>
                <th className="pb-1 font-medium">{tr("Base", "Base")}</th>
                <th className="pb-1 font-medium">{tr("Race", "Race")}</th>
                <th className="pb-1 font-medium">DEX</th>
                <th className="pb-1 font-medium">{tr("Points", "Points")}</th>
                <th className="pb-1 font-medium">{tr("Total", "Total")}</th>
              </tr>
            </thead>
            <tbody>
              {BARD_SKILLS.map((s) => (
                <tr key={s.en} className="border-t border-border">
                  <td className="py-1.5 pr-2">{tr(s.fr, s.en)}</td>
                  <td className="text-ink-muted tabular-nums">{s.base}</td>
                  <td className="text-ink-muted tabular-nums">+{s.race}</td>
                  <td className="text-ink-muted tabular-nums">+{s.dex}</td>
                  <td>
                    <span className={cn(fieldClass, "flex w-14 items-center tabular-nums")}>{s.points}</span>
                  </td>
                  <td className="text-key tabular-nums">{s.base + s.race + s.dex + s.points} %</td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="flex flex-wrap items-end gap-6">
            <div className="flex flex-col gap-1">
              <span className={labelClass}>{tr("Armure portée", "Armour worn")}</span>
              <span className={cn(fieldClass, "flex w-44 items-center")}>{tr("Cuir", "Leather")}</span>
            </div>
            <p className="min-w-0 flex-1 text-label text-ink-muted">
              {tr(
                "Budget au niveau 3 : 20 + 2 × 15. Le dépasser est un avertissement, pas un refus.",
                "Budget at level 3: 20 + 2 × 15. Going over is a warning, not a refusal.",
              )}
            </p>
          </div>
        </Group>
      </div>
      <Note kind="info">
        {tr(
          "Un guerrier voit ici sa Force exceptionnelle (18/xx) ; un voleur ses talents de voleur ; un magicien ou un clerc seulement ses points de vie.",
          "A fighter gets their exceptional Strength (18/xx) here; a thief their thief skills; a mage or a cleric only their hit points.",
        )}
      </Note>
    </CreationFrame>
  );
}

// --- Step 5: identity (a refusal and a warning showing) ----------------------------------

function RollField({
  label,
  value,
  unit,
  range,
  roll,
}: {
  label: string;
  value: string;
  unit: string;
  range: string;
  roll: string;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className={labelClass}>{label}</span>
      <div className="flex items-center gap-2">
        <span className={cn(fieldClass, "flex w-20 items-center tabular-nums")}>{value}</span>
        <span className="text-body text-ink-muted">{unit}</span>
        <Button variant="quiet" size="sm">
          <DiceFive aria-hidden />
          {tr("Tirer", "Roll")}
        </Button>
      </div>
      <span className="text-label text-ink-muted">{range}</span>
      <span className="text-label">{roll}</span>
    </div>
  );
}

export function StepIdentity() {
  const extra = LANGUAGES.filter((l) => l.on).length;
  return (
    <CreationFrame
      step={4}
      title={tr("Identité", "Identity")}
      lead={tr("Qui est votre personnage.", "Who your character is.")}
      nextBlocked
      footNote={tr("Choisissez un alignement permis pour continuer.", "Pick an allowed alignment to go on.")}
    >
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-x-6 gap-y-5">
        <Field label={tr("Nom", "Name")} value={CHAR.name} wide className="col-span-2 max-w-[32rem]" />
        <Group title={tr("Alignement", "Alignment")}>
          <div className="grid grid-cols-3 gap-1.5">
            {ALIGNMENTS.map((a, i) => (
              <span
                key={a.en}
                className={cn(
                  "flex min-h-8 items-center gap-1.5 rounded-md px-2 py-1 text-body",
                  i === 0
                    ? "bg-raised text-danger shadow-[inset_0_0_0_1.5px_var(--danger)]"
                    : a.bard
                      ? "chip"
                      : "text-ink-muted shadow-[inset_0_0_0_1px_var(--border)]",
                )}
              >
                {!a.bard && <Prohibit aria-hidden weight="bold" className="size-3.5 shrink-0" />}
                {tr(a.fr, a.en)}
              </span>
            ))}
          </div>
          <Note kind="refusal">
            {tr(
              "Un barde est toujours en partie neutre : loyal bon lui est interdit. Choisissez un des cinq alignements permis.",
              "A bard is always partly neutral: lawful good is not allowed. Pick one of the five allowed alignments.",
            )}
          </Note>
        </Group>
        <Group title={tr("Sexe", "Sex")}>
          <Seg chosen={1} options={[tr("Masculin", "Male"), tr("Féminin", "Female")]} />
          <p className="text-label text-ink-muted">
            {tr("Fixe les fourchettes de taille et de poids.", "Sets the height and weight ranges.")}
          </p>
        </Group>
        <Group
          title={tr("Langues", "Languages")}
          className="col-span-2"
          right={
            <span className="flex items-center gap-1 text-body text-warning">
              <Warning aria-hidden weight="bold" className="size-4" />
              {tr(`${extra} en plus · 3 permises`, `${extra} extra · 3 allowed`)}
            </span>
          }
        >
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="flex min-h-7 items-center gap-1.5 rounded-md bg-raised px-2 py-1 text-body">
              <Lock aria-hidden className="size-3.5" />
              {tr("Commun", "Common")}
              <span className="text-label text-ink-muted">{tr("maternelle", "native")}</span>
            </span>
            {LANGUAGES.map((l) => (
              <span
                key={l.en}
                className={cn(
                  "flex min-h-7 items-center gap-1 px-2 py-1 text-body",
                  l.on ? "chip-chosen" : "chip",
                  !l.race && !l.on && "text-ink-muted",
                )}
              >
                {l.on && <Check aria-hidden weight="bold" className="size-3.5" />}
                {tr(l.fr, l.en)}
              </span>
            ))}
          </div>
          <p className="text-label text-ink-muted">
            {tr(
              "En clair, la liste du demi-elfe ; en grisé, les autres langues, si le MD les permet.",
              "Bright: the half-elf's list; dimmed: the other languages, if the DM allows them.",
            )}
          </p>
          <Note kind="warning">
            {tr(
              "4 langues en plus de la maternelle, l'Intelligence 13 en permet 3. Ce n'est qu'un avertissement : la création reste possible.",
              "4 languages besides the native one; Intelligence 13 allows 3. Only a warning: you can still create.",
            )}
          </Note>
        </Group>
        <Group title={tr("Mensurations", "Build")} className="col-span-2">
          <div className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-4">
            <RollField
              label={tr("Taille", "Height")}
              value={tr("170", "5′7″")}
              unit={tr("cm", "")}
              range={tr("Demi-elfe féminin : 152 – 178 cm", "Female half-elf: 5′0″ – 5′10″")}
              roll={tr("58 po + 2d6 (4 + 5) = 67 po", "58 in + 2d6 (4 + 5) = 67 in")}
            />
            <RollField
              label={tr("Poids", "Weight")}
              value={tr("48", "105")}
              unit={tr("kg", "lb")}
              range={tr("Demi-elfe féminin : 39 – 55 kg", "Female half-elf: 88 – 121 lb")}
              roll={tr("85 lb + 3d12 (7 + 2 + 11) = 105 lb", "85 lb + 3d12 (7 + 2 + 11) = 105 lb")}
            />
            <RollField
              label={tr("Âge", "Age")}
              value="19"
              unit={tr("ans", "years")}
              range={tr("Demi-elfe : 16 – 185 ans", "Half-elf: 16 – 185 years")}
              roll={tr("15 ans + 1d6 (4) = 19 ans", "15 years + 1d6 (4) = 19 years")}
            />
          </div>
        </Group>
        <Field label={tr("Cheveux", "Hair")} value={tr("Auburn, tressés", "Auburn, braided")} wide />
        <Field label={tr("Yeux", "Eyes")} value={tr("Vert d'eau", "Sea green")} wide />
      </div>
    </CreationFrame>
  );
}

// --- Step 6: review -----------------------------------------------------------------------

function ReviewBlock({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex min-w-0 flex-col gap-2 rounded-lg bg-raised p-3">
      <div className="flex items-center justify-between gap-2">
        <h3 className="section-label">{title}</h3>
        <Button variant="ghost" size="icon-sm" aria-label={tr("Modifier", "Edit")}>
          <PencilSimple aria-hidden />
        </Button>
      </div>
      {children}
    </section>
  );
}

function Dl({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-body">
      {rows.map(([k, v]) => (
        <div key={k} className="contents">
          <dt className="text-ink-muted">{k}</dt>
          <dd className="break-words">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

export function StepReview() {
  return (
    <CreationFrame
      step={5}
      title={tr("Récapitulatif", "Review")}
      lead={tr(
        "Vérifiez, puis créez : le personnage devient celui que vous jouez.",
        "Check, then create: the character becomes the one you play.",
      )}
      next={tr("Créer", "Create")}
      footNote={tr("1 avertissement, rien de bloquant.", "1 warning, nothing blocking.")}
    >
      <Note kind="warning">
        {tr(
          "Langues : 4 en plus de la maternelle, l'Intelligence 13 en permet 3.",
          "Languages: 4 besides the native one; Intelligence 13 allows 3.",
        )}
      </Note>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(18rem,1fr))] gap-3">
        <ReviewBlock title={tr("Classe et race", "Class and race")}>
          <Dl
            rows={[
              [tr("Nom", "Name"), CHAR.name],
              [tr("Classe", "Class"), tr("Barde · niveau 3 (2 500 XP)", "Bard · level 3 (2,500 XP)")],
              [tr("Race", "Race"), tr("Demi-elfe", "Half-elf")],
              ["TAC0", "19"],
            ]}
          />
        </ReviewBlock>
        <ReviewBlock title={tr("Caractéristiques", "Ability scores")}>
          <div className="grid grid-cols-6 gap-1">
            {ABILITIES.map((a) => (
              <div key={a.id} className="flex flex-col items-center rounded-md bg-surface py-1">
                <span className="text-label text-ink-muted">{tr(a.fr, a.en)}</span>
                <span className="text-key tabular-nums">{a.score}</span>
              </div>
            ))}
          </div>
          <span className="text-label text-ink-muted">{tr("Tirées · méthode V", "Rolled · method V")}</span>
        </ReviewBlock>
        <ReviewBlock title={tr("Points de vie et talents", "Hit points and skills")}>
          <Dl
            rows={[
              [tr("PV max", "Max HP"), tr("14 (tirés : 5 + 3 + 6)", "14 (rolled: 5 + 3 + 6)")],
              [tr("Escalade", "Climb walls"), "65 %"],
              [tr("Perception des bruits", "Detect noise"), "35 %"],
              [tr("Vol à la tire", "Pick pockets"), "30 %"],
              [tr("Lecture des langues", "Read languages"), "13 %"],
            ]}
          />
        </ReviewBlock>
        <ReviewBlock title={tr("Identité", "Identity")}>
          <Dl
            rows={[
              [tr("Alignement", "Alignment"), tr("Neutre bon", "Neutral good")],
              [tr("Sexe", "Sex"), tr("Féminin", "Female")],
              [
                tr("Langues", "Languages"),
                <span key="l" className="text-warning">
                  {tr("Commun, elfe, gnome, halfelin, orque", "Common, elf, gnome, halfling, orc")}
                </span>,
              ],
              [tr("Taille", "Height"), tr("1,70 m", "5′7″")],
              [tr("Poids", "Weight"), tr("48 kg", "105 lb")],
              [tr("Âge", "Age"), tr("19 ans", "19 years")],
              [tr("Cheveux", "Hair"), tr("Auburn, tressés", "Auburn, braided")],
              [tr("Yeux", "Eyes"), tr("Vert d'eau", "Sea green")],
            ]}
          />
        </ReviewBlock>
      </div>
    </CreationFrame>
  );
}

// --- Roster -------------------------------------------------------------------------------

export function Roster() {
  return (
    <div className="flex h-full flex-col gap-3 p-3">
      <header className="zone-card flex items-center gap-4 px-4 py-2">
        <Wordmark />
        <h1 className="text-title">{tr("Vos personnages", "Your characters")}</h1>
        <span className="flex-1" />
        <Button variant="quiet">
          <ArrowLeft aria-hidden />
          {tr("Retour", "Back")}
        </Button>
      </header>
      <main className="stage-card min-h-0 flex-1 overflow-y-auto p-4">
        <div className="grid grid-cols-[repeat(auto-fill,minmax(19rem,1fr))] gap-3">
          {ROSTER.map((c) => (
            <section
              key={c.name}
              className={cn("zone-card flex flex-col", c.current && "latest")}
            >
              <div className="flex flex-col gap-1 px-4 pt-3">
                <div className="flex items-start gap-2">
                  <span className="min-w-0 flex-1 text-key break-words">{c.name}</span>
                  {c.current && (
                    <span className="shrink-0 rounded-md bg-tint px-1.5 py-0.5 text-label font-semibold text-on-accent">
                      {tr("En jeu", "Playing")}
                    </span>
                  )}
                </div>
                <span className="text-body">
                  {tr(`${c.classFr} · niveau ${c.level}`, `${c.classEn} · level ${c.level}`)}
                </span>
                <span className="text-body text-ink-muted">
                  {tr(c.raceFr, c.raceEn)}
                  {c.alFr && ` · ${tr(c.alFr, c.alEn)}`}
                </span>
                {c.missing && (
                  <span className="mt-1 flex items-center gap-1 self-start rounded-md bg-raised px-1.5 py-0.5 text-label text-warning">
                    <Warning aria-hidden weight="bold" className="size-3.5" />
                    {tr("Alignement et sexe à choisir", "Alignment and sex to pick")}
                  </span>
                )}
                <span className="text-body text-ink-muted">
                  {tr("PV", "HP")} {c.hp}
                </span>
              </div>
              <div className="mt-auto flex items-center gap-2 px-4 pt-3 pb-3">
                {!c.current && (
                  <Button variant="main">
                    <Play aria-hidden />
                    {tr("Jouer", "Play")}
                  </Button>
                )}
                <span className="flex-1" />
                <Button variant="quiet">
                  <PencilSimple aria-hidden />
                  {tr("Renommer", "Rename")}
                </Button>
                <Button variant="destructive">
                  <Archive aria-hidden />
                  {tr("Retirer", "Retire")}
                </Button>
              </div>
            </section>
          ))}
          <section className="flex min-h-36 flex-col items-center justify-center gap-2 rounded-[0.625rem] border-2 border-dashed border-input p-4">
            <Button variant="quiet" size="lg">
              <UserPlus aria-hidden />
              {tr("Créer", "Create")}
            </Button>
            <span className="text-center text-label text-ink-muted">
              {tr("Ouvre la création en plein écran.", "Opens the full-screen creation.")}
            </span>
          </section>
        </div>
      </main>
    </div>
  );
}

// --- The dashboard's character card: the roster's way in, and Rename in the details ------

export function CharacterCard() {
  return (
    <div className="flex h-full gap-3 p-3">
      <aside className="zone-card flex w-[clamp(280px,21vw,384px)] shrink-0 flex-col">
        <div className="card-band flex items-center gap-2 px-4 py-2">
          <span className="min-w-0 flex-1 text-title break-words underline decoration-dotted underline-offset-4">
            Kelsen Balstriel
          </span>
          <span className="text-body text-ink-muted">{tr("Niveau 15", "Level 15")}</span>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={tr("Vos personnages", "Your characters")}
            className="shadow-[inset_0_0_0_1.5px_var(--tint)]"
          >
            <ArrowsLeftRight aria-hidden />
          </Button>
        </div>
        <div className="flex flex-col gap-3 px-4 py-3">
          <div className="grid grid-cols-7 gap-1">
            {["FOR 12", "DEX 17", "CON 16", "INT 14", "SAG 8", "CHA 16", "TAC0 13"].map((s) => (
              <div key={s} className="flex flex-col items-center rounded-md bg-raised py-1">
                <span className="text-label text-ink-muted">{s.split(" ")[0]}</span>
                <span className="text-key tabular-nums">{s.split(" ")[1]}</span>
              </div>
            ))}
          </div>
          <span className="text-label text-ink-muted">
            {tr(
              "Le bouton à droite du niveau ouvre « Vos personnages » ; le nom ouvre les détails.",
              "The button right of the level opens “Your characters”; the name opens the details.",
            )}
          </span>
        </div>
      </aside>
      <section className="zone-card flex w-[30rem] flex-col self-start shadow-[var(--float-shadow)]">
        <CardHead>{tr("Détails du personnage", "Character details")}</CardHead>
        <div className="grid grid-cols-2 gap-3 px-4 py-3">
          <Field label={tr("Nom", "Name")} value="Kelsen Balstriel" wide className="col-span-2" />
          <Field label={tr("Alignement", "Alignment")} value={tr("Neutre bon", "Neutral good")} wide />
          <Field label={tr("Sexe", "Sex")} value={tr("Masculin", "Male")} wide />
          <Field label={tr("Taille", "Height")} value="1,78" unit="m" />
          <Field label={tr("Poids", "Weight")} value="72" unit="kg" />
          <Field label={tr("Âge", "Age")} value="34" unit={tr("ans", "years")} />
          <Field label={tr("Cheveux", "Hair")} value={tr("Noirs", "Black")} wide />
          <Field
            label={tr("Langues", "Languages")}
            value={tr("Commun, elfe, gnome, halfelin", "Common, elf, gnome, halfling")}
            wide
            className="col-span-2"
          />
        </div>
        <div className="flex justify-end gap-2 border-t border-border px-4 py-3">
          <Button variant="quiet">{tr("Annuler", "Cancel")}</Button>
          <Button variant="main">{tr("Enregistrer", "Save")}</Button>
        </div>
      </section>
    </div>
  );
}

// --- The slimmed cog menu -----------------------------------------------------------------

export function CogMenu() {
  const seg = "flex min-h-7 flex-auto items-center justify-center gap-1 rounded-md px-2 py-1 text-body [&_svg]:size-4";
  return (
    <div className="flex h-full flex-col justify-end gap-3 p-3">
      <section className="zone-card flex w-80 flex-col gap-3 p-4 shadow-[var(--float-shadow)]">
        <h2 className="section-label">{tr("Réglages", "Settings")}</h2>
        <div className="flex flex-col gap-2">
          <span className="text-label text-muted-foreground">{tr("Langue", "Language")}</span>
          <div className="flex gap-1">
            <span className={cn(seg, tr("chip", "chip-chosen"))}>English</span>
            <span className={cn(seg, tr("chip-chosen", "chip"))}>Français</span>
          </div>
        </div>
        <div className="flex flex-col gap-2">
          <span className="text-label text-muted-foreground">{tr("Apparence", "Appearance")}</span>
          <div className="flex flex-wrap gap-1">
            <span className={cn(seg, "chip-chosen")}>
              <Moon aria-hidden />
              {tr("Sombre", "Dark")}
            </span>
            <span className={cn(seg, "chip")}>
              <Sun aria-hidden />
              {tr("Clair", "Light")}
            </span>
            <span className={cn(seg, "chip")}>
              <Desktop aria-hidden />
              {tr("Système", "System")}
            </span>
          </div>
        </div>
        <div className="flex flex-col gap-2">
          <span className="text-label text-muted-foreground">{tr("Thème", "Theme")}</span>
          <div className="flex flex-wrap gap-1">
            {[tr("Arcane", "Arcane"), tr("Forêt", "Forest"), tr("Braise", "Ember"), tr("Ambre", "Amber")].map(
              (n, i) => (
                <span key={n} className={cn(seg, i === 0 ? "chip-chosen" : "chip")}>
                  {n}
                </span>
              ),
            )}
          </div>
        </div>
        <div className="flex flex-col gap-2">
          <span className="text-label text-muted-foreground">{tr("Compte", "Account")}</span>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="min-w-0 text-body [overflow-wrap:anywhere]">moncef.naji@gmail.com</span>
            <Button variant="quiet">
              <SignOut aria-hidden />
              {tr("Se déconnecter", "Log out")}
            </Button>
          </div>
          <Button variant="quiet" className="self-start">
            <ShieldCheck aria-hidden />
            {tr("Administration", "Admin")}
          </Button>
        </div>
      </section>
      <footer className="zone-card flex items-center gap-3 px-3 py-2">
        <span className="flex size-8 items-center justify-center rounded-md bg-raised-hover [&_svg]:size-5">
          <GearSix aria-hidden />
        </span>
        <span className="section-label">{tr("Dés", "Dice")}</span>
        <span className="text-body text-ink-muted">d4 · d6 · d8 · d10 · d12 · d20 · d%</span>
      </footer>
    </div>
  );
}

export const SCREENS: Record<string, () => ReactNode> = {
  intro: Intro,
  "1-class": StepClass,
  "2-race": StepRace,
  "3-scores": StepScores,
  "4-hp-skills": StepHp,
  "5-identity": StepIdentity,
  "6-review": StepReview,
  roster: Roster,
  "character-card": CharacterCard,
  "cog-menu": CogMenu,
};
