// The equipment round's own pieces: a possession's row, the possessions card, the AC card and the
// review list, on the app's real classes (ADR 0026 step 5's shared pieces, ADR 0027's tiers).
import type { ReactNode } from "react";
import {
  ArrowCounterClockwise,
  Backpack,
  Crosshair,
  Diamond,
  Flask,
  MagicWand,
  MagnifyingGlass,
  Minus,
  PencilSimple,
  Plus,
  Shield,
  Sword,
  TShirt,
  Trash,
  Warning,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { lt, tr } from "./lang";
import {
  AC_ROWS,
  CATEGORIES,
  POSSESSIONS,
  REVIEW,
  type Category,
  type Piece,
} from "./data";

export function CategoryIcon({ p, className }: { p: Piece; className?: string }) {
  const c = cn("size-5 shrink-0 text-ink-muted", className);
  const map: Record<Category, ReactNode> = {
    weapon: <Sword className={c} aria-hidden />,
    ammunition: <Crosshair className={c} aria-hidden />,
    armour: <TShirt className={c} aria-hidden />,
    shield: <Shield className={c} aria-hidden />,
    magic:
      p.id === "wand" ? (
        <MagicWand className={c} aria-hidden />
      ) : (
        <Diamond className={c} aria-hidden />
      ),
    gear:
      p.id === "flasks" ? (
        <Flask className={c} aria-hidden />
      ) : (
        <Backpack className={c} aria-hidden />
      ),
  };
  return map[p.category];
}

/** Worn and wielded are one concept on screen (user, round 1): Équipé / Equipped. */
export const equipped = (p: Piece) => p.state !== "carried";

/** The one-word move of a row: Équiper / Retirer, for the categories that can be equipped. */
function stateMove(p: Piece): string | null {
  const equippable =
    p.category === "weapon" || p.category === "armour" || p.category === "shield" || (p.category === "magic" && p.id !== "wand");
  if (!equippable) return null;
  return equipped(p) ? tr("Retirer", "Unequip") : tr("Équiper", "Equip");
}

export function Tag({
  tone,
  children,
}: {
  tone: "warning" | "tint" | "muted";
  children: ReactNode;
}) {
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded-md px-1.5 text-label font-medium whitespace-nowrap",
        tone === "warning" && "bg-raised text-warning",
        tone === "tint" && "chip-chosen",
        tone === "muted" && "chip text-ink-muted",
      )}
    >
      {tone === "warning" && <Warning weight="fill" className="size-3" aria-hidden />}
      {children}
    </span>
  );
}

/** A possession's row: icon, name and its line, count, the state move; edit / delete on hover. */
export function PieceRow({
  p,
  hover,
  warning,
}: {
  p: Piece;
  hover?: boolean;
  warning?: string;
}) {
  const move = stateMove(p);
  return (
    <li
      className={cn(
        "flex shrink-0 items-center gap-3 rounded-md px-3 py-1.5",
        hover ? "bg-raised-hover" : "bg-raised",
      )}
    >
      <CategoryIcon p={p} />
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="text-body font-medium">{lt(p.name)}</span>
          {equipped(p) && <Tag tone="tint">{tr("Équipé", "Equipped")}</Tag>}
          {p.review && <Tag tone="warning">{tr("À vérifier", "To check")}</Tag>}
        </span>
        <span className="text-label text-ink-muted">
          {lt(p.kind)}
          {lt(p.stats) && ` · ${lt(p.stats)}`}
        </span>
        {warning && <span className="text-label text-warning">{warning}</span>}
      </div>
      {p.count && <Count c={p.count} />}
      {hover && (
        <span className="flex gap-0.5">
          <Button variant="ghost" size="icon-sm" title={tr("Modifier", "Edit")}>
            <PencilSimple aria-hidden />
          </Button>
          <Button variant="ghost" size="icon-sm" title={tr("Supprimer", "Delete")}>
            <Trash aria-hidden />
          </Button>
        </span>
      )}
      {move && (
        <Button variant="quiet" size="sm" className="w-[5.5rem]">
          {move}
        </Button>
      )}
      {!move && <span className="w-[5.5rem] shrink-0" />}
    </li>
  );
}

