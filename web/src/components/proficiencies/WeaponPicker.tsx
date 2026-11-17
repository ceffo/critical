import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Check, Plus, X } from "@phosphor-icons/react";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Note } from "@/components/creation/kit";
import { Button } from "@/components/ui/button";
import { catalogueRows } from "@/lib/catalogue";
import { cn } from "@/lib/utils";
import { weaponName } from "@/lib/weaponNames";
import { useEquipmentCatalogueStore } from "@/stores/equipmentCatalogue";
import type {
  ProficiencySlots,
  WeaponOption,
  WeaponPickInput,
  WeaponPool,
} from "@/types/character";
import { fold } from "./catalogue";
import { Band, PoolBar } from "./pickParts";
import {
  PLAIN_WEAPON_SLOTS,
  addWeapon,
  groupWeaponRows,
  isSpecialization,
  removeWeapon,
  specialize,
  weaponRows,
  type WeaponRow,
} from "./weapons";

/**
 * The weapon proficiency picker (ADR 0037 steps 3, 12 and 21), shared by the Skills activity's editor
 * and the creation's step: the pool bar that stays at the top of the scroll, the weapons held on the
 * left (a fighter's Specialize on each) and on the right a search over the catalogue's weapons by
 * family, each family a band that opens. In a family the weapons the character's classes may pick
 * come first (the derived `weapons`, named from the equipment catalogue), then the others, greyed
 * with the reason; a family with none to pick is a quiet closed band. A weapon is added at one slot,
 * and the add is disabled when the pool has none left. Every number is the server's: the pool from
 * its preview or checks (`figures`), a specialization's cost from the options; a refusal (a second
 * specialization, an overspend) is the server's, shown on the row or above the list.
 */
