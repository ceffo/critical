import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { ArrowUp, CaretRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useDiceStore } from "@/stores/dice";
import { useAttackStore } from "@/stores/attack";
import { useFilStore } from "@/stores/fil";
import { useLogStore } from "@/stores/log";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { formatFactor, formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import {
  MAX_ADJUSTMENT,
  adjustmentChanged,
  adjustmentInEffect,
  stepAdjustment,
} from "@/lib/saveAdjust";
import {
  checkAdjustedTarget,
  maxSkillAdjustment,
  stepSkillAdjustment,
} from "@/lib/skillAdjust";
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
import { damageFormula, itemRollText, modLabel } from "@/lib/rollText";
import { itemNamer } from "@/lib/itemPowers";
import {
  cardHeader,
  cardModel,
  cardSource,
  dieFaces,
  dieTone,
  attackDiceChips,
  castInstances,
  type DieChipView,
  liveKind,
  liveModel,
  type CardModel,
  type InfoView,
  type SaveView,
} from "@/lib/filCard";
import { useSpellName } from "@/i18n/spellName";
import { type InventoryNamer, useInventoryName } from "@/i18n/inventoryName";
import { modeName } from "@/lib/spellMode";
import { rollLabel } from "@/lib/spellDice";
import { abilityText, counters, type CastLogPayload } from "@/lib/logSummary";
import type { RollResult, SaveCategory } from "@/types/combat";
import type { UseResult } from "@/types/items";
import type { AbilityCheck, SkillCheck } from "@/types/skills";
import type { DiceRoll } from "@/types/dice";
import type { TurnResult } from "@/types/character";
import { dieName } from "@/lib/diceFormula";
import { fieldClass } from "./fieldStyles";
import {
  CastResultView,
  ExposureResultView,
  SpellFailureLine,
} from "./CastResultView";
import { ExposureControls } from "./ExposureControls";

const labelClass = "text-label text-muted-foreground";

/**
 * The result panel of the Act activity (ADR 0022 step 2.5): the latest result by default, or the journal entry the player
 * selected (read-only, accent ring, "Latest" returns). Every kind renders from structured data, the
 * entry's payload or this session's roll response; corrections only on the latest result.
 */
export function ResultCard() {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const last = useRollStore((s) => s.last);
  const entries = useLogStore((s) => s.entries);
  const selectedId = useFilStore((s) => s.selectedId);
  const showLatest = useFilStore((s) => s.showLatest);
  const source = cardSource(last, entries, selectedId);

  let body: ReactNode = (
    <p className="py-8 text-center text-body text-muted-foreground">
      {t("result.empty")}
    </p>
  );
  let header: ReactNode = null;
  const readOnly = source.kind === "entry" && !source.latest;
  if (source.kind !== "empty") {
    const entry = source.entry;
    const model =
      source.kind === "live"
        ? liveModel(source.last)
        : cardModel(source.entry, t, spellName, names);
    const kind =
      entry?.kind ??
      (source.kind === "live" ? liveKind(source.last) : "attack");
    const h = cardHeader(
      kind,
      model,
      entry,
      t,
      i18n.language,
      spellName,
      names,
    );
    header = (
      <div
        className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1"
        data-testid="card-header"
      >
        <span className="chip shrink-0 px-1.5 py-0.5 text-label font-medium">
          {h.tag}
        </span>
        <span
          className="min-w-0 flex-1 basis-24 break-words text-body font-medium"
          title={h.subtitle}
        >
          {h.subtitle}
        </span>
        {entry?.status === "voided" && (
          <span className="shrink-0 text-label text-danger">
            {t("log.status.voided")}
          </span>
        )}
        {h.when && (
          <span
            className="shrink-0 text-label text-muted-foreground tabular-nums"
            data-testid="card-when"
          >
            {h.when}
          </span>
        )}
        {readOnly && (
          <Button size="xs" className="ml-auto shrink-0" onClick={showLatest}>
            {t("fil.latest")}
            <ArrowUp aria-hidden />
          </Button>
        )}
      </div>
    );
    body = <CardBody model={model} interactive={!readOnly} />;
  }

  return (
    <section
      className={cn(
        // The "latest" mark (ADR 0026 step 5.2) while the card shows the latest result; an older
        // entry picked in the journal shows on the plain card.
        "zone-card flex min-h-0 min-w-0 flex-col",
        !readOnly && "latest",
      )}
      aria-label={t("fil.card")}
      data-readonly={readOnly}
    >
      {/* The header band (ADR 0026 step 6.2): kind, name and time; it replaces the top padding. */}
      <div className="card-band flex min-h-10 shrink-0 items-center gap-2 px-4 py-2">
        {header ?? <h2 className="section-label">{t("fil.card")}</h2>}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 pt-3 pb-3.5">
        {body}
      </div>
    </section>
  );
}

function CardBody({
  model,
  interactive,
}: {
  model: CardModel;
  interactive: boolean;
}) {
  switch (model.layout) {
    case "attack":
      return <AttackResult r={model.roll} interactive={interactive} />;
    case "save":
      return <SaveResult save={model.save} interactive={interactive} />;
    case "castLive":
      return <CastResultView result={model.result} />;
    case "cast":
      return <CastEntry cast={model.cast} interactive={interactive} />;
    case "exposure":
      return (
        <ExposureResultView result={model.exposure} interactive={interactive} />
      );
    case "item":
      return <ItemResult r={model.item} />;
    case "initiative":
      return <InitiativeResult d10={model.d10} />;
    case "skill":
      return <SkillResult check={model.check} interactive={interactive} />;
    case "ability":
      return <AbilityResult check={model.check} interactive={interactive} />;
    case "dice":
      return <DiceResult roll={model.roll} />;
    case "turn":
      return <TurnResultView turn={model.turn} />;
    case "info":
      return <InfoResult info={model.info} />;
  }
}

/**
 * Turning undead (ADR 0030 step 16): the cleric's level, the d20 against the cell's target (or the
 * cell's outcome when no die is rolled), the verdict and the dice of how many, all as the server
 * recorded them.
 */
function TurnResultView({ turn: r }: { turn: TurnResult }) {
  const { t } = useTranslation();
  const verdictText = r.success
    ? t(r.effect === "destroy" ? "result.turn.destroyed" : "result.turn.turned")
    : t("result.turn.failed");
  return (
    <div className="flex flex-col gap-3" data-testid="turn-result">
      <div className="text-body text-muted-foreground">
        {t("result.turn.heading", { level: r.clericLevel })}
      </div>
      {r.d20 !== undefined && r.target !== undefined ? (
        <div className="flex items-center gap-3">
          <HexDie value={r.d20} label={t("fil.d20", { value: r.d20 })} />
          <ResultLine
            verdict={
              <Verdict ok={r.success} yes={verdictText} no={verdictText} />
            }
          >
            <span className="text-body">
              {t("result.turn.target", { target: r.target })}
            </span>
          </ResultLine>
        </div>
      ) : (
        <ResultLine
          verdict={
            <Verdict ok={r.success} yes={verdictText} no={verdictText} />
          }
        >
          <span className="text-body">
            {t("result.turn.auto", {
              outcome: t(`turning.outcome.${r.outcome ?? "cannot"}`),
            })}
          </span>
        </ResultLine>
      )}
      {r.destroyed > 0 && (
        <p className="text-body font-semibold" data-testid="turn-destroyed">
          {t("result.turn.destroyedCount", {
            count: r.destroyed,
            dice: (r.affected ?? []).join(" + "),
          })}
        </p>
      )}
      {r.turned > 0 && (
        <p className="text-body font-semibold" data-testid="turn-turned">
          {t("result.turn.turnedCount", {
            count: r.turned,
            dice: (r.effect === "destroy"
              ? (r.more ?? [])
              : (r.affected ?? [])
            ).join(" + "),
          })}
        </p>
      )}
    </div>
  );
}

/** The natural die of a roll in a hexagon, the handoff's d20 shape. */
function HexDie({ value, label }: { value: number; label: string }) {
  return (
    <span
      role="img"
      aria-label={label}
      className="flex h-12 w-11 shrink-0 items-center justify-center bg-tint text-key text-on-accent tabular-nums [clip-path:polygon(50%_0,100%_25%,100%_75%,50%_100%,0_75%,0_25%)]"
      data-testid="hex-die"
    >
      {value}
    </span>
  );
}

/**
 * A stat tile (a label over its value). A row of them is the tile grid: as many columns as fit at
 * 5rem each, so the save's four facts sit on one row on a wide card and wrap, never cut, on a
 * narrow one.
 */
function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="rounded-lg bg-raised px-3 py-2">
      <dt className={labelClass}>{label}</dt>
      <dd className="text-body font-semibold tabular-nums">{children}</dd>
    </div>
  );
}

