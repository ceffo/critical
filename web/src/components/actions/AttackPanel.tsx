import { useEffect, useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ArrowsLeftRight, CaretDown, Minus, Plus, Sword } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useAttackStore } from "@/stores/attack";
import { useBuffsStore } from "@/stores/buffs";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { CritTableEditor } from "@/components/actions/CritTableEditor";
import { formatSigned } from "@/lib/format";
import { UNKNOWN_AC_STEP_BASE } from "@/lib/attackAdjust";
import { cn } from "@/lib/utils";
import type { AmmoChoice, AttackFormView, TargetSize, Weapon } from "@/types/combat";

const SIZES: TargetSize[] = ["sm", "l"];
/** Same bound as the server's (combatsvc MaxTargetAC / MaxSituational); the server checks it. */
const BOUND = 20;
/** From this count down, an ammunition chip turns amber. */
const LOW_AMMO = 3;

/**
 * Weapons tab of the Agir workspace: the target bar (AC, size, situational modifier), the weapon in
 * hand with its numbers and ammunition, and quick weapon switching. Every choice is saved on the
 * server (`/api/attack/form`) and every number shown comes from it; switching never attacks.
 */
export function AttackPanel() {
  const { t } = useTranslation();
  const { view, load, update, attack } = useAttackStore();
  const rolling = useRollStore((s) => s.rolling);
  const weapons = useItemsStore((s) => s.weapons);
  const items = useItemsStore((s) => s.items);
  const buffs = useBuffsStore((s) => s.buffs);
  const strength = useBuffsStore((s) => s.strength);

  // The preview depends on buffs, effective STR, item counts and the weapons: reload with them.
  useEffect(() => {
    void load();
  }, [load, weapons, items, buffs, strength]);

  if (!view || weapons.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("attack.loading")}</p>;
  }
  const { form, preview } = view;
  const weapon = weapons.find((w) => w.id === form.weaponId);
  const previous = weapons.find((w) => w.id === form.previousWeaponId);
  const needsAmmo = Boolean(weapon?.ammo_kind);
  const ammo = view.ammoChoices.find((a) => a.itemId === form.ammoItemId);
  const ammoSpent = ammo !== undefined && ammo.count === 0;
  const ready = preview !== null && weapon !== undefined && (!needsAmmo || (ammo && !ammoSpent));

  return (
    <div className="flex flex-col gap-4">
      <form
        className="flex flex-col gap-4"
        aria-label={t("attack.form")}
        onSubmit={(e) => {
          e.preventDefault();
          if (ready) void attack();
        }}
      >
        <TargetBar view={view} />
        {weapon && (
          <section
            aria-label={t("attack.inHand")}
            className="@container flex flex-col gap-4 rounded-xl bg-amb-900/40 p-4 ring-1 ring-amb-700"
          >
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <p className="text-sm text-muted-foreground">{t("attack.inHand")}</p>
                <h3 className="truncate text-xl font-medium" data-testid="weapon-in-hand">
                  {weapon.name}
                </h3>
                <p className="text-sm text-muted-foreground">
                  {t(`attack.kinds.${weapon.kind}`)}
                  {weapon.two_handed && ` · ${t("attack.twoHanded")}`}
                </p>
              </div>
              <Button
                type="submit"
                size="lg"
                className="h-12 shrink-0 px-6 text-base"
                disabled={rolling || !ready}
              >
                <Sword />
                {rolling ? t("attack.rolling") : t("attack.roll")}
              </Button>
            </div>
            {preview && <Tiles weapon={weapon} view={view} />}
            {needsAmmo && <AmmoChips choices={view.ammoChoices} chosen={form.ammoItemId} />}
            {ammoSpent && (
              <p className="text-sm text-danger" data-testid="ammo-spent">
                {t("attack.ammoSpent", { name: ammo.name })}
              </p>
            )}
          </section>
        )}
        <div className="flex flex-wrap items-center gap-2">
          {previous && (
            <Button
              type="button"
              variant="secondary"
              onClick={() => void update({ weaponId: previous.id })}
            >
              <ArrowsLeftRight />
              {t("attack.switchTo", { name: previous.name })}
            </Button>
          )}
          <OtherWeapon
            weapons={weapons.filter((w) => w.id !== form.weaponId)}
            onPick={(id) => void update({ weaponId: id })}
          />
        </div>
      </form>
      <CritTableEditor />
    </div>
  );
}