export function WeaponPicker({
  weapons,
  picks,
  onChange,
  figures,
  stale = false,
  errorFor,
  classes = "",
  header,
  barClassName,
}: {
  /** The character's weapons and their specialization cost (the derived `weapons`). */
  weapons: WeaponOption[];
  picks: WeaponPickInput[];
  onChange: (picks: WeaponPickInput[]) => void;
  /** The server's figures for the picks shown; null before its first answer. */
  figures: ProficiencySlots | null;
  /** True while the figures shown are not those of the picks (a refusal, a failed preview). */
  stale?: boolean;
  /** A refusal's words for a field (`weapon.<code>`, or `weapon` for the budget), or null. */
  errorFor: (field: string) => string | null;
  /** The names of the character's classes, for the reason a weapon is not allowed. */
  classes?: string;
  /** The editor's title and Save / Cancel, kept with the pool bar at the top. */
  header?: ReactNode;
  barClassName?: string;
}) {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState("");
  // The rows are named from the equipment catalogue (ADR 0037 step 21.6), read once.
  const catalogue = useEquipmentCatalogueStore((s) => s.catalogue);
  const load = useEquipmentCatalogueStore((s) => s.load);
  useEffect(() => {
    void load().catch(() => undefined); // the codes name the rows meanwhile
  }, [load]);
  const nameOf = (code: string) =>
    weaponName(t, code, catalogue, i18n.language);
  const q = fold(query.trim());
  const all = weaponRows(
    weapons,
    picks,
    figures?.weaponPicks,
    catalogue
      ? catalogueRows(catalogue, "weapon", "", i18n.language, t).map(
          (r) => r.code,
        )
      : [],
  );
  const families = groupWeaponRows(
    all.filter((r) => q === "" || fold(nameOf(r.code)).includes(q)),
    catalogue,
    i18n.language,
    t,
  );
  const firstOpen = families.findIndex((g) => g.rows.some((r) => r.allowed));
  const budgetError = errorFor("weapon");
  const redo = figures?.weaponRedo ?? [];
  const left = figures?.weapon?.left;
  const held = picks.flatMap((p) => {
    const row = all.find((r) => r.code === p.code);
    return row ? [row] : [];
  });
  const offersSpecialization = weapons.some((o) => o.specializationSlots > 0);
  const bar = figures?.weapon ? (
    <WeaponPoolSummary
      pool={figures.weapon}
      header={header}
      stale={stale}
      className={barClassName}
    />
  ) : (
    header && <div className="sticky top-0 z-10">{header}</div>
  );
  return (
    <div className="flex min-w-0 flex-col gap-3" data-testid="weapon-picker">
      {bar}
      {budgetError && <Note kind="refusal">{budgetError}</Note>}
      {redo.length > 0 && (
        <Note kind="warning" testId="weapon-redo">
          {t("weaponProficiencies.redo", {
            types: redo
              .map((r) =>
                t("weaponProficiencies.redoType", {
                  type: t(`weapons.types.${r.type}`, { defaultValue: r.type }),
                  count: r.slots,
                }),
              )
              .join(", "),
          })}
        </Note>
      )}
      <div className="grid grid-cols-1 items-start gap-5 min-[1100px]:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
        <section
          aria-labelledby="weapon-held-title"
          className="flex min-w-0 flex-col gap-2"
        >
          <h4
            id="weapon-held-title"
            className="group-heading section-label px-2.5"
          >
            {t("weaponProficiencies.heldTitle")}
          </h4>
          {held.length === 0 ? (
            <p className="px-2.5 text-body break-words text-ink-muted">
              {t("weaponProficiencies.none")}
            </p>
          ) : (
            <ul className="flex flex-col gap-1" data-testid="weapon-held">
              {held.map((r) => (
                <HeldRow
                  key={r.code}
                  row={r}
                  name={nameOf(r.code)}
                  error={errorFor(`weapon.${r.code}`)}
                  onRemove={() => onChange(removeWeapon(picks, r.code))}
                  onSpecialize={(on) =>
                    onChange(
                      specialize(
                        picks,
                        r.code,
                        on ? (r.option?.specializationSlots ?? null) : null,
                      ),
                    )
                  }
                />
              ))}
            </ul>
          )}
          <p className="px-2.5 text-label break-words text-ink-muted">
            {t("weaponProficiencies.lead")}
            {offersSpecialization &&
              ` ${t("weaponProficiencies.leadSpecializes")}`}
          </p>
        </section>
        <section
          aria-labelledby="weapon-catalogue-title"
          className="flex min-w-0 flex-col gap-2"
          role="group"
          data-testid="weapon-rows"
        >
          <h4
            id="weapon-catalogue-title"
            className="group-heading section-label px-2.5"
          >
            {t("weaponProficiencies.catalogue")}
          </h4>
          <div className="flex flex-col gap-1 px-2.5">
            <label htmlFor="weapon-search" className={labelClass}>
              {t("proficiencies.search")}
            </label>
            <input
              id="weapon-search"
              type="search"
              className={fieldClass}
              autoComplete="off"
              placeholder={t("weaponProficiencies.searchPlaceholder")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              data-testid="weapon-search"
            />
          </div>
          {families.map((g, i) => {
            const allowed = g.rows.filter((r) => r.allowed);
            // The class's weapons first, then the others (stable within each, the catalogue's order).
            const rows = [...allowed, ...g.rows.filter((r) => !r.allowed)];
            return (
              <Band
                key={g.key || "other"}
                title={g.label || t("weaponProficiencies.otherFamily")}
                count={rows.length}
                quiet={allowed.length === 0}
                defaultOpen={i === firstOpen}
                forceOpen={q !== ""}
                testId={g.key ? `weapon-group-${g.key}` : undefined}
              >
                <ul className="flex flex-col gap-1">
                  {rows.map((r) => (
                    <CatalogueRowItem
                      key={r.code}
                      row={r}
                      name={nameOf(r.code)}
                      classes={classes}
                      full={left !== undefined && left <= 0}
                      onAdd={() => onChange(addWeapon(picks, r.code))}
                    />
                  ))}
                </ul>
              </Band>
            );
          })}
          {families.length === 0 && (
            <p className="px-2.5 text-body break-words text-ink-muted">
              {t("weaponProficiencies.noMatch")}
            </p>
          )}
        </section>
      </div>
    </div>
  );
}

/** The weapon pool as the server derives it (ADR 0037 step 3), in the bar that stays at the top,
 * with the overspend's warning. */
export function WeaponPoolSummary({
  pool,
  header,
  stale = false,
  className,
}: {
  pool: WeaponPool;
  header?: ReactNode;
  stale?: boolean;
  className?: string;
}) {
  const { t } = useTranslation();
  return (
    <PoolBar
      testId="weapon-pool"
      header={header}
      left={pool.left}
      available={pool.available}
      stale={stale}
      className={className}
      terms={t("weaponProficiencies.pool", {
        initial: pool.initial,
        levels: pool.levelGained,
        perLevels: pool.perLevels,
        available: pool.available,
      })}
    >
      {stale && (
        <p className="px-2.5 text-label break-words text-ink-muted">
          {t("proficiencies.previewStale")}
        </p>
      )}
      {pool.left < 0 && (
        <div className="px-2.5">
          <Note kind="warning" testId="weapon-over">
            {t("proficiencies.over", {
              spent: pool.spent,
              available: pool.available,
            })}
          </Note>
        </div>
      )}
    </PoolBar>
  );
}

/** One weapon held: its name, what it spends, a fighter's Specialize (or the specialization held)
 * and its own Remove. */
function HeldRow({
  row,
  name,
  error,
  onRemove,
  onSpecialize,
}: {
  row: WeaponRow;
  name: string;
  error: string | null;
  onRemove: () => void;
  onSpecialize: (on: boolean) => void;
}) {
  const { t } = useTranslation();
  const { pick, option } = row;
  const specialized = isSpecialization(pick);
  const cost = option?.specializationSlots ?? 0;
  return (
    <li
      className="flex min-w-0 flex-col gap-1 rounded-md bg-sub-band px-2.5 py-1"
      data-testid={`weapon-held-${row.code}`}
      data-specialized={specialized ? "true" : undefined}
    >
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-x-3 gap-y-1">
        <span className="flex min-w-0 flex-col">
          <span className="text-body break-words">{name}</span>
          {!row.allowed && (
            <span
              className="text-label break-words text-warning"
              data-testid="weapon-row-not-allowed"
            >
              {t("weaponProficiencies.notAllowed")}
            </span>
          )}
        </span>
        <span className="flex flex-wrap items-center justify-end gap-2">
          {pick && cost > 0 && (
            <Button
              type="button"
              size="xs"
              // A pressed toggle is the chosen chip (ADR 0026 step 5.3).
              aria-pressed={specialized}
              title={
                specialized
                  ? t("weaponProficiencies.unspecializeHint", { name })
                  : t("weaponProficiencies.specializeHint", {
                      name,
                      count: cost,
                    })
              }
              onClick={() => onSpecialize(!specialized)}
              data-testid="weapon-specialize"
            >
              {specialized
                ? t("weaponProficiencies.specialized")
                : t("weaponProficiencies.specializeMore", {
                    count: Math.max(cost - pick.slots, 0),
                  })}
            </Button>
          )}
          <span
            className="text-label whitespace-nowrap text-ink-muted tabular-nums"
            data-testid="weapon-row-status"
          >
            {t("proficiencies.slotsShort", { count: pick?.slots ?? 0 })}
          </span>
          <Button
            type="button"
            size="icon-xs"
            aria-label={t("weaponProficiencies.remove", { name })}
            title={t("weaponProficiencies.remove", { name })}
            onClick={onRemove}
            data-testid="weapon-remove"
          >
            <X aria-hidden />
          </Button>
        </span>
      </div>
      {error && (
        <p role="alert" className="text-label break-words text-danger">
          {error}
        </p>
      )}
    </li>
  );
}

/** One catalogue weapon: its name and, for one the character may pick, its add; for the others, the
 * reason it is not allowed. `full`: the pool has no slot left. */
function CatalogueRowItem({
  row,
  name,
  classes,
  full,
  onAdd,
}: {
  row: WeaponRow;
  name: string;
  classes: string;
  full: boolean;
  onAdd: () => void;
}) {
  const { t } = useTranslation();
  const has = row.pick !== undefined;
  const label = has
    ? t("weaponProficiencies.alreadyHeld", { name })
    : full
      ? t("weaponProficiencies.noRoom", { name })
      : t("weaponProficiencies.addHint", { name });
  return (
    <li
      className={cn(
        "flex min-w-0 items-center justify-between gap-3 rounded-md px-2.5 py-1 text-body",
        row.allowed && !has ? "bg-raised" : "text-ink-muted",
      )}
      data-testid={`weapon-row-${row.code}`}
      data-allowed={row.allowed ? "true" : "false"}
    >
      <span className="flex min-w-0 flex-col">
        <span className="break-words">{name}</span>
        {!row.allowed && !has && (
          <span
            className="text-label break-words"
            data-testid="weapon-row-reason"
          >
            {classes
              ? t("weaponProficiencies.outsideList", { classes })
              : t("weaponProficiencies.outsideListAny")}
          </span>
        )}
      </span>
      {(row.allowed || has) && (
        <span className="flex shrink-0 items-center gap-2">
          <span className="text-label tabular-nums">
            {t("proficiencies.slotsShort", { count: PLAIN_WEAPON_SLOTS })}
          </span>
          <Button
            type="button"
            size="icon-xs"
            title={label}
            aria-label={label}
            disabled={has || full}
            onClick={onAdd}
            data-testid="weapon-add"
          >
            {has ? <Check aria-hidden /> : <Plus aria-hidden />}
          </Button>
        </span>
      )}
    </li>
  );
}
