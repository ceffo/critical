import { Fragment, useEffect, useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ArrowsLeftRight, Minus, Plus, Sword } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useAttackStore } from "@/stores/attack";
import { useBuffsStore } from "@/stores/buffs";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { CritTableEditor } from "@/components/actions/CritTableEditor";
import { formatSigned } from "@/lib/format";
import { UNKNOWN_AC_STEP_BASE } from "@/lib/attackAdjust";
import { cn } from "@/lib/utils";
import type {
  AmmoChoice,
  AttackFormView,
  TargetSize,
  Weapon,
} from "@/types/combat";

const SIZES: TargetSize[] = ["sm", "l"];
/** Same bound as the server's (combatsvc MaxTargetAC / MaxSituational); the server checks it. */
const BOUND = 20;
/** From this count down, an ammunition chip turns amber. */
const LOW_AMMO = 3;

/**
 * Weapons tab of the Act panel (ADR 0022 step 2.5): the target bar (AC, size, situational modifier),
 * the weapon in hand as one row (name, stats line, the 44 px Attack of step 3.3), its ammunition, and
 * the other weapons as chips. Every choice is saved on the server (`/api/attack/form`) and every
 * number shown comes from it; switching never attacks.
 */
export function AttackPanel() {
  const { t } = useTranslation();
  const { view, load, update, attack } = useAttackStore();
  const rolling = useRollStore((s) => s.rolling);
  const weapons = useItemsStore((s) => s.weapons);
  const items = useItemsStore((s) => s.items);
  const buffs = useBuffsStore((s) => s.buffs);
  const strength = useBuffsStore((s) => s.strength);
  const needsId = useId();

  // The preview depends on buffs, effective STR, item counts and the weapons: reload with them.
  useEffect(() => {
    void load();
  }, [load, weapons, items, buffs, strength]);

  if (!view || weapons.length === 0) {
    return (
      <p className="text-body text-muted-foreground">{t("attack.loading")}</p>
    );
  }
  const { form, preview } = view;
  const weapon = weapons.find((w) => w.id === form.weaponId);
  const needsAmmo = Boolean(weapon?.ammo_kind);
  const ammo = view.ammoChoices.find((a) => a.itemId === form.ammoItemId);
  const ammoSpent = ammo !== undefined && ammo.count === 0;
  const ready =
    preview !== null &&
    weapon !== undefined &&
    (!needsAmmo || (ammo && !ammoSpent));
  const needs =
    preview === null
      ? null
      : preview.rollNeeded === null || form.targetAc === null
        ? t("attack.needsUnknown")
        : t("attack.needs", { ac: form.targetAc, n: preview.rollNeeded });
  // The other weapons as chips, the previous one first (one click switches back).
  const others = weapons
    .filter((w) => w.id !== form.weaponId)
    .sort(
      (a, b) =>
        Number(b.id === form.previousWeaponId) -
        Number(a.id === form.previousWeaponId),
    );

  return (
    <div className="flex flex-col gap-3">
      <form
        className="flex flex-col gap-3"
        aria-label={t("attack.form")}
        onSubmit={(e) => {
          e.preventDefault();
          if (ready) void attack();
        }}
      >
        <TargetBar view={view} />
        {weapon && (
          // ADR 0022 step 2.5: the weapon in hand is one row: name, stats on one line, and Attack,
          // the only control in the row (step 3.3).
          <section
            aria-label={t("attack.inHand")}
            className="flex items-center gap-4 rounded-lg border border-amb-700 bg-amb-900/40 px-4 py-3"
          >
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="section-label">{t("attack.inHand")}</span>
              <h3 className="text-key break-words" data-testid="weapon-in-hand">
                {weapon.name}
              </h3>
              {preview && <StatsLine weapon={weapon} view={view} />}
            </div>
            <Button
              type="submit"
              className="h-11 shrink-0 gap-0 py-0 pr-4 pl-0 whitespace-normal"
              disabled={rolling || !ready}
              aria-label={rolling ? t("attack.rolling") : t("attack.roll")}
              aria-describedby={needs ? needsId : undefined}
            >
              <span
                aria-hidden
                className="grid size-11 shrink-0 place-items-center"
              >
                <Sword className="size-5" />
              </span>
              <span className="flex flex-col items-start text-left">
                <span className="text-title leading-4">
                  {rolling ? t("attack.rolling") : t("attack.roll")}
                </span>
                {needs && (
                  <span
                    id={needsId}
                    className="text-label leading-4 opacity-80"
                    data-testid="attack-needs"
                  >
                    {needs}
                  </span>
                )}
              </span>
            </Button>
          </section>
        )}
        {weapon && needsAmmo && (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            <span className="section-label">{t("attack.ammo")}</span>
            <AmmoChips choices={view.ammoChoices} chosen={form.ammoItemId} />
          </div>
        )}
        {ammoSpent && (
          <p className="text-body text-danger" data-testid="ammo-spent">
            {t("attack.ammoSpent", { name: ammo.name })}
          </p>
        )}
        {others.length > 0 && (
          <div
            role="group"
            aria-label={t("attack.otherWeapons")}
            className="flex flex-wrap items-center gap-1.5"
            data-testid="other-weapons"
          >
            <span className="mr-1 section-label">
              {t("attack.switchLabel")}
            </span>
            {others.map((w) => (
              <Button
                key={w.id}
                type="button"
                variant="outline"
                className="h-auto min-h-7 py-1 whitespace-normal"
                aria-label={t("attack.switchTo", { name: w.name })}
                data-weapon={w.name}
                onClick={() => void update({ weaponId: w.id })}
              >
                {w.id === form.previousWeaponId && (
                  <ArrowsLeftRight aria-hidden />
                )}
                <span className="break-words">{w.name}</span>
                <span className="text-muted-foreground tabular-nums">
                  {w.damage_sm}
                </span>
              </Button>
            ))}
          </div>
        )}
      </form>
      <CritTableEditor />
    </div>
  );
}

