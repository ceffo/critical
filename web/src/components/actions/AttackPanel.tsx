import { Fragment, useEffect, useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import {
  ArrowsLeftRight,
  Minus,
  Plus,
  SlidersHorizontal,
  Sword,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useAttackStore } from "@/stores/attack";
import { useCharacterStore } from "@/stores/character";
import { useBuffsStore } from "@/stores/buffs";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { useSessionStore } from "@/stores/session";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { SizeChips } from "@/components/actions/SizeChips";
import { CritTableEditor } from "@/components/actions/CritTableEditor";
import { formatSigned } from "@/lib/format";
import { UNKNOWN_AC_STEP_BASE } from "@/lib/attackAdjust";
import { setModifiers, weaponSwitch } from "@/lib/weaponSwitch";
import { cn } from "@/lib/utils";
import type {
  AmmoChoice,
  AttackFormView,
  AttackPreview,
  Weapon,
} from "@/types/combat";

/** Same bound as the server's (combatsvc MaxTargetAC / MaxSituational); the server checks it. */
const BOUND = 20;
/** From this count down, an ammunition chip turns amber. */
const LOW_AMMO = 3;

/**
 * Weapons tab of the Act panel (ADR 0022 step 2.5, ADR 0027 step 7): the target bar on one line (AC,
 * Unknown, the set modifiers as tags, the toggle for size and situational), the weapon in hand as
 * one row (name, stats line, the 44 px Attack of step 3.3), its ammunition, then one button back to
 * the previous weapon and a drop-down with every other one. Every choice is saved on the server
 * (`/api/attack/form`) and every number shown comes from it; switching never attacks.
 */
export function AttackPanel() {
  const { t, i18n } = useTranslation();
  const {
    view,
    load,
    update,
    attack,
    pointBlank,
    setPointBlank,
    backstab,
    setBackstab,
  } = useAttackStore();
  const backstabMultiplier = useCharacterStore(
    (s) => s.character?.derived.backstab ?? 0,
  );
  const rolling = useRollStore((s) => s.rolling);
  const isAdmin = useSessionStore((s) => s.me?.role === "admin");
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
  const { form } = view;
  // ADR 0030 steps 12 and 19: with an option on, the server's preview with it is shown.
  const preview =
    backstab && view.preview?.backstab
      ? view.preview.backstab
      : pointBlank && view.preview?.pointBlank
        ? view.preview.pointBlank
        : view.preview;
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
  const { previous, others } = weaponSwitch(weapons, form);
  const nameOf = (w: Weapon) => localized(i18n.language, w.name);

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
            className="flex items-center gap-4 rounded-lg border-2 border-tint bg-selected px-4 py-3"
          >
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="section-label">{t("attack.inHand")}</span>
              <h3 className="text-key break-words" data-testid="weapon-in-hand">
                {localized(i18n.language, weapon.name)}
              </h3>
              {preview && (
                <StatsLine weapon={weapon} view={view} preview={preview} />
              )}
              {view.preview?.pointBlank && (
                <label
                  className="flex w-fit items-center gap-2 text-body"
                  title={t("attack.pointBlankHint")}
                >
                  <input
                    type="checkbox"
                    className="size-4"
                    checked={pointBlank}
                    onChange={(e) => setPointBlank(e.target.checked)}
                    data-testid="point-blank"
                  />
                  {t("attack.pointBlank")}
                </label>
              )}
              {view.preview?.backstab && (
                <label
                  className="flex w-fit items-center gap-2 text-body"
                  title={t("attack.backstabHint", {
                    n: backstabMultiplier,
                    // The +4 is the server's: the backstab modifier of its preview.
                    toHit: formatSigned(
                      view.preview.backstab.modifiers.find(
                        (m) => m.kind === "backstab",
                      )?.value ?? 0,
                    ),
                  })}
                >
                  <input
                    type="checkbox"
                    className="size-4"
                    checked={backstab}
                    onChange={(e) => setBackstab(e.target.checked)}
                    data-testid="backstab"
                  />
                  {t("attack.backstab", { n: backstabMultiplier })}
                </label>
              )}
            </div>
            <Button
              type="submit"
              variant="main"
              className="h-11 shrink-0 gap-0 py-0 pr-4 pl-0 whitespace-normal"
              disabled={rolling || !ready}
              aria-label={t("attack.roll")}
              aria-busy={rolling}
              aria-describedby={needs ? needsId : undefined}
            >
              <span
                aria-hidden
                className="grid size-11 shrink-0 place-items-center"
              >
                <Sword className="size-5" />
              </span>
              <span className="flex flex-col items-start text-left">
                <span className="text-title leading-4">{t("attack.roll")}</span>
                {needs && (
                  <span
                    id={needsId}
                    className="text-label leading-4"
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
            {t("attack.ammoSpent", {
              name: localized(i18n.language, ammo.name),
            })}
          </p>
        )}
        {(previous || others.length > 0) && (
          // ADR 0027 step 7.1: one click back to the previous weapon, the others in a drop-down.
          <div
            role="group"
            aria-label={t("attack.otherWeapons")}
            className="flex flex-wrap items-center gap-2"
            data-testid="other-weapons"
          >
            {previous && (
              <Button
                type="button"
                className="h-auto min-h-7 py-1 whitespace-normal"
                aria-label={t("attack.switchTo", { name: nameOf(previous) })}
                title={t("attack.switchTo", { name: nameOf(previous) })}
                data-weapon={nameOf(previous)}
                onClick={() => void update({ weaponId: previous.id })}
              >
                <ArrowsLeftRight aria-hidden />
                <span className="break-words">{nameOf(previous)}</span>
              </Button>
            )}
            {others.length > 0 && (
              <select
                aria-label={t("attack.otherWeapon")}
                title={t("attack.otherWeapon")}
                className={cn(fieldClass, "w-auto max-w-56")}
                value=""
                onChange={(e) => void update({ weaponId: e.target.value })}
              >
                <option value="" disabled>
                  {t("attack.otherWeaponPlaceholder")}
                </option>
                {others.map((w) => (
                  <option key={w.id} value={w.id}>
                    {nameOf(w)}
                  </option>
                ))}
              </select>
            )}
          </div>
        )}
      </form>
      {/* The crit table is one rule set for every user: only an admin edits it (the API refuses a player). */}
      {isAdmin && <CritTableEditor />}
    </div>
  );
}

