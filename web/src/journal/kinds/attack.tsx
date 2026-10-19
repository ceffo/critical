// The attack entry (combatsvc.AttackBreakdown, ADR 0002, 0003, 0012): the payload in effect is the
// latest correction's (target AC, the DM's verdict).
import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Sword } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { useAttackStore } from "@/stores/attack";
import { useItemsStore } from "@/stores/items";
import { useJournalStore } from "@/stores/journal";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { formatFactor, formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { itemNamer } from "@/lib/itemPowers";
import {
  MAX_TARGET_AC,
  UNKNOWN_AC_STEP_BASE,
  acCorrectionInEffect,
  adjustFormKey,
  attackSettled,
  hitsAc,
  parseTargetAc,
  stepTargetAc,
  tellGmWithCrit,
} from "@/lib/attackAdjust";
import {
  type InventoryNamer,
  recordedNames,
  useInventoryName,
} from "@/i18n/inventoryName";
import type { RollResult } from "@/types/combat";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { chips, type DieChipView } from "../dice";
import { formatMultiplier, modLabel, plusTerm } from "../words";
import {
  AdjustFold,
  DieChip,
  Fact,
  HexDie,
  ResultLine,
  Total,
  Verdict,
  labelClass,
} from "../parts";

/** The attack's payload in effect (a corrected attack's is the latest amend's), with its entry id
 * for the corrections. */
export function decodeAttack(e: LogEntry): RollResult | null {
  const p = (e.effectivePayload ?? e.payload) as Partial<RollResult> | null;
  return p && typeof p.hit === "boolean" && typeof p.weaponName === "string"
    ? { ...(p as RollResult), logEntryId: e.id }
    : null;
}

/**
 * An attack's weapon, with its ammunition when it fired one: "Short Bow +2 with Arrows +1". Both are
 * named by id in the player's language while the inventory has them (ADR 0023 step 12).
 */
export function attackWeapon(
  p: Pick<RollResult, "weaponId" | "weaponName" | "ammo">,
  t: TFunction,
  names: InventoryNamer = recordedNames,
): string {
  const weapon = names.weapon(p.weaponId, p.weaponName ?? "");
  return p.ammo
    ? t("log.attack.withAmmo", {
        weapon,
        ammo: names.item(p.ammo.itemId, p.ammo.name),
      })
    : weapon;
}

function attackText(
  p: RollResult,
  t: TFunction,
  names: InventoryNamer,
): string {
  const weapon = attackWeapon(p, t, names);
  // Against an unknown AC (docs/adr/0012): the AC it hits until the DM settles it, then the DM's
  // verdict (no crit).
  if (p.targetAc === null) {
    const hits = p.hitsAc ?? p.targetACNeeded;
    if (p.verdict === undefined)
      return t("log.attack.unknown", { weapon, hits });
    return p.verdict === "hit"
      ? t("log.attack.verdictHit", { weapon, damage: p.totalDamage })
      : t("log.attack.verdictMiss", { weapon });
  }
  const args = {
    weapon,
    ac: p.targetAc,
    damage: p.totalDamage,
    mult: p.critMultiplier,
    check: p.d100CritRoll,
    roll: p.d100MultiplierRoll,
  };
  // Attacks logged before the multiplier d100 existed keep the line without the rolls.
  const crit =
    p.d100MultiplierRoll === undefined
      ? "log.attack.crit"
      : "log.attack.critRolls";
  const text = !p.hit
    ? t("log.attack.miss", args)
    : t(p.isCrit ? crit : "log.attack.hit", args);
  if (p.rolledTargetAc === undefined || p.rolledHit === undefined) return text;
  const outcome = (hit: boolean, crit: boolean | undefined) =>
    t(`log.attack.outcome.${crit ? "crit" : hit ? "hit" : "miss"}`);
  return t("log.attack.corrected", {
    text,
    from: p.rolledTargetAc,
    to: p.targetAc,
    was: outcome(p.rolledHit, p.rolledCrit),
    now: outcome(p.hit, p.isCrit),
  });
}

/**
 * The damage formula of a hit with its numbers (docs/adr/0003 step 2):
 * ((standard × spells [× backstab]) + weapon) × crit + strength [± bonus] = total, where the
 * standard damage is the dice [× dice multiplier], in parentheses with the damage's flat part
 * when it has one. A result the server floored (or clamped at 0) shows its unrounded value
 * first: "= 10.5 → 10".
 */
export function damageFormula(r: RollResult, lang?: string): string {
  const m = (n: number) => formatMultiplier(n, lang);
  const shown = (f: number | undefined): f is number =>
    f !== undefined && f > 0 && f !== 1;
  let standard = `${r.baseDamage}`;
  if (shown(r.diceMultiplier)) standard += ` × ${m(r.diceMultiplier)}`;
  if (r.diceModifier) standard = `(${standard}${plusTerm(r.diceModifier)})`;
  let dice = `${standard} × ${m(r.spellsBuff)}`;
  if (shown(r.backstabMultiplier)) dice += ` × ${m(r.backstabMultiplier)}`;
  let s = `((${dice}) + ${r.weaponBuff}) × ${m(r.multipliers)} + ${r.strengthBonus}`;
  if (r.bonusDamage !== 0) s += plusTerm(r.bonusDamage);
  const unrounded = r.unroundedDamage;
  if (unrounded !== undefined && Math.abs(unrounded - r.totalDamage) > 1e-9) {
    return `${s} = ${new Intl.NumberFormat(lang, { maximumFractionDigits: 4 }).format(unrounded)} → ${r.totalDamage}`;
  }
  return `${s} = ${r.totalDamage}`;
}

/**
 * The attack's damage dice as chips, from the roll as recorded (the payload or the roll endpoint);
 * null when it has none (a miss, or an attack logged before they were recorded): the card then
 * shows "dice -> total".
 */
export function attackDiceChips(
  r: Pick<RollResult, "damageRolls">,
): DieChipView[] | null {
  return r.damageRolls && r.damageRolls.length > 0
    ? chips(r.damageRolls)
    : null;
}

export const attackKind: KindView<RollResult, RollResult> = {
  decode: decodeAttack,
  icon: Sword,
  text: (p, w) => attackText(p, w.t, w.names),
  value(p) {
    if (!attackSettled(p)) return { text: "?", tone: "muted" };
    return p.hit
      ? { text: String(p.totalDamage), tone: p.isCrit ? "crit" : "ink" }
      : { text: "–", tone: "muted" };
  },
  rolled: () => true,
  subtitle: (p, w) => attackWeapon(p, w.t, w.names),
  short(p, { t }) {
    const settled = attackSettled(p);
    return {
      // The d20, as the card's roll; the damage is the result card's business.
      value: { text: String(p.d20Roll), tone: "muted" },
      what: t("lastShort.attack"),
      verdict: !settled
        ? null
        : p.hit
          ? p.isCrit
            ? { text: t("lastShort.verdict.crit"), tone: "crit" }
            : { text: t("lastShort.verdict.hit"), tone: "pass" }
          : { text: t("lastShort.verdict.miss"), tone: "fail" },
    };
  },
  card: ({ payload, interactive }) => (
    <AttackCard r={payload} interactive={interactive} />
  ),
  live: (r) => r,
};

/** "Crit check 7 · multiplier 63 → ×3 (51–90)"; entries without the multiplier d100 show the old text. */
function critText(t: TFunction, r: RollResult): string {
  const placeholder = r.critPlaceholder || r.critFrom === undefined;
  if (r.d100MultiplierRoll === undefined) {
    return placeholder
      ? t("result.critPlaceholderLegacy", { mult: r.critMultiplier })
      : t("result.critLegacy", {
          mult: r.critMultiplier,
          from: r.critFrom,
          to: r.critTo,
        });
  }
  const args = {
    check: r.d100CritRoll,
    roll: r.d100MultiplierRoll,
    mult: r.critMultiplier,
  };
  return placeholder
    ? t("result.critPlaceholder", args)
    : t("result.crit", { ...args, from: r.critFrom, to: r.critTo });
}

/**
 * "Long Sword +2 vs AC 5", "… vs unknown AC". The weapon and the ammunition are named by id in the
 * player's language (ADR 0023 step 12).
 */
function attackHeading(
  t: TFunction,
  r: RollResult,
  names: InventoryNamer,
): string {
  const unknown = r.targetAc === null;
  const weapon = names.weapon(r.weaponId, r.weaponName ?? "");
  if (r.ammo) {
    const ammo = names.item(r.ammo.itemId, r.ammo.name);
    return unknown
      ? t("result.attackHeadingAmmoUnknown", { weapon, ammo })
      : t("result.attackHeadingAmmo", { weapon, ammo, ac: r.targetAc });
  }
  return unknown
    ? t("result.attackHeadingUnknown", { weapon })
    : t("result.attackHeading", { weapon, ac: r.targetAc });
}

/**
 * Attack: d20, the AC it hits, the verdict, the damage row and the calculation behind a toggle.
 * Against an unknown AC (docs/adr/0012) the card also says what to tell the DM, offers the DM's
 * Touché / Raté, and dims the damage "if it hits" until the AC or a verdict settles it.
 */
function AttackCard({
  r,
  interactive,
}: {
  r: RollResult;
  interactive: boolean;
}) {
  const { t, i18n } = useTranslation();
  const itemName = itemNamer(
    useItemsStore((st) => st.items),
    i18n.language,
  );
  const names = useInventoryName();
  // The weapon's own +N is detailed by the weapon's name: named by id in the player's language.
  const weaponNamed = <M extends { kind: string; detail?: string }>(m: M): M =>
    m.kind === "weapon_magic"
      ? { ...m, detail: names.weapon(r.weaponId, m.detail ?? "") }
      : m;
  const detailOpen = useJournalStore((s) => s.detailOpen);
  const setDetailOpen = useJournalStore((s) => s.setDetailOpen);
  const settled = attackSettled(r);
  const diceChips = attackDiceChips(r);
  const voided = useLogStore((s) =>
    s.entries.some((e) => e.id === r.logEntryId && e.status === "voided"),
  );
  const correctable = interactive && r.logEntryId !== undefined && !voided;
  return (
    <div className="flex flex-col gap-3" data-testid="attack-result">
      <div className="flex items-center gap-3">
        <HexDie
          value={r.d20Roll}
          label={t("journal.d20", { value: r.d20Roll })}
        />
        <div className="min-w-0 flex-1">
          <ResultLine
            verdict={
              settled ? (
                <>
                  <Verdict
                    ok={r.hit}
                    yes={t("result.hit")}
                    no={t("result.miss")}
                  />
                  {r.verdict && (
                    <span
                      className="text-label text-muted-foreground"
                      data-testid="verdict-by-gm"
                    >
                      {t("result.unknownAc.byGm")}
                    </span>
                  )}
                </>
              ) : (
                <span
                  data-testid="verdict"
                  className="chip shrink-0 px-2.5 py-0.5 text-body font-semibold text-muted-foreground"
                  title={t("result.unknownAc.unsettled")}
                  aria-label={t("result.unknownAc.unsettled")}
                >
                  ?
                </span>
              )
            }
          >
            <span className="text-key tabular-nums" data-testid="hits-ac">
              {t("journal.hitsAc", { ac: hitsAc(r) })}
            </span>
          </ResultLine>
          <div className="text-body text-muted-foreground">
            {attackHeading(t, r, names)}
            {r.targetSize && <> · {t(`result.sizes.${r.targetSize}`)}</>}
          </div>
        </div>
      </div>
      {/* The stat tiles of the save (ADR 0027 step 8.4): the d20, the target's AC, the margin. */}
      <dl
        className="grid grid-cols-[repeat(auto-fit,minmax(5rem,1fr))] gap-2"
        data-testid="attack-tiles"
      >
        <Fact label={t("result.d20")}>{r.d20Roll}</Fact>
        <Fact label={t("result.targetAc")}>
          <span data-testid="attack-target-ac">
            {r.targetAc === null ? "?" : r.targetAc}
          </span>
        </Fact>
        <Fact label={t("result.margin")}>
          {r.acKnown === false ? "–" : formatSigned(r.hitMargin)}
        </Fact>
      </dl>
      {r.isCrit && (
        <p
          className="rounded-md bg-highlight-fill px-3 py-1 text-body font-semibold text-on-highlight"
          data-testid="crit-badge"
        >
          {critText(t, r)}
        </p>
      )}
      {r.acKnown === false && r.critsFromAc !== undefined && (
        <p
          className="rounded-md bg-raised px-3 py-2 text-body tabular-nums"
          data-testid="tell-gm"
        >
          {r.verdict
            ? t("result.unknownAc.noCrit")
            : tellGmWithCrit(r)
              ? t("result.unknownAc.tellGm", {
                  hits: hitsAc(r),
                  crits: r.critsFromAc,
                })
              : t("result.unknownAc.tellGmNoCrit", { hits: hitsAc(r) })}
        </p>
      )}

      <div className="flex items-end justify-between gap-3">
        <div
          className={cn("flex min-w-0 flex-col gap-2", !r.hit && "opacity-50")}
        >
          <span className={labelClass}>{t("result.damage")}</span>
          {diceChips ? (
            <div
              className="flex flex-wrap items-center gap-2"
              data-testid="damage-dice"
            >
              <span className="text-body tabular-nums text-muted-foreground">
                {r.damageDice}
              </span>
              <ul
                className="flex flex-wrap gap-2"
                data-testid="damage-dice-chips"
              >
                {diceChips.map((c, i) => (
                  <DieChip key={i} value={c.value} tone={c.tone} />
                ))}
              </ul>
              <span className="text-body font-semibold tabular-nums">
                = {r.baseDamage}
              </span>
            </div>
          ) : (
            <span
              className="chip w-fit px-2 py-0.5 text-body font-semibold tabular-nums"
              data-testid="damage-dice"
            >
              {t("journal.diceRolled", {
                dice: r.damageDice,
                total: r.baseDamage,
              })}
            </span>
          )}
          {(r.hit || !settled) && (
            <span
              className="text-label break-words text-muted-foreground tabular-nums"
              data-testid="damage-formula"
            >
              {damageFormula(r, i18n.language)}
            </span>
          )}
        </div>
        <Total
          label={
            settled
              ? t("result.totalDamage")
              : t("result.unknownAc.damageIfHit")
          }
          value={r.hit || !settled ? r.totalDamage : "–"}
          testId="total-damage"
          dim={!r.hit}
          gold={r.hit && r.isCrit}
        />
      </div>

      {correctable && r.acKnown === false && (
        <DmVerdict result={r} entryId={r.logEntryId as number} />
      )}
      {correctable && (
        // The fold is keyed by entry: a new attack starts it closed (or open on a correction), and
        // it stays as the player left it while the same attack is corrected. The form is keyed by
        // entry and AC so a new attack, or a popped correction, starts from the AC shown.
        <AdjustFold
          key={r.logEntryId}
          title={
            r.targetAc === null
              ? t("result.adjustAc.giveLabel")
              : t("result.adjustAc.label")
          }
          open={acCorrectionInEffect(r)}
          testId="adjust-ac"
        >
          <AdjustAttackAc
            key={adjustFormKey(r)}
            result={r}
            entryId={r.logEntryId as number}
          />
        </AdjustFold>
      )}

      {r.ammo && (
        <p className="text-body text-muted-foreground" data-testid="ammo-left">
          {t("result.ammoLeft", {
            name: names.item(r.ammo.itemId, r.ammo.name),
            count: r.ammo.remaining,
          })}
        </p>
      )}

      <details
        open={detailOpen}
        onToggle={(ev) => setDetailOpen(ev.currentTarget.open)}
        className="rounded-lg bg-raised px-3 py-2"
      >
        <summary className="cursor-pointer text-body font-medium">
          {t("journal.details")}
        </summary>
        <div className="mt-2 flex flex-col gap-3">
          {/* The d20 and the margin are the tiles above; the rest of the to-hit math is here. */}
          <dl className="grid grid-cols-3 gap-2">
            <Fact label={t("result.critRoll")}>{r.d100CritRoll}</Fact>
            {r.d100MultiplierRoll !== undefined && (
              <Fact label={t("result.multRoll")}>{r.d100MultiplierRoll}</Fact>
            )}
            <Fact label={t("result.baseThac0")}>{r.baseThac0}</Fact>
            <Fact label={t("result.effectiveThac0")}>{r.effectiveThac0}</Fact>
            <Fact label={t("result.hitsAc")}>{hitsAc(r)}</Fact>
            {r.critsFromAc !== undefined && (
              <Fact label={t("result.critsFromAc")}>{r.critsFromAc}</Fact>
            )}
          </dl>
          <div>
            <div className={labelClass}>{t("result.modifiers")}</div>
            {r.modifiers.length === 0 ? (
              <p className="text-body text-muted-foreground">
                {t("result.noModifiers")}
              </p>
            ) : (
              <ul
                className="mt-1 flex flex-col gap-1 text-body"
                data-testid="to-hit-modifiers"
              >
                {r.modifiers.map((m, i) => (
                  <li key={i} className="flex justify-between gap-2">
                    <span className="min-w-0 break-words">
                      {modLabel(t, weaponNamed(m), undefined, itemName)}
                    </span>
                    <span className="tabular-nums">
                      {formatSigned(m.value)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </div>
          {(r.hit || !settled) && r.damageTerms.length > 0 && (
            <div>
              <div className={labelClass}>{t("result.damageTerms")}</div>
              <ul
                className="mt-1 flex flex-col gap-1 text-body"
                data-testid="damage-terms"
              >
                {r.damageTerms.map((d, i) => (
                  <li key={i} className="flex justify-between gap-2">
                    <span className="min-w-0 break-words">
                      {modLabel(t, weaponNamed(d), r, itemName)}
                    </span>
                    <span className="tabular-nums">
                      {d.term === "spells" ||
                      d.term === "dice_multiplier" ||
                      d.term === "standard_multiplier" ||
                      d.term === "multiplier"
                        ? formatFactor(d.value, i18n.language)
                        : formatSigned(d.value)}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </details>
    </div>
  );
}

/** The DM's Touché / Raté on the latest attack while its AC is unknown; the server settles it. */
function DmVerdict({
  result: r,
  entryId,
}: {
  result: RollResult;
  entryId: number;
}) {
  const { t } = useTranslation();
  const verdictLastAttack = useRollStore((s) => s.verdictLastAttack);
  const rolling = useRollStore((s) => s.rolling);
  return (
    <div
      role="group"
      aria-label={t("result.unknownAc.verdictLabel")}
      className="flex flex-wrap items-center gap-2 rounded-lg bg-raised px-3 py-1.5"
    >
      <span className="text-body font-medium">
        {t("result.unknownAc.verdictTitle")}
      </span>
      <Button
        type="button"
        size="sm"
        aria-pressed={r.verdict === "hit"}
        disabled={rolling}
        onClick={() => void verdictLastAttack(entryId, true)}
      >
        {t("result.hit")}
      </Button>
      <Button
        type="button"
        size="sm"
        aria-pressed={r.verdict === "miss"}
        disabled={rolling}
        onClick={() => void verdictLastAttack(entryId, false)}
      >
        {t("result.miss")}
      </Button>
    </div>
  );
}

/**
 * The target AC of the latest attack (inside its one-line fold): − / + around the AC field, Apply.
 * The server recomputes hit, crit and damage with the dice of the attack. Against an unknown AC the
 * field starts empty and Apply gives the DM's AC (it replaces a verdict).
 */
function AdjustAttackAc({
  result: r,
  entryId,
}: {
  result: RollResult;
  entryId: number;
}) {
  const { t } = useTranslation();
  const adjustLastAttack = useRollStore((s) => s.adjustLastAttack);
  const clearLastAttackAc = useRollStore((s) => s.clearLastAttackAc);
  const rolling = useRollStore((s) => s.rolling);
  const maxAc = useAttackStore((s) => s.view?.maxRealAc);
  const [text, setText] = useState(
    r.targetAc === null ? "" : String(r.targetAc),
  );
  const unknown = r.targetAc === null;
  const ac = parseTargetAc(text, maxAc);
  const changed = ac !== null && ac !== r.targetAc;
  const step = (by: number) =>
    setText((v) =>
      String(
        stepTargetAc(
          parseTargetAc(v, maxAc) ?? r.targetAc ?? UNKNOWN_AC_STEP_BASE,
          by,
          maxAc,
        ),
      ),
    );

  const label = unknown
    ? t("result.adjustAc.giveLabel")
    : t("result.adjustAc.label");
  return (
    <form
      className="flex flex-wrap items-center gap-2"
      aria-label={label}
      onSubmit={(ev) => {
        ev.preventDefault();
        if (changed) void adjustLastAttack(entryId, ac);
      }}
    >
      <div className="flex items-center gap-1">
        <Button
          type="button"
          size="sm"
          aria-label={t("result.adjustAc.decrease")}
          onClick={() => step(-1)}
        >
          −
        </Button>
        <input
          className={cn(fieldClass, "w-16 text-center tabular-nums")}
          type="number"
          min={-MAX_TARGET_AC}
          max={maxAc ?? MAX_TARGET_AC}
          step={1}
          value={text}
          placeholder={t("attack.unknownAcPlaceholder")}
          aria-label={
            unknown
              ? t("result.adjustAc.realValue")
              : t("result.adjustAc.value")
          }
          aria-invalid={text !== "" && ac === null}
          onChange={(ev) => setText(ev.target.value)}
        />
        <Button
          type="button"
          size="sm"
          aria-label={t("result.adjustAc.increase")}
          onClick={() => step(1)}
        >
          +
        </Button>
      </div>
      <Button type="submit" size="sm" disabled={rolling || !changed}>
        {unknown ? t("result.adjustAc.give") : t("result.adjustAc.apply")}
      </Button>
      {!unknown && (
        <Button
          type="button"
          size="sm"
          disabled={rolling}
          data-testid="ac-unknown"
          title={t("result.adjustAc.backToUnknown")}
          aria-label={t("result.adjustAc.backToUnknown")}
          onClick={() => void clearLastAttackAc(entryId)}
        >
          {t("attack.unknownAc")}
        </Button>
      )}
      {r.rolledTargetAc !== undefined && (
        <span
          className="text-body text-muted-foreground"
          data-testid="ac-corrected"
        >
          {t("result.adjustAc.was", { ac: r.rolledTargetAc })}
        </span>
      )}
      {r.rolledAcUnknown && !unknown && (
        <span
          className="text-body text-muted-foreground"
          data-testid="ac-given"
        >
          {t("result.adjustAc.given")}
        </span>
      )}
    </form>
  );
}