/** Target AC, size and the situational modifier with its reason (shown when not 0). */
function TargetBar({ view }: { view: AttackFormView }) {
  const { t } = useTranslation();
  const update = useAttackStore((s) => s.update);
  const { targetAc, targetSize, situational } = view.form;
  const sizeLabel = useId();
  return (
    <div
      role="group"
      aria-label={t("attack.target")}
      className="flex flex-wrap items-end gap-x-6 gap-y-3 rounded-lg bg-neu-900 px-4 py-3"
    >
      <div className="flex items-end gap-1">
        <Stepper
          label={t("attack.targetAc")}
          lower={t("attack.lowerAc")}
          raise={t("attack.raiseAc")}
          value={targetAc}
          placeholder={t("attack.unknownAcPlaceholder")}
          emptyBase={UNKNOWN_AC_STEP_BASE}
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
      <div className="flex flex-col gap-1">
        <span id={sizeLabel} className="text-xs text-muted-foreground">
          {t("attack.targetSize")}
        </span>
        <div
          role="radiogroup"
          aria-labelledby={sizeLabel}
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
                "h-7 rounded-md px-3 text-sm text-muted-foreground",
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
        onChange={(v) => void update({ situational: { ...situational, value: v } })}
      />
      {situational.value !== 0 && (
        <label className="flex min-w-40 flex-1 flex-col gap-1">
          <span className="text-xs text-muted-foreground">{t("attack.situationalLabel")}</span>
          <input
            className={fieldClass}
            type="text"
            placeholder={t("attack.situationalPlaceholder")}
            value={situational.label}
            onChange={(e) =>
              void update({ situational: { ...situational, label: e.target.value } })
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
  lower,
  raise,
  value: current,
  signed = false,
  placeholder,
  emptyBase = 0,
  onChange,
  onClear,
}: {
  label: string;
  lower: string;
  raise: string;
  value: number | null;
  signed?: boolean;
  placeholder?: string;
  emptyBase?: number;
  onChange: (v: number) => void;
  onClear?: () => void;
}) {
  const id = useId();
  const [draft, setDraft] = useState<string | null>(null);
  const value = current ?? emptyBase;
  const shown =
    draft ?? (current === null ? "" : signed && current > 0 ? `+${current}` : String(current));
  const set = (v: number) => {
    setDraft(null);
    onChange(Math.max(-BOUND, Math.min(BOUND, v)));
  };
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-xs text-muted-foreground">
        {label}
      </label>
      <span className="flex items-center gap-1">
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
          className={cn(fieldClass, "w-14 text-center")}
          inputMode="numeric"
          value={shown}
          placeholder={placeholder}
          onChange={(e) => {
            const text = e.target.value;
            setDraft(text);
            const n = Number(text.trim());
            if (/^[+-]?\d+$/.test(text.trim()) && Math.abs(n) <= BOUND) onChange(n);
            else if (text.trim() === "" && onClear) onClear();
          }}
          onBlur={() => setDraft(null)}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={raise}
          disabled={value >= BOUND}
          onClick={() => set(value + 1)}
        >
          <Plus />
        </Button>
      </span>
    </div>
  );
}

/** Damage dice per size (the current one highlighted), to-hit / damage bonus, effective THAC0. */
function Tiles({ weapon, view }: { weapon: Weapon; view: AttackFormView }) {
  const { t } = useTranslation();
  const preview = view.preview;
  if (!preview) return null;
  const size = view.form.targetSize;
  return (
    <dl className="grid grid-cols-2 gap-2 @xl:grid-cols-4">
      <Tile label={t("attack.tiles.damageSm")} testId="tile-damage-sm" dim={size !== "sm"} current={size === "sm"}>
        {weapon.damage_sm}
      </Tile>
      <Tile label={t("attack.tiles.damageL")} testId="tile-damage-l" dim={size !== "l"} current={size === "l"}>
        {weapon.damage_l || weapon.damage_sm}
      </Tile>
      <Tile label={t("attack.tiles.bonus")} testId="tile-bonus">
        {`${formatSigned(preview.toHit)} / ${formatSigned(preview.damageBonus)}`}
      </Tile>
      <Tile
        label={t("attack.tiles.thac0")}
        testId="tile-thac0"
        hint={
          preview.rollNeeded === null || view.form.targetAc === null
            ? t("attack.tiles.rollNeededUnknown")
            : t("attack.tiles.rollNeeded", { ac: view.form.targetAc, n: preview.rollNeeded })
        }
      >
        {preview.effectiveThac0}
      </Tile>
    </dl>
  );
}

function Tile({
  label,
  testId,
  hint,
  dim = false,
  current = false,
  children,
}: {
  label: string;
  testId: string;
  hint?: string;
  dim?: boolean;
  current?: boolean;
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-0.5 rounded-lg bg-background/60 px-3 py-2",
        current && "ring-1 ring-amb-600",
        dim && "opacity-60",
      )}
    >
      <dt className="truncate text-xs text-muted-foreground">{label}</dt>
      <dd className="text-lg font-medium" data-testid={testId}>
        {children}
      </dd>
      {hint && <dd className="truncate text-xs text-muted-foreground">{hint}</dd>}
    </div>
  );
}