/**
 * The target bar (ADR 0027 step 7.2) on one line: the TARGET label, the AC stepper and Unknown (every
 * turn), the set modifiers as tags (step 1.5: a large target, a situational modifier other than 0),
 * and a small toggle that shows size and situational under it (rare). The toggle is this screen's
 * state only; the choices themselves are the server's.
 */
function TargetBar({ view }: { view: AttackFormView }) {
  const { t } = useTranslation();
  const update = useAttackStore((s) => s.update);
  const [open, setOpen] = useState(false);
  const modifiersId = useId();
  const { targetAc, targetSize, situational } = view.form;
  const set = setModifiers(view.form);
  return (
    <div
      role="group"
      aria-label={t("attack.target")}
      className="flex flex-col gap-2 rounded-lg bg-raised px-3 py-1.5"
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
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
        <SizeChips
          value={targetSize}
          label={t("attack.targetSize")}
          names={{ sm: t("attack.sizes.sm"), l: t("attack.sizes.l") }}
          shorts={{
            sm: t("attack.sizesShort.sm"),
            l: t("attack.sizesShort.l"),
          }}
          onChange={(size) => void update({ targetSize: size })}
        />
        {/* The toggle stays on the line; the tag of a set situational modifier follows it and
            wraps to a second row when the panel is too narrow. */}
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="ml-auto"
          aria-expanded={open}
          aria-controls={modifiersId}
          aria-label={t("attack.modifiers")}
          title={t("attack.modifiers")}
          onClick={() => setOpen(!open)}
        >
          <SlidersHorizontal />
        </Button>
        {set.situational && (
          <span
            className="chip px-2 py-0.5 text-body whitespace-nowrap"
            title={set.situational.label.trim() || undefined}
            data-testid="modifier-tag"
          >
            {t("attack.tags.situational", {
              value: formatSigned(set.situational.value),
            })}
          </span>
        )}
      </div>
      {open && (
        <div
          id={modifiersId}
          className="flex flex-wrap items-center gap-x-4 gap-y-2 pb-1"
        >
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
function StatsLine({
  weapon,
  view,
  preview,
}: {
  weapon: Weapon;
  view: AttackFormView;
  preview: AttackPreview;
}) {
  const { t } = useTranslation();
  const rate = view.attackRate;
  const size = view.form.targetSize;
  // Each segment stays on one line; the line breaks between segments when the row is narrow.
  const segments: ReactNode[] = [
    t(`attack.kinds.${weapon.kind}`),
    weapon.two_handed && t("attack.twoHanded"),
    weapon.specialized && (
      <span data-testid="tile-specialized">{t("attack.specialized")}</span>
    ),
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
    // ADR 0030 step 11: the rate, in the PHB's notation, and this round's count for a half rate.
    rate && (
      <span
        data-testid="tile-rate"
        title={t("attack.rateHint", { ...rate, count: rate.thisRound })}
      >
        {t(rate.rounds > 1 ? "attack.rateRound" : "attack.rate", {
          ...rate,
          count: rate.thisRound,
        })}
      </span>
    ),
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
  const { t, i18n } = useTranslation();
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
          aria-label={t("attack.ammoOption", {
            name: localized(i18n.language, a.name),
            count: a.count,
          })}
          data-item={a.itemId}
          className={cn(
            "flex min-h-7 items-center gap-2 px-3 py-0.5 text-left text-body",
            a.itemId === chosen ? "chip-chosen" : "chip",
          )}
          onClick={() => void update({ ammoItemId: a.itemId })}
        >
          <span className="break-words">
            {localized(i18n.language, a.name)}
          </span>
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