function Count({ c }: { c: NonNullable<Piece["count"]> }) {
  if (c.behaviour === "stock")
    return (
      <span className="flex items-center gap-1">
        <Button variant="quiet" size="icon-xs">
          <Minus aria-hidden />
        </Button>
        <span className="w-7 text-center text-body font-medium tabular-nums">{c.value}</span>
        <Button variant="quiet" size="icon-xs">
          <Plus aria-hidden />
        </Button>
      </span>
    );
  return (
    <span className="text-label whitespace-nowrap text-ink-muted">
      {c.behaviour === "charges" ? tr("Charges", "Charges") : tr("Quotidien", "Daily")}{" "}
      <b className="text-body text-ink tabular-nums">
        {c.value}
        {c.max ? ` / ${c.max}` : ""}
      </b>
    </span>
  );
}

export function GroupHead({ title, note }: { title: string; note?: string }) {
  return (
    <div className="group-heading flex items-baseline justify-between gap-3 px-3">
      <h3 className="section-label">{title}</h3>
      {note && <span className="text-label text-ink-muted">{note}</span>}
    </div>
  );
}

/** The category filter chips (rare: the list is short; "Tout" by default). */
export function CategoryChips({ chosen = "all" }: { chosen?: string }) {
  return (
    <div className="flex flex-wrap gap-1">
      {CATEGORIES.map((c) => (
        <span
          key={c.id}
          className={cn(
            "flex min-h-7 items-center rounded-md px-2.5 text-body",
            c.id === chosen ? "chip-chosen" : "chip",
          )}
        >
          {lt(c.name)}
        </span>
      ))}
    </div>
  );
}

const byId = (id: string) => POSSESSIONS.find((p) => p.id === id)!;

/**
 * The possessions card (ADR 0033 step 1): one flat list in the player's order, the category chips
 * as its only filter; an equipped piece is tagged "Équipé" (user, round 1: no groups by state).
 */
export function PossessionsCard({
  pieces = POSSESSIONS,
  review = "band",
  warnings = {},
  hoverId = "longsword",
}: {
  pieces?: Piece[];
  review?: "band" | "none";
  warnings?: Record<string, string>;
  hoverId?: string;
}) {
  return (
    <section className="stage-card flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="card-band flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2">
        <h2 className="section-label">{tr("Possessions", "Possessions")}</h2>
        <CategoryChips />
        <span className="flex-1" />
        <Button variant="quiet">
          <Plus aria-hidden />
          {tr("Ajouter", "Add")}
        </Button>
      </div>
      {review === "band" && (
        <div className="flex items-center gap-3 bg-raised px-4 py-2 text-body text-warning shadow-[inset_3px_0_0_var(--warning)]">
          <Warning weight="fill" className="size-4 shrink-0" aria-hidden />
          <span className="min-w-0 flex-1">
            {tr(
              "3 possessions à vérifier après le passage au nouvel inventaire",
              "3 possessions to check after the move to the new inventory",
            )}
          </span>
          <Button variant="quiet" size="sm">
            {tr("Vérifier", "Check")}
          </Button>
        </div>
      )}
      <ul className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto p-4">
        {pieces.map((p) => (
          <PieceRow key={p.id} p={p} hover={p.id === hoverId} warning={warnings[p.id]} />
        ))}
      </ul>
    </section>
  );
}

/** The AC card (ADR 0033 step 10): the number, then one row per term with its source. */
export function AcCard({
  override,
  warnings,
  rows = AC_ROWS,
  total = "−2",
  className,
}: {
  override?: boolean;
  warnings?: string[];
  rows?: typeof AC_ROWS;
  total?: string;
  className?: string;
}) {
  return (
    <section
      className={cn(
        "zone-card flex w-[clamp(300px,22vw,420px)] shrink-0 flex-col overflow-y-auto",
        className,
      )}
    >
      <div className="card-band flex min-h-9 items-center justify-between gap-3 px-4 py-2">
        <h2 className="section-label">{tr("Classe d'armure", "Armour class")}</h2>
        {!override && (
          <Button variant="quiet" size="sm" title={tr("Forcer une CA (sort Bouclier…)", "Override the AC (Shield spell…)")}>
            {tr("Forcer", "Override")}
          </Button>
        )}
      </div>
      <div className="flex flex-col gap-3 p-4">
        {override ? (
          <div className="flex items-end justify-between gap-3">
            <div className="flex flex-col">
              <span className="text-label text-warning">
                {tr("Forcée : sort Bouclier", "Override: Shield spell")}
              </span>
              <span className="text-hero leading-none tabular-nums">4</span>
            </div>
            <div className="flex flex-col items-end">
              <span className="text-label text-ink-muted">{tr("Calculée", "Derived")}</span>
              <span className="text-key text-ink-muted tabular-nums">{total}</span>
            </div>
            <Button variant="quiet" size="sm">
              <ArrowCounterClockwise aria-hidden />
              {tr("Effacer", "Clear")}
            </Button>
          </div>
        ) : (
          <span className="text-hero leading-none tabular-nums">{total}</span>
        )}
        {warnings?.map((w) => (
          <p
            key={w}
            className="flex items-start gap-2 rounded-md bg-raised px-3 py-2 text-body text-warning shadow-[inset_3px_0_0_var(--warning)]"
          >
            <Warning weight="fill" className="mt-0.5 size-4 shrink-0" aria-hidden />
            <span className="min-w-0">{w}</span>
          </p>
        ))}
        <AcBreakdown rows={rows} total={total} />
      </div>
    </section>
  );
}