/**
 * The target bar on one line (it wraps when narrow): the TARGET label, AC, size and the situational
 * modifier with its reason (shown when not 0). Field labels sit beside their control.
 */
function TargetBar({ view }: { view: AttackFormView }) {
  const { t } = useTranslation();
  const update = useAttackStore((s) => s.update);
  const { targetAc, targetSize, situational } = view.form;
  return (
    <div
      role="group"
      aria-label={t("attack.target")}
      className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg bg-neu-900 px-3 py-2.5"
    >
      <span className="section-label">{t("attack.target")}</span>
      <div className="flex items-center gap-1">
        <Stepper
          label={t("attack.targetAc")}
          shortLabel={t("attack.acShort")}
          lower={t("attack.lowerAc")}
          raise={t("attack.raiseAc")}
          value={targetAc}
          placeholder={t("attack.unknownAcPlaceholder")}
          emptyBase={UNKNOWN_AC_STEP_BASE}
          max={view.maxRealAc}
          onChange={(v) => void update({ targetAc: v })}
          onClear={() => void update({ targetAc: null })}
        />
        {targetAc !== null && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            title={t("attack.unknownAcHint")}
            onClick={() => void update({ targetAc: null })}
          >
            {t("attack.unknownAc")}
          </Button>
        )}
      </div>
      <div className="flex items-center gap-2">
        <span aria-hidden className="text-body text-muted-foreground">
          {t("attack.sizeShort")}
        </span>
        <div
          role="radiogroup"
          aria-label={t("attack.targetSize")}
          className="flex rounded-lg bg-background p-0.5"
        >
          {SIZES.map((s) => (
            <button
              key={s}
              type="button"
              role="radio"
              aria-checked={targetSize === s}
              aria-label={t(`attack.sizes.${s}`)}
              className={cn(
                "h-6 rounded-md px-3 text-body whitespace-nowrap text-muted-foreground",
                targetSize === s && "bg-amb-800 text-foreground",
              )}
              onClick={() => void update({ targetSize: s })}
            >
              {t(`attack.sizesShort.${s}`)}
            </button>
          ))}
        </div>
      </div>
      <Stepper
        label={t("attack.situational")}
        lower={t("attack.lowerSituational")}
        raise={t("attack.raiseSituational")}
        value={situational.value}
        signed
        onChange={(v) =>
          void update({ situational: { ...situational, value: v } })
        }
      />
      {situational.value !== 0 && (
        <label className="flex min-w-48 flex-1 items-center gap-2">
          <span className="text-body text-muted-foreground">
            {t("attack.situationalLabel")}
          </span>
          <input
            className={fieldClass}
            type="text"
            placeholder={t("attack.situationalPlaceholder")}
            value={situational.label}
            onChange={(e) =>
              void update({
                situational: { ...situational, label: e.target.value },
              })
            }
          />
        </label>
      )}
    </div>
  );
}

/**
 * − / + around a whole number in ±BOUND; typing is kept as text until it is a number. A null value
 * shows an empty field with the placeholder, − / + then start from emptyBase; clearing the field
 * calls onClear when given.
 */