/** The weapon's ammunition as chips with their counts; the chosen one is checked. */
function AmmoChips({ choices, chosen }: { choices: AmmoChoice[]; chosen: string }) {
  const { t } = useTranslation();
  const update = useAttackStore((s) => s.update);
  if (choices.length === 0) {
    return <p className="text-sm text-danger">{t("attack.noAmmo")}</p>;
  }
  return (
    <div role="radiogroup" aria-label={t("attack.ammo")} className="flex flex-wrap gap-2">
      {choices.map((a) => (
        <button
          key={a.itemId}
          type="button"
          role="radio"
          aria-checked={a.itemId === chosen}
          aria-label={t("attack.ammoOption", { name: a.name, count: a.count })}
          data-item={a.itemId}
          className={cn(
            "flex h-8 items-center gap-2 rounded-full bg-background/60 px-3 text-sm",
            a.itemId === chosen && "bg-amb-800 ring-1 ring-amb-500",
          )}
          onClick={() => void update({ ammoItemId: a.itemId })}
        >
          <span>{a.name}</span>
          <span
            data-testid="ammo-count"
            className={cn(
              "font-medium",
              a.count === 0 ? "text-danger" : a.count <= LOW_AMMO && "text-warning",
            )}
          >
            {a.count}
          </span>
        </button>
      ))}
    </div>
  );
}

/** "Other weapon": a popover listing the weapons not in hand; picking one switches, never attacks. */
function OtherWeapon({ weapons, onPick }: { weapons: Weapon[]; onPick: (id: string) => void }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  if (weapons.length === 0) return null;
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button type="button" variant="ghost">
            {t("attack.otherWeapon")}
            <CaretDown />
          </Button>
        }
      />
      <PopoverContent align="start" className="w-72 p-1.5">
        <ul aria-label={t("attack.otherWeapon")} className="flex flex-col">
          {weapons.map((w) => (
            <li key={w.id}>
              <button
                type="button"
                className="flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm hover:bg-neu-900"
                onClick={() => {
                  setOpen(false);
                  onPick(w.id);
                }}
              >
                <span className="truncate">{w.name}</span>
                <span className="shrink-0 text-muted-foreground">
                  {t("attack.weaponDice", { sm: w.damage_sm, l: w.damage_l || w.damage_sm })}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  );
}