export function AcBreakdown({ rows, total }: { rows: typeof AC_ROWS; total: string }) {
  return (
    <dl className="grid grid-cols-[auto_1fr_auto] gap-x-3 gap-y-1.5 text-body">
      {rows.map((r) => (
        <div key={r.term.en} className={cn("contents", r.muted && "text-ink-muted")}>
          <dt className={r.muted ? "" : "font-medium"}>{lt(r.term)}</dt>
          <dd className="min-w-0 text-ink-muted">{lt(r.source)}</dd>
          <dd className="text-right tabular-nums">{r.value}</dd>
        </div>
      ))}
      <div className="col-span-3 my-0.5 h-px bg-border" />
      <dt className="font-semibold">{tr("CA", "AC")}</dt>
      <dd className="text-label text-ink-muted">{tr("jamais mieux que −10", "never better than −10")}</dd>
      <dd className="text-right font-semibold tabular-nums">{total}</dd>
    </dl>
  );
}

/** The review list (ADR 0033 step 27): each piece, why, and the quickest way to settle it. */
export function ReviewCard() {
  return (
    <section className="zone-card flex shrink-0 flex-col">
      <div className="card-band flex min-h-9 items-center justify-between gap-3 px-4 py-2">
        <h2 className="flex items-center gap-2 section-label">
          <Warning weight="fill" className="size-4 text-warning" aria-hidden />
          {tr("À vérifier", "To check")}
          <span className="text-ink-muted">· 3</span>
        </h2>
        <span className="text-label text-ink-muted">
          {tr(
            "Le passage au nouvel inventaire n'a pas pu tout classer.",
            "The move to the new inventory could not file everything.",
          )}
        </span>
      </div>
      <ul className="flex flex-col gap-1 p-3">
        {REVIEW.map((r) => {
          const p = byId(r.id);
          return (
            <li key={r.id} className="flex flex-col gap-2 rounded-md bg-raised px-3 py-2">
              <div className="flex items-start gap-3">
                <CategoryIcon p={p} className="mt-0.5" />
                <div className="flex min-w-0 flex-1 flex-col">
                  <span className="text-body font-medium">{lt(p.name)}</span>
                  <span className="text-body text-ink-muted">{lt(r.reason)}</span>
                </div>
                <span className="flex shrink-0 gap-1">
                  {r.id === "mace" && (
                    <Button variant="quiet" size="sm">
                      <MagnifyingGlass aria-hidden />
                      {tr("Choisir", "Pick")}
                    </Button>
                  )}
                  {r.id === "bag" && (
                    <Button variant="quiet" size="sm">
                      <PencilSimple aria-hidden />
                      {tr("Modifier", "Edit")}
                    </Button>
                  )}
                  <Button variant="ghost" size="sm" title={tr("Garder tel quel et retirer de la liste", "Keep as is and take off the list")}>
                    {tr("Ignorer", "Dismiss")}
                  </Button>
                </span>
              </div>
              {r.choices && (
                <div className="flex flex-wrap gap-1 pl-8">
                  {r.choices.map((c, i) => (
                    <span
                      key={c.en}
                      className={cn(
                        "flex min-h-7 items-center rounded-md px-2.5 text-body",
                        i === 0 ? "chip-chosen" : "chip",
                      )}
                    >
                      {lt(c)}
                    </span>
                  ))}
                  <Button variant="quiet" size="sm">
                    {tr("Confirmer", "Confirm")}
                  </Button>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