function Stepper({
  label,
  shortLabel,
  lower,
  raise,
  value: current,
  signed = false,
  placeholder,
  emptyBase = 0,
  max = BOUND,
  onChange,
  onClear,
}: {
  label: string;
  /** Shown beside the field instead of `label`, which stays the field's accessible name. */
  shortLabel?: string;
  lower: string;
  raise: string;
  value: number | null;
  signed?: boolean;
  placeholder?: string;
  emptyBase?: number;
  /** Upper bound, when below BOUND (the target AC stops at the highest real AC). */
  max?: number;
  onChange: (v: number) => void;
  onClear?: () => void;
}) {
  const id = useId();
  const [draft, setDraft] = useState<string | null>(null);
  const value = current ?? emptyBase;
  const shown =
    draft ??
    (current === null
      ? ""
      : signed && current > 0
        ? `+${current}`
        : String(current));
  const set = (v: number) => {
    setDraft(null);
    onChange(Math.max(-BOUND, Math.min(max, v)));
  };
  return (
    <div className="flex items-center gap-2">
      <label
        htmlFor={id}
        className="text-body whitespace-nowrap text-muted-foreground"
      >
        {shortLabel ?? label}
      </label>
      <span className="flex items-center gap-0.5">
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={lower}
          disabled={value <= -BOUND}
          onClick={() => set(value - 1)}
        >
          <Minus />
        </Button>
        <input
          id={id}
          aria-label={label}
          className={cn(fieldClass, "w-12 text-center")}
          inputMode="numeric"
          value={shown}
          placeholder={placeholder}
          onChange={(e) => {
            const text = e.target.value;
            setDraft(text);
            const n = Number(text.trim());
            if (/^[+-]?\d+$/.test(text.trim()) && n >= -BOUND && n <= max)
              onChange(n);
            else if (text.trim() === "" && onClear) onClear();
          }}
          onBlur={() => setDraft(null)}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={raise}
          disabled={value >= max}
          onClick={() => set(value + 1)}
        >
          <Plus />
        </Button>
      </span>
    </div>
  );
}

/**
 * The weapon's numbers on one line of text (ADR 0022 step 2.5), from the server's preview (ADR 0002
 * step 12): "Melee · 1d8 S/M · 1d12 L · +2 / +2 to hit / dmg · THAC0 11", the size not in use dimmed.
 */
function StatsLine({ weapon, view }: { weapon: Weapon; view: AttackFormView }) {
  const { t } = useTranslation();
  const preview = view.preview;
  if (!preview) return null;
  const size = view.form.targetSize;
  // Each segment stays on one line; the line breaks between segments when the row is narrow.
  const segments: ReactNode[] = [
    t(`attack.kinds.${weapon.kind}`),
    weapon.two_handed && t("attack.twoHanded"),
    <span className={cn(size !== "sm" && "opacity-60")}>
      <Value testId="tile-damage-sm">{weapon.damage_sm}</Value>{" "}
      {t("attack.line.sm")}
    </span>,
    <span className={cn(size !== "l" && "opacity-60")}>
      <Value testId="tile-damage-l">
        {weapon.damage_l || weapon.damage_sm}
      </Value>{" "}
      {t("attack.line.l")}
    </span>,
    <>
      <Value testId="tile-bonus">
        {`${formatSigned(preview.toHit)} / ${formatSigned(preview.damageBonus)}`}
      </Value>{" "}
      {t("attack.line.bonus")}
    </>,
    <>
      {t("attack.line.thac0")}{" "}
      <Value testId="tile-thac0">{preview.effectiveThac0}</Value>
    </>,
  ].filter(Boolean);
  return (
    <p className="text-body text-muted-foreground" data-testid="weapon-stats">
      {segments.map((seg, i) => (
        <Fragment key={i}>
          {i > 0 && " · "}
          <span className="whitespace-nowrap">{seg}</span>
        </Fragment>
      ))}
    </p>
  );
}

function Value({ testId, children }: { testId: string; children: ReactNode }) {
  return (
    <b
      className="font-semibold whitespace-nowrap text-foreground tabular-nums"
      data-testid={testId}
    >
      {children}
    </b>
  );
}

/** The weapon's ammunition as chips with their counts; the chosen one is checked. */
function AmmoChips({
  choices,
  chosen,
}: {
  choices: AmmoChoice[];
  chosen: string;
}) {
  const { t } = useTranslation();
  const update = useAttackStore((s) => s.update);
  if (choices.length === 0) {
    return <p className="text-body text-danger">{t("attack.noAmmo")}</p>;
  }
  return (
    <div
      role="radiogroup"
      aria-label={t("attack.ammo")}
      className="flex flex-wrap gap-2"
    >
      {choices.map((a) => (
        <button
          key={a.itemId}
          type="button"
          role="radio"
          aria-checked={a.itemId === chosen}
          aria-label={t("attack.ammoOption", { name: a.name, count: a.count })}
          data-item={a.itemId}
          className={cn(
            "flex min-h-7 items-center gap-2 rounded-full bg-neu-900 px-3 py-0.5 text-left text-body",
            a.itemId === chosen && "bg-amb-800 ring-1 ring-amb-500",
          )}
          onClick={() => void update({ ammoItemId: a.itemId })}
        >
          <span className="break-words">{a.name}</span>
          <span
            data-testid="ammo-count"
            className={cn(
              "font-medium tabular-nums",
              a.count === 0
                ? "text-danger"
                : a.count <= LOW_AMMO && "text-warning",
            )}
          >
            {a.count}
          </span>
        </button>
      ))}
    </div>
  );
}
