import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Plus, X } from "@phosphor-icons/react";
import { Note } from "@/components/creation/kit";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { weaponName } from "@/lib/weaponNames";
import { useEquipmentCatalogueStore } from "@/stores/equipmentCatalogue";
import type {
  ProficiencySlots,
  WeaponOption,
  WeaponPickInput,
  WeaponPool,
} from "@/types/character";
import {
  addWeapon,
  isSpecialization,
  removeWeapon,
  specialize,
  groupWeaponRows,
  weaponRows,
  type WeaponRow,
} from "./weapons";

/**
 * The weapon proficiency picker (ADR 0037 steps 3, 12 and 21), shared by the Skills tab's editor
 * and the creation's step: the weapon pool with its terms, the type picks left to redo (step
 * 21.8.3), then one row per catalogue weapon the class may pick (the derived `weapons`, named from
 * the equipment catalogue) and per kept pick it may not (shown as such, removable). A row adds or
 * removes the weapon; for a class that specializes, a picked weapon's toggle makes it the
 * specialization at the cost the options give. Every number is the server's: the pool comes from
 * its preview or checks (`figures`), a specialization's cost from the options; a refusal (a
 * second specialization, an overspend) is the server's, shown on the row or above the list.
 */
export function WeaponPicker({
  weapons,
  picks,
  onChange,
  figures,
  stale = false,
  errorFor,
}: {
  /** The class's weapons and their specialization cost (the derived `weapons`). */
  weapons: WeaponOption[];
  picks: WeaponPickInput[];
  onChange: (picks: WeaponPickInput[]) => void;
  /** The server's figures for the picks shown; null before its first answer. */
  figures: ProficiencySlots | null;
  /** True while the figures shown are not those of the picks (a refusal, a failed preview). */
  stale?: boolean;
  /** A refusal's words for a field (`weapon.<code>`, or `weapon` for the budget), or null. */
  errorFor: (field: string) => string | null;
}) {
  const { t, i18n } = useTranslation();
  // The rows are named from the equipment catalogue (ADR 0037 step 21.6), read once.
  const catalogue = useEquipmentCatalogueStore((s) => s.catalogue);
  const load = useEquipmentCatalogueStore((s) => s.load);
  useEffect(() => {
    void load().catch(() => undefined); // the codes name the rows meanwhile
  }, [load]);
  const nameOf = (code: string) =>
    weaponName(t, code, catalogue, i18n.language);
  const groups = groupWeaponRows(
    weaponRows(weapons, picks, figures?.weaponPicks),
    catalogue,
    i18n.language,
    t,
  );
  const budgetError = errorFor("weapon");
  const redo = figures?.weaponRedo ?? [];
  return (
    <div className="flex min-w-0 flex-col gap-3" data-testid="weapon-picker">
      {figures?.weapon && (
        <div
          className={cn("flex flex-col gap-2", stale && "opacity-60")}
          data-stale={stale ? "true" : undefined}
        >
          <WeaponPoolSummary pool={figures.weapon} />
          {stale && (
            <p className="text-label break-words text-ink-muted">
              {t("proficiencies.previewStale")}
            </p>
          )}
        </div>
      )}
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
      <div
        className="flex flex-col gap-2"
        role="group"
        aria-label={t("weaponProficiencies.list")}
        data-testid="weapon-rows"
      >
        {groups.map((g) => (
          <section
            key={g.key}
            className="flex min-w-0 flex-col gap-1"
            data-testid={g.key ? `weapon-group-${g.key}` : undefined}
          >
            {g.key && (
              <h4 className="px-2.5 text-label font-semibold text-tint">
                {g.label}
              </h4>
            )}
            <ul className="grid grid-cols-[repeat(auto-fill,minmax(20rem,1fr))] gap-x-4 gap-y-1">
              {g.rows.map((r) => (
                <WeaponRowItem
                  key={r.code}
                  row={r}
                  name={nameOf(r.code)}
                  error={errorFor(`weapon.${r.code}`)}
                  onAdd={() => onChange(addWeapon(picks, r.code))}
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
          </section>
        ))}
      </div>
    </div>
  );
}

/** The weapon pool as the server derives it (ADR 0037 step 3), with step 11's reminder or the
 * overspend's warning. */
export function WeaponPoolSummary({ pool }: { pool: WeaponPool }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-1" data-testid="weapon-pool">
      <p
        className="text-body break-words tabular-nums"
        data-testid="weapon-pool-terms"
      >
        {t("weaponProficiencies.pool", {
          initial: pool.initial,
          levels: pool.levelGained,
          perLevels: pool.perLevels,
          available: pool.available,
        })}
      </p>
      <p
        className="text-body break-words text-ink-muted tabular-nums"
        data-testid="weapon-pool-spent"
      >
        {t("proficiencies.spentLeft", { spent: pool.spent, left: pool.left })}
      </p>
      {pool.left > 0 && (
        <Note kind="info" testId="weapon-to-spend">
          {t("proficiencies.toSpend", { count: pool.left })}
        </Note>
      )}
      {pool.left < 0 && (
        <Note kind="warning" testId="weapon-over">
          {t("proficiencies.over", {
            spent: pool.spent,
            available: pool.available,
          })}
        </Note>
      )}
    </div>
  );
}

/** One catalogue weapon: its name, what the pick spends, and its controls. */
function WeaponRowItem({
  row,
  name,
  error,
  onAdd,
  onRemove,
  onSpecialize,
}: {
  row: WeaponRow;
  name: string;
  error: string | null;
  onAdd: () => void;
  onRemove: () => void;
  onSpecialize: (on: boolean) => void;
}) {
  const { t } = useTranslation();
  const { pick, option } = row;
  const specialized = isSpecialization(pick);
  const cost = option?.specializationSlots ?? 0;
  const status = !pick
    ? t("weaponProficiencies.unpicked")
    : specialized
      ? t("weaponProficiencies.specialization", { count: pick.slots })
      : t("weaponProficiencies.plain", { count: pick.slots });
  return (
    <li
      className={cn(
        "flex min-w-0 flex-col gap-1 rounded-md px-2.5 py-1",
        pick ? "bg-selected" : "bg-raised",
      )}
      data-testid={`weapon-row-${row.code}`}
      data-picked={pick ? "true" : undefined}
      data-specialized={specialized ? "true" : undefined}
    >
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
        <span className="flex min-w-0 flex-col">
          <span className="text-body break-words">{name}</span>
          <span
            className={cn(
              "text-label break-words",
              pick ? "text-muted-foreground" : "text-ink-muted",
            )}
            data-testid="weapon-row-status"
          >
            {status}
            {/* ADR 0037 step 21.5: the specialization's cost, the options' figure, beside a pick. */}
            {pick &&
              !specialized &&
              cost > 0 &&
              ` · ${t("weaponProficiencies.specializationCost", { count: cost })}`}
          </span>
          {!row.allowed && (
            <span
              className="text-label break-words text-warning"
              data-testid="weapon-row-not-allowed"
            >
              {t("weaponProficiencies.notAllowed")}
            </span>
          )}
        </span>
        <span className="flex flex-wrap items-center justify-end gap-1">
          {/* Specialize a picked weapon: its cost shows beside the pick (ADR 0037 step 21.5). */}
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
                : t("weaponProficiencies.specialize")}
            </Button>
          )}
          {pick ? (
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
          ) : (
            <Button
              type="button"
              size="xs"
              title={t("weaponProficiencies.addHint", { name })}
              aria-label={t("weaponProficiencies.addHint", { name })}
              onClick={onAdd}
              data-testid="weapon-add"
            >
              <Plus aria-hidden />
              {t("weaponProficiencies.add")}
            </Button>
          )}
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