/**
 * The verdict as a filled pill (ADR 0026 step 5.5): success or danger fill, on-accent text. It sits
 * right after the result it judges (ADR 0027 step 8.1), never at the card's far edge.
 */
function Verdict({ ok, yes, no }: { ok: boolean; yes: string; no: string }) {
  return (
    <span
      data-testid="verdict"
      data-ok={ok}
      className={cn(
        "inline-flex shrink-0 items-center rounded-full px-2.5 py-0.5 text-body font-semibold text-on-accent",
        ok ? "bg-success" : "bg-danger",
      )}
    >
      {ok ? yes : no}
    </span>
  );
}

/** The key line of a check: its result (the save's total, the check's roll, the attack's AC) and the verdict beside it. */
function ResultLine({
  children,
  verdict,
}: {
  children: ReactNode;
  verdict: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
      {children}
      {verdict}
    </div>
  );
}

/**
 * A correction after the roll folded to one line (ADR 0027 step 8.2): closed by default, open when a
 * correction is in effect. The line is a real disclosure (keyboard and screen reader), the form
 * opens under it.
 */
function AdjustFold({
  title,
  open: initiallyOpen,
  testId,
  children,
}: {
  title: string;
  open: boolean;
  testId: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(initiallyOpen);
  return (
    <details
      open={open}
      onToggle={(ev) => setOpen(ev.currentTarget.open)}
      className="group/fold rounded-lg bg-raised px-3 py-1.5"
      data-testid={testId}
    >
      <summary className="flex cursor-pointer list-none items-center gap-1.5 text-body font-medium [&::-webkit-details-marker]:hidden">
        <CaretRight
          aria-hidden
          className="size-3.5 shrink-0 transition-transform group-open/fold:rotate-90"
        />
        {title}
      </summary>
      <div className="pt-1.5 pb-1">{children}</div>
    </details>
  );
}

/** A big total on the right of a row, in ink (gold only when the roll says crit or max). */
function Total({
  label,
  value,
  testId,
  dim,
  gold,
}: {
  label: string;
  value: ReactNode;
  testId: string;
  dim?: boolean;
  /** Gold only for a crit or a maximum (ADR 0026 steps 5.3 and 5.8); a total is in ink. */
  gold?: boolean;
}) {
  return (
    <div className="shrink-0 text-right">
      <div className={labelClass}>{label}</div>
      <div
        className={cn(
          "text-headline tabular-nums",
          dim
            ? "text-muted-foreground"
            : gold
              ? "text-highlight"
              : "text-foreground",
        )}
        data-testid={testId}
      >
        {value}
      </div>
    </div>
  );
}

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
 * Against an unknown AC (docs/adr/0012) the card also says what to tell the GM, offers the GM's
 * Touché / Raté, and dims the damage "if it hits" until the AC or a verdict settles it.
 */
function AttackResult({
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
  const detailOpen = useFilStore((s) => s.detailOpen);
  const setDetailOpen = useFilStore((s) => s.setDetailOpen);
  const settled = attackSettled(r);
  const diceChips = attackDiceChips(r);
  const voided = useLogStore((s) =>
    s.entries.some((e) => e.id === r.logEntryId && e.status === "voided"),
  );
  const correctable = interactive && r.logEntryId !== undefined && !voided;
  return (
    <div className="flex flex-col gap-3" data-testid="attack-result">
      <div className="flex items-center gap-3">
        <HexDie value={r.d20Roll} label={t("fil.d20", { value: r.d20Roll })} />
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
              {t("fil.hitsAc", { ac: hitsAc(r) })}
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
              {t("fil.diceRolled", { dice: r.damageDice, total: r.baseDamage })}
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
        <GmVerdict result={r} entryId={r.logEntryId as number} />
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
          {t("fil.details")}
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

/** The GM's Touché / Raté on the latest attack while its AC is unknown; the server settles it. */
function GmVerdict({
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
 * The target AC of the latest attack (inside its one-line fold): − / + around the AC field, Apply. The server recomputes hit,
 * crit and damage with the dice of the attack. Against an unknown AC the field starts empty and
 * Apply gives the GM's AC (it replaces a verdict).
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

/**
 * Save: d20, "roll total vs adjusted target+", the verdict, the buff modifiers, the adjustment on
 * the target side (ADR 0006 step 5) and (latest only) the adjustment row.
 */
function SaveResult({
  save: s,
  interactive,
}: {
  save: SaveView;
  interactive: boolean;
}) {
  const { t, i18n } = useTranslation();
  const itemName = itemNamer(
    useItemsStore((st) => st.items),
    i18n.language,
  );
  const name = t(`saves.categories.${s.category}`, {
    defaultValue: s.category,
  });
  return (
    <div className="flex flex-col gap-3" data-testid="save-result">
      <div className="flex items-center gap-3">
        <HexDie value={s.d20} label={t("fil.d20", { value: s.d20 })} />
        <div className="min-w-0 flex-1">
          <ResultLine
            verdict={
              <Verdict
                ok={s.passed}
                yes={t("result.passed")}
                no={t("result.failed")}
              />
            }
          >
            <span className="text-key tabular-nums" data-testid="save-vs">
              {t("fil.saveVs", {
                total: s.rollTotal,
                target: s.adjustedTarget,
              })}
            </span>
          </ResultLine>
          <div className="text-body text-muted-foreground">
            {t("result.saveHeading", { name })}
          </div>
        </div>
      </div>
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(5rem,1fr))] gap-2">
        <Fact label={t("result.d20")}>{s.d20}</Fact>
        <Fact label={t("result.target")}>{s.target}</Fact>
        {s.adjustment !== 0 && (
          <Fact label={t("result.adjust.adjustedTarget")}>
            <span data-testid="save-adjusted-target">{s.adjustedTarget}</span>
          </Fact>
        )}
        <Fact label={t("result.margin")}>{formatSigned(s.margin)}</Fact>
      </dl>
      {s.adjustment !== 0 && (
        <div
          className="flex justify-between gap-2 text-body"
          data-testid="save-adjustment-line"
        >
          <span className="min-w-0 break-words">
            {s.adjustmentReason ?? t("result.adjust.value")}
          </span>
          <span className="tabular-nums">{formatSigned(s.adjustment)}</span>
        </div>
      )}
      {s.modifiers.length > 0 && (
        <ul
          className="flex flex-col gap-1 text-body"
          data-testid="save-modifiers"
        >
          {s.modifiers.map((m, i) => (
            <li key={i} className="flex justify-between gap-2">
              <span className="min-w-0 break-words">
                {modLabel(t, m, undefined, itemName)}
              </span>
              <span className="tabular-nums">{formatSigned(m.value)}</span>
            </li>
          ))}
        </ul>
      )}
      {interactive && s.logEntryId !== undefined && (
        // Keyed by entry: a new save starts its fold afresh (AdjustCheck).
        <AdjustSave key={s.logEntryId} save={s} entryId={s.logEntryId} />
      )}
    </div>
  );
}

/**
 * The adjustment of the latest save (ADR 0006 steps 5-7): the shared check adjustment, ±20 by 1.
 * The server recomputes verdict and margin.
 */
function AdjustSave({ save: s, entryId }: { save: SaveView; entryId: number }) {
  const { t } = useTranslation();
  const adjustLastSave = useRollStore((st) => st.adjustLastSave);
  return (
    <AdjustCheck
      current={s}
      label={t("result.adjust.label")}
      entryId={entryId}
      valueTestId="save-adjustment"
      step={stepAdjustment}
      onApply={(value, reason) =>
        adjustLastSave(entryId, s.category as SaveCategory, value, reason)
      }
    />
  );
}

/**
 * The adjustment of the latest skill check (ADR 0014 step 6): the shared check adjustment, by 1 on
 * a proficiency, 5 points on a bard skill. The server recomputes the verdict; the roll never changes.
 */
function AdjustSkill({
  check: c,
  entryId,
}: {
  check: SkillCheck;
  entryId: number;
}) {
  const { t } = useTranslation();
  const adjustLastSkill = useRollStore((s) => s.adjustLastSkill);
  const max = maxSkillAdjustment(c.kind);
  return (
    <AdjustCheck
      current={{
        adjustment: c.adjustment ?? 0,
        adjustmentReason: c.adjustmentReason,
      }}
      label={t("result.skill.adjustLabel")}
      entryId={entryId}
      valueTestId="skill-adjustment"
      min={-max}
      max={max}
      step={(v, by) => stepSkillAdjustment(c.kind, v, by)}
      onApply={(value, reason) => adjustLastSkill(entryId, value, reason)}
    />
  );
}

/**
 * "Ajuster après le jet" on the latest check, folded to one line (ADR 0027 steps 8.2 and 8.3):
 * closed by default, open while an adjustment is in effect. Open, one row: − / + around a signed
 * value starting at the current adjustment, an optional reason, Appliquer (disabled only while
 * nothing changed). The value's steps and bounds come from the caller; the server recomputes.
 * Hidden once the entry is voided.
 */
function AdjustCheck({
  current,
  label,
  entryId,
  valueTestId,
  min,
  max,
  step,
  onApply,
}: AdjustCheckProps) {
  const { t } = useTranslation();
  const voided = useLogStore((st) =>
    st.entries.some((e) => e.id === entryId && e.status === "voided"),
  );
  if (voided) return null;
  return (
    <AdjustFold
      title={t("result.adjust.title")}
      open={adjustmentInEffect(current)}
      testId="adjust-check"
    >
      {/* Keyed by the adjustment so an applied or popped one restarts the form from what is shown. */}
      <AdjustCheckForm
        key={`${current.adjustment}:${current.adjustmentReason ?? ""}`}
        current={current}
        label={label}
        valueTestId={valueTestId}
        min={min}
        max={max}
        step={step}
        onApply={onApply}
      />
    </AdjustFold>
  );
}

type AdjustCheckProps = {
  current: { adjustment: number; adjustmentReason?: string };
  label: string;
  entryId: number;
  valueTestId: string;
  min?: number;
  max?: number;
  step: (value: number, by: 1 | -1) => number;
  onApply: (value: number, reason: string) => Promise<unknown>;
};

/** The open fold's one row: − value +, the reason, Appliquer. */
function AdjustCheckForm({
  current,
  label,
  valueTestId,
  min,
  max,
  step,
  onApply,
}: Omit<AdjustCheckProps, "entryId">) {
  const { t } = useTranslation();
  const rolling = useRollStore((s) => s.rolling);
  const [value, setValue] = useState(current.adjustment);
  const [reason, setReason] = useState(current.adjustmentReason ?? "");
  const changed = adjustmentChanged(current, value, reason);

  return (
    <form
      className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2"
      aria-label={label}
      onSubmit={(ev) => {
        ev.preventDefault();
        if (changed) void onApply(value, value === 0 ? "" : reason.trim());
      }}
    >
      <div className="flex items-center gap-1">
        <Button
          type="button"
          size="sm"
          aria-label={t("result.adjust.decrease")}
          disabled={min !== undefined && value <= min}
          onClick={() => setValue((v) => step(v, -1))}
        >
          −
        </Button>
        <output
          className="w-12 text-center text-key tabular-nums"
          aria-label={t("result.adjust.value")}
          data-testid={valueTestId}
        >
          {formatSigned(value)}
        </output>
        <Button
          type="button"
          size="sm"
          aria-label={t("result.adjust.increase")}
          disabled={max !== undefined && value >= max}
          onClick={() => setValue((v) => step(v, 1))}
        >
          +
        </Button>
      </div>
      <input
        className={fieldClass}
        type="text"
        maxLength={80}
        value={reason}
        aria-label={t("result.adjust.reason")}
        placeholder={t("result.adjust.reason")}
        onChange={(ev) => setReason(ev.target.value)}
      />
      <Button type="submit" size="sm" disabled={rolling || !changed}>
        {t("result.adjust.apply")}
      </Button>
    </form>
  );
}

/** One die as a chip: its maximum in the accent, a 1 dimmed. */
function DieChip({
  value,
  faces,
  tone: given,
}: {
  value: number;
  faces?: number | null;
  tone?: DieChipView["tone"];
}) {
  const tone = given ?? dieTone(value, faces ?? null);
  return (
    <li
      className={cn(
        "min-w-7 rounded-md px-2 py-0.5 text-center text-body font-semibold tabular-nums",
        tone === "max" ? "chip-max" : "chip",
        tone === "min" && "text-muted-foreground",
      )}
      data-tone={tone}
    >
      {value}
    </li>
  );
}

function ItemResult({ r }: { r: UseResult }) {
  const { t } = useTranslation();
  const name = useInventoryName().item(r.itemId, r.name);
  const faces = r.dice ? dieFaces(r.dice) : null;
  return (
    <div className="flex flex-col gap-3" data-testid="item-result">
      <div className="text-body text-muted-foreground">
        {t("result.itemHeading", { name })}
      </div>
      {r.dice ? (
        <>
          <div className="flex items-end justify-between gap-3">
            <div className="flex min-w-0 flex-col gap-2">
              <span className={labelClass}>{r.dice}</span>
              <ul className="flex flex-wrap gap-2" data-testid="item-rolls">
                {r.rolls.map((v, i) => (
                  <DieChip key={i} value={v} faces={faces} />
                ))}
              </ul>
            </div>
            <Total
              label={t("result.total")}
              value={r.total}
              testId="item-total"
            />
          </div>
          <div className="text-label text-muted-foreground tabular-nums">
            {itemRollText(r)}
          </div>
        </>
      ) : (
        <p className="text-body text-muted-foreground">{t("result.noRoll")}</p>
      )}
      {r.save && (
        <p className="text-body font-medium" data-testid="item-save">
          {t("result.save", {
            name: t(`saves.categories.${r.save}`, { defaultValue: r.save }),
          })}
        </p>
      )}
      {r.notes && <p className="text-body text-muted-foreground">{r.notes}</p>}
      <p className="text-body text-muted-foreground" data-testid="item-left">
        {r.removed
          ? t("result.leftRemoved", { name })
          : t("result.left", { name, count: r.remaining })}
      </p>
    </div>
  );
}

/** A cast from its journal payload: who cast it, the buff, the to-hit roll, the damage, the item. */
function CastEntry({
  cast: c,
  interactive,
}: {
  cast: CastLogPayload;
  interactive: boolean;
}) {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const spell = spellName(c.spellId, c.spell);
  const instances = castInstances(c);
  return (
    <div className="flex flex-col gap-3" data-testid="cast-entry">
      <div className="text-body text-muted-foreground">
        {c.action === "received"
          ? t("log.cast.received", { spell, level: c.casterLevel })
          : t("log.cast.cast", { spell })}
      </div>
      {c.failure && <SpellFailureLine failure={c.failure} />}
      {c.mode && (
        <p className="text-body" data-testid="cast-entry-mode">
          {t("result.cast.mode", { mode: modeName(t, c.spellId, c.mode) })}
        </p>
      )}
      {c.attack && (
        <div className="flex items-center gap-3">
          <HexDie
            value={c.attack.d20}
            label={t("fil.d20", { value: c.attack.d20 })}
          />
          <div className="min-w-0 flex-1">
            <ResultLine
              verdict={
                <Verdict
                  ok={c.attack.hit}
                  yes={t("result.cast.attackHit")}
                  no={t("result.cast.attackMiss")}
                />
              }
            >
              <span className="text-body">
                {t("fil.castVsAc", { ac: c.attack.targetAc })}
              </span>
            </ResultLine>
          </div>
        </div>
      )}
      {c.buff?.applied && (
        <p className="text-body" data-testid="cast-entry-buff">
          {t("fil.castBuff", {
            buff: spellName(c.spellId, c.buff.name),
            counters: counters(t, c.buff.rounds, c.buff.charges),
          })}
          {c.buff.strGain ? (
            <> · {t("log.cast.strGain", { gain: c.buff.strGain })}</>
          ) : null}
        </p>
      )}
      {c.damage && (
        <div className="flex items-end justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-2">
            <span className="text-body text-muted-foreground">
              {c.damage.type}
            </span>
            {instances && (
              <ul
                className="flex flex-col gap-1"
                data-testid="cast-entry-instances"
              >
                {instances.map((inst, i) => (
                  <li
                    key={i}
                    className="flex flex-wrap items-center gap-x-2 gap-y-1"
                  >
                    <span className="text-label text-muted-foreground">
                      {rollLabel(t, c.damage?.stepsPer, inst.step, i)}
                    </span>
                    {inst.parts ? (
                      <span
                        className="inline-flex flex-wrap items-center gap-x-3 gap-y-1"
                        data-testid="cast-entry-parts"
                      >
                        {inst.parts.map((p, j) => (
                          <span
                            key={j}
                            className="inline-flex flex-wrap items-center gap-2"
                          >
                            {p.dice.length > 0 ? (
                              <>
                                <ul className="flex flex-wrap gap-2">
                                  {p.dice.map((d, k) => (
                                    <DieChip
                                      key={k}
                                      value={d.value}
                                      tone={d.tone}
                                    />
                                  ))}
                                </ul>
                                <span className="text-body font-semibold tabular-nums">
                                  = {p.total}
                                </span>
                              </>
                            ) : (
                              <span className="text-body font-semibold tabular-nums">
                                {p.total}
                              </span>
                            )}
                            <span className="text-label text-muted-foreground">
                              {t(`result.cast.partSave.${p.save}`, {
                                defaultValue: p.save,
                              })}
                            </span>
                          </span>
                        ))}
                        <span className="text-body font-semibold tabular-nums">
                          Σ {inst.total}
                        </span>
                      </span>
                    ) : inst.step &&
                      inst.dice.length === 0 &&
                      inst.total === 0 ? (
                      <span className="text-body text-muted-foreground">
                        {t("result.cast.noDamage")}
                      </span>
                    ) : (
                      <>
                        <ul className="flex flex-wrap gap-2">
                          {inst.dice.map((d, j) => (
                            <DieChip key={j} value={d.value} tone={d.tone} />
                          ))}
                        </ul>
                        <span className="text-body font-semibold tabular-nums">
                          = {inst.total}
                        </span>
                      </>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="flex flex-col items-end gap-1">
            <Total
              label={t("result.cast.damage")}
              value={c.damage.total}
              testId="cast-entry-damage"
            />
            {c.damage.totalOnSave !== undefined && (
              <span
                className="text-body tabular-nums text-muted-foreground"
                data-testid="cast-entry-damage-on-save"
              >
                {t("result.cast.totalOnSave", { total: c.damage.totalOnSave })}
              </span>
            )}
          </div>
        </div>
      )}
      {c.save && (
        <p className="text-body" data-testid="cast-entry-save">
          {t("result.cast.save", {
            type: c.save.type,
            effect: t(`result.cast.saveEffects.${c.save.effect}`, {
              defaultValue: c.save.effect,
            }),
          })}
        </p>
      )}
      {c.item && (
        <p className="text-body">
          {t("result.cast.itemSpent", {
            name: names.item(c.item.itemId, c.item.name),
            left: c.item.remaining,
          })}
        </p>
      )}
      {interactive && c.exposure && (
        <ExposureControls
          spellId={c.spellId}
          casterLevel={c.casterLevel}
          exposure={c.exposure}
        />
      )}
    </div>
  );
}

function InitiativeResult({ d10 }: { d10: number }) {
  const { t } = useTranslation();
  return (
    <div
      className="flex items-end justify-between gap-3"
      data-testid="initiative-result"
    >
      <div className="flex min-w-0 flex-col gap-2">
        <span className={labelClass}>{t("turn.initiativeHint")}</span>
        <ul className="flex gap-2">
          <DieChip value={d10} faces={10} />
        </ul>
      </div>
      <Total
        label={t("turn.initiative")}
        value={d10}
        testId="initiative-total"
      />
    </div>
  );
}

/**
 * A d20/d100-under check (skill or ability): the die, "roll vs adjusted target", the verdict, what
 * the target is made of, the DM's adjustment (with its reason) and, on the latest check only, the
 * control to change it. Every number is the server's.
 */
function CheckView({
  die,
  roll,
  target,
  adjustment,
  adjustmentReason,
  adjustedTarget,
  success,
  heading,
  percent,
  adjust,
  testId,
}: {
  testId: string;
  die: number;
  roll: number;
  target: number;
  adjustment: number;
  adjustmentReason?: string;
  adjustedTarget: number;
  success: boolean;
  heading: string;
  /** A bard skill's targets are percentages. */
  percent?: boolean;
  /** The adjustment control, on the latest check only. */
  adjust: ReactNode;
}) {
  const { t } = useTranslation();
  const show = (n: number) => (percent ? t("skills.percent", { value: n }) : n);
  return (
    <div className="flex flex-col gap-3" data-testid={testId}>
      <div className="flex items-center gap-3">
        <HexDie
          value={roll}
          label={t("fil.die", { faces: die, value: roll })}
        />
        <div className="min-w-0 flex-1">
          <ResultLine
            verdict={
              <Verdict
                ok={success}
                yes={t("result.skill.success")}
                no={t("result.skill.failure")}
              />
            }
          >
            <span className="text-key tabular-nums" data-testid="skill-vs">
              {t(percent ? "fil.skillVsPercent" : "fil.skillVs", {
                roll,
                target: adjustedTarget,
              })}
            </span>
          </ResultLine>
          <div className="text-body text-muted-foreground tabular-nums">
            {heading}
          </div>
        </div>
      </div>
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(5rem,1fr))] gap-2">
        <Fact label={t("result.skill.die", { die })}>{roll}</Fact>
        <Fact label={t("result.target")}>{show(target)}</Fact>
        {adjustment !== 0 && (
          <Fact label={t("result.skill.adjustedTarget")}>
            <span data-testid="skill-adjusted-target">
              {show(adjustedTarget)}
            </span>
          </Fact>
        )}
      </dl>
      {adjustment !== 0 && (
        <div
          className="flex justify-between gap-2 text-body"
          data-testid="skill-adjustment-line"
        >
          <span className="min-w-0 break-words">
            {adjustmentReason ?? t("result.skill.adjustment")}
          </span>
          <span className="tabular-nums">{formatSigned(adjustment)}</span>
        </div>
      )}
      {adjust}
    </div>
  );
}

/** Skill check (ADR 0014): a proficiency against its ability and modifier, a bard skill against its percentage. */
function SkillResult({
  check: c,
  interactive,
}: {
  check: SkillCheck;
  interactive: boolean;
}) {
  const { t } = useTranslation();
  const proficiency = c.kind === "proficiency";
  return (
    <CheckView
      testId="skill-result"
      die={c.die}
      roll={c.roll}
      target={c.target}
      adjustment={c.adjustment ?? 0}
      adjustmentReason={c.adjustmentReason}
      adjustedTarget={checkAdjustedTarget(c)}
      success={c.success}
      percent={!proficiency}
      heading={
        proficiency
          ? t("result.skill.proficiency", {
              die: c.die,
              stat: abilityText(
                t,
                c.ability ?? "",
                c.abilityScore ?? 0,
                c.modifier ?? 0,
              ),
            })
          : t("result.skill.bardSkill", { die: c.die, percent: c.target })
      }
      adjust={
        interactive &&
        c.logEntryId !== undefined && (
          // Keyed by entry: a new check starts its fold afresh (AdjustCheck).
          <AdjustSkill key={c.logEntryId} check={c} entryId={c.logEntryId} />
        )
      }
    />
  );
}

/**
 * Ability check (ADR 0028): a d20 under the score in effect; the sheet's score beside it when an
 * effect changed it. The DM's adjustment is set here, after the roll (step 6).
 */
function AbilityResult({
  check: c,
  interactive,
}: {
  check: AbilityCheck;
  interactive: boolean;
}) {
  const { t } = useTranslation();
  const name = t(`attr.${c.ability}`, { defaultValue: c.ability });
  return (
    <CheckView
      testId="ability-result"
      die={c.die}
      roll={c.roll}
      target={c.target}
      adjustment={c.adjustment}
      adjustmentReason={c.adjustmentReason}
      adjustedTarget={c.adjustedTarget}
      success={c.success}
      heading={t(
        c.target === c.abilityScore
          ? "abilityCheck.heading"
          : "abilityCheck.headingEffect",
        { die: c.die, name, score: c.target, sheet: c.abilityScore },
      )}
      adjust={
        interactive &&
        c.logEntryId !== undefined && (
          // Keyed by entry: a new check starts its fold afresh (AdjustCheck).
          <AdjustAbility key={c.logEntryId} check={c} entryId={c.logEntryId} />
        )
      }
    />
  );
}

/**
 * The adjustment of the latest ability check (ADR 0028 steps 6 and 8): the shared check
 * adjustment, ±20 by 1. The server recomputes the verdict; the roll never changes.
 */
function AdjustAbility({
  check: c,
  entryId,
}: {
  check: AbilityCheck;
  entryId: number;
}) {
  const { t } = useTranslation();
  const adjustLastAbility = useRollStore((s) => s.adjustLastAbility);
  return (
    <AdjustCheck
      current={c}
      label={t("abilityCheck.adjustLabel")}
      entryId={entryId}
      valueTestId="ability-adjustment"
      min={-MAX_ADJUSTMENT}
      max={MAX_ADJUSTMENT}
      step={stepAdjustment}
      onApply={(value, reason) => adjustLastAbility(entryId, value, reason)}
    />
  );
}

/** A free dice roll: the formula, the dice per kind as chips with their subtotal, the constant and the big total. */
function DiceResult({ roll: r }: { roll: DiceRoll }) {
  const { t } = useTranslation();
  const reroll = useDiceStore((s) => s.roll);
  const rolling = useDiceStore((s) => s.rolling);
  return (
    <div className="flex flex-col gap-3" data-testid="dice-result">
      <div className="flex items-end justify-between gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <span
            className="text-body font-medium tabular-nums"
            data-testid="dice-formula"
          >
            {r.formula}
          </span>
          {r.groups.map((g, i) => (
            <div
              key={i}
              className="flex flex-wrap items-center gap-2"
              data-testid="dice-group"
            >
              <span className={cn(labelClass, "tabular-nums")}>
                {g.negative ? "−" : ""}
                {dieName(g.sides)}
              </span>
              <ul className="flex flex-wrap gap-2">
                {g.rolls.map((v, j) => (
                  <DieChip
                    key={j}
                    value={v}
                    faces={g.negative ? null : g.sides}
                  />
                ))}
              </ul>
              <span
                className="text-body font-semibold tabular-nums"
                data-testid="dice-subtotal"
              >
                = {formatSigned(g.subtotal)}
              </span>
            </div>
          ))}
          {r.modifier !== 0 && (
            <div className="text-body tabular-nums" data-testid="dice-modifier">
              <span className={labelClass}>{t("dice.modifier")} </span>
              {formatSigned(r.modifier)}
            </div>
          )}
        </div>
        <Total label={t("dice.total")} value={r.total} testId="dice-total" />
      </div>
      <div>
        <Button
          size="xs"
          variant="ghost"
          disabled={rolling}
          title={t("dice.rerollTitle", { formula: r.formula })}
          onClick={() => void reroll(r.formula)}
        >
          {t("dice.reroll")}
        </Button>
      </div>
    </div>
  );
}

function InfoResult({ info }: { info: InfoView }) {
  const { t } = useTranslation();
  return (
    <div
      className="flex items-start justify-between gap-3"
      data-testid="info-result"
    >
      <div className="flex min-w-0 flex-col gap-1">
        <p className="text-body break-words">{info.text}</p>
        {info.capped && (
          <p className="text-label text-muted-foreground">
            {t(info.capped.hpRequested < 0 ? "log.floored" : "log.capped", {
              requested: formatSigned(info.capped.hpRequested),
            })}
          </p>
        )}
      </div>
      {info.value && (
        <span
          className={cn(
            "shrink-0 text-headline tabular-nums",
            info.value.tone === "damage" ? "text-danger" : "text-success",
          )}
          data-testid="info-value"
        >
          {formatSigned(info.value.amount)}
        </span>
      )}
    </div>
  );
}
