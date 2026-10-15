import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { ArrowUp } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useDiceStore } from "@/stores/dice";
import { useAttackStore } from "@/stores/attack";
import { useFilStore } from "@/stores/fil";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { formatFactor, formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import { adjustmentChanged, stepAdjustment } from "@/lib/saveAdjust";
import {
  MAX_TARGET_AC,
  UNKNOWN_AC_STEP_BASE,
  adjustFormKey,
  attackSettled,
  hitsAc,
  parseTargetAc,
  stepTargetAc,
  tellGmWithCrit,
} from "@/lib/attackAdjust";
import { damageFormula, itemRollText, modLabel } from "@/lib/rollText";
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
import { spellName } from "@/i18n/spellName";
import { modeName } from "@/lib/spellMode";
import { rollLabel } from "@/lib/spellDice";
import { abilityText, counters, type CastLogPayload } from "@/lib/logSummary";
import type { RollResult, SaveCategory } from "@/types/combat";
import type { UseResult } from "@/types/items";
import type { SkillCheck } from "@/types/skills";
import type { DiceRoll } from "@/types/dice";
import { dieName } from "@/lib/diceFormula";
import { fieldClass } from "./fieldStyles";
import { CastResultView } from "./CastResultView";

const labelClass = "text-label text-muted-foreground";

/**
 * The Fil column's detail card: the latest result by default, or the journal entry the player
 * selected (read-only, accent ring, "Latest" returns). Every kind renders from structured data, the
 * entry's payload or this session's roll response; corrections only on the latest result.
 */
export function ResultCard() {
  const { t, i18n } = useTranslation();
  const last = useRollStore((s) => s.last);
  const entries = useLogStore((s) => s.entries);
  const selectedId = useFilStore((s) => s.selectedId);
  const showLatest = useFilStore((s) => s.showLatest);
  const source = cardSource(last, entries, selectedId);

  let body: ReactNode = (
    <p className="py-8 text-center text-body text-muted-foreground">{t("result.empty")}</p>
  );
  let header: ReactNode = null;
  const readOnly = source.kind === "entry" && !source.latest;
  if (source.kind !== "empty") {
    const entry = source.entry;
    const model = source.kind === "live" ? liveModel(source.last) : cardModel(source.entry, t);
    const kind = entry?.kind ?? (source.kind === "live" ? liveKind(source.last) : "attack");
    const h = cardHeader(kind, model, entry, t, i18n.language);
    header = (
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1" data-testid="card-header">
        <span className="shrink-0 rounded-md bg-neu-900 px-1.5 py-0.5 text-label font-medium">
          {h.tag}
        </span>
        <span className="min-w-0 flex-1 basis-24 truncate text-body font-medium" title={h.subtitle}>
          {h.subtitle}
        </span>
        {entry?.status === "voided" && (
          <span className="shrink-0 text-label text-danger">{t("log.status.voided")}</span>
        )}
        {h.when && (
          <span className="shrink-0 text-label text-muted-foreground tabular-nums" data-testid="card-when">
            {h.when}
          </span>
        )}
        {readOnly && (
          <Button size="xs" variant="outline" className="ml-auto shrink-0" onClick={showLatest}>
            {t("fil.latest")}
            <ArrowUp aria-hidden />
          </Button>
        )}
      </div>
    );
    body = (
      <>
        <CardBody model={model} interactive={!readOnly} />
        {entry?.note && (
          <p className="mt-3 text-body italic text-muted-foreground" data-testid="card-note">
            {entry.note}
          </p>
        )}
      </>
    );
  }

  return (
    <section
      className={cn(
        "flex min-h-0 min-w-0 flex-col rounded-xl",
        readOnly ? "bg-card ring-1 ring-amb-700" : "bg-card",
      )}
      aria-label={t("fil.card")}
      data-readonly={readOnly}
    >
      <div className="flex shrink-0 items-center gap-2 px-3 pt-3 pb-2">
        {header ?? <h2 className="text-title">{t("fil.card")}</h2>}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-3">{body}</div>
    </section>
  );
}

function CardBody({ model, interactive }: { model: CardModel; interactive: boolean }) {
  switch (model.layout) {
    case "attack":
      return <AttackResult r={model.roll} interactive={interactive} />;
    case "save":
      return <SaveResult save={model.save} interactive={interactive} />;
    case "castLive":
      return <CastResultView result={model.result} />;
    case "cast":
      return <CastEntry cast={model.cast} />;
    case "item":
      return <ItemResult r={model.item} />;
    case "initiative":
      return <InitiativeResult d10={model.d10} />;
    case "skill":
      return <SkillResult check={model.check} />;
    case "dice":
      return <DiceResult roll={model.roll} />;
    case "info":
      return <InfoResult info={model.info} />;
  }
}

/** The natural die of a roll in a hexagon, the handoff's d20 shape. */
function HexDie({ value, label }: { value: number; label: string }) {
  return (
    <span
      role="img"
      aria-label={label}
      className="flex h-12 w-11 shrink-0 items-center justify-center bg-amb-800 text-key text-amb-100 tabular-nums [clip-path:polygon(50%_0,100%_25%,100%_75%,50%_100%,0_75%,0_25%)]"
      data-testid="hex-die"
    >
      {value}
    </span>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="rounded-lg bg-neu-900/60 px-3 py-2">
      <dt className={labelClass}>{label}</dt>
      <dd className="text-body font-semibold tabular-nums">{children}</dd>
    </div>
  );
}

function Verdict({ ok, yes, no }: { ok: boolean; yes: string; no: string }) {
  return (
    <span
      data-testid="verdict"
      className={cn("shrink-0 text-key", ok ? "text-heal" : "text-danger")}
    >
      {ok ? yes : no}
    </span>
  );
}

/** A big total on the right of a row. */
function Total({ label, value, testId, dim }: { label: string; value: ReactNode; testId: string; dim?: boolean }) {
  return (
    <div className="shrink-0 text-right">
      <div className={labelClass}>{label}</div>
      <div
        className={cn(
          "text-headline tabular-nums",
          dim ? "text-muted-foreground" : "text-amb-300",
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
      : t("result.critLegacy", { mult: r.critMultiplier, from: r.critFrom, to: r.critTo });
  }
  const args = { check: r.d100CritRoll, roll: r.d100MultiplierRoll, mult: r.critMultiplier };
  return placeholder
    ? t("result.critPlaceholder", args)
    : t("result.crit", { ...args, from: r.critFrom, to: r.critTo });
}

/** "Long Sword +2 vs AC 5", "… vs unknown AC". */
function attackHeading(t: TFunction, r: RollResult): string {
  const unknown = r.targetAc === null;
  if (r.ammo) {
    return unknown
      ? t("result.attackHeadingAmmoUnknown", { weapon: r.weaponName, ammo: r.ammo.name })
      : t("result.attackHeadingAmmo", { weapon: r.weaponName, ammo: r.ammo.name, ac: r.targetAc });
  }
  return unknown
    ? t("result.attackHeadingUnknown", { weapon: r.weaponName })
    : t("result.attackHeading", { weapon: r.weaponName, ac: r.targetAc });
}

/**
 * Attack: d20, the AC it hits, the verdict, the damage row and the calculation behind a toggle.
 * Against an unknown AC (docs/adr/0012) the card also says what to tell the GM, offers the GM's
 * Touché / Raté, and dims the damage "if it hits" until the AC or a verdict settles it.
 */
function AttackResult({ r, interactive }: { r: RollResult; interactive: boolean }) {
  const { t, i18n } = useTranslation();
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
          <div className="text-key tabular-nums" data-testid="hits-ac">
            {t("fil.hitsAc", { ac: hitsAc(r) })}
          </div>
          <div className="text-body text-muted-foreground">
            {attackHeading(t, r)}
            {r.targetSize && <> · {t(`result.sizes.${r.targetSize}`)}</>}
          </div>
        </div>
        {settled ? (
          <span className="flex shrink-0 flex-col items-end">
            <Verdict ok={r.hit} yes={t("result.hit")} no={t("result.miss")} />
            {r.verdict && (
              <span className="text-label text-muted-foreground" data-testid="verdict-by-gm">
                {t("result.unknownAc.byGm")}
              </span>
            )}
          </span>
        ) : (
          <span
            data-testid="verdict"
            className="shrink-0 text-key text-muted-foreground"
            title={t("result.unknownAc.unsettled")}
          >
            ?
          </span>
        )}
      </div>
      {r.isCrit && (
        <p className="rounded-md bg-amb-900 px-3 py-1 text-body font-medium text-amb-300" data-testid="crit-badge">
          {critText(t, r)}
        </p>
      )}
      {r.acKnown === false && r.critsFromAc !== undefined && (
        <p className="rounded-md bg-neu-900/60 px-3 py-2 text-body tabular-nums" data-testid="tell-gm">
          {r.verdict
            ? t("result.unknownAc.noCrit")
            : tellGmWithCrit(r)
              ? t("result.unknownAc.tellGm", { hits: hitsAc(r), crits: r.critsFromAc })
              : t("result.unknownAc.tellGmNoCrit", { hits: hitsAc(r) })}
        </p>
      )}

      <div className="flex items-end justify-between gap-3">
        <div className={cn("flex min-w-0 flex-col gap-2", !r.hit && "opacity-50")}>
          <span className={labelClass}>{t("result.damage")}</span>
          {diceChips ? (
            <div className="flex flex-wrap items-center gap-2" data-testid="damage-dice">
              <span className="text-body tabular-nums text-muted-foreground">{r.damageDice}</span>
              <ul className="flex flex-wrap gap-2" data-testid="damage-dice-chips">
                {diceChips.map((c, i) => (
                  <DieChip key={i} value={c.value} tone={c.tone} />
                ))}
              </ul>
              <span className="text-body font-semibold tabular-nums">= {r.baseDamage}</span>
            </div>
          ) : (
            <span className="w-fit rounded-md bg-neu-900 px-2 py-0.5 text-body font-semibold tabular-nums" data-testid="damage-dice">
              {t("fil.diceRolled", { dice: r.damageDice, total: r.baseDamage })}
            </span>
          )}
          {(r.hit || !settled) && (
            <span className="text-label break-words text-muted-foreground tabular-nums" data-testid="damage-formula">
              {damageFormula(r, i18n.language)}
            </span>
          )}
        </div>
        <Total
          label={settled ? t("result.totalDamage") : t("result.unknownAc.damageIfHit")}
          value={r.hit || !settled ? r.totalDamage : "–"}
          testId="total-damage"
          dim={!r.hit}
        />
      </div>

      {correctable && r.acKnown === false && (
        <GmVerdict result={r} entryId={r.logEntryId as number} />
      )}
      {correctable && (
        // Keyed by entry and AC so a new attack, or a popped correction, starts from the AC shown.
        <AdjustAttackAc
          key={adjustFormKey(r)}
          result={r}
          entryId={r.logEntryId as number}
        />
      )}

      {r.ammo && (
        <p className="text-body text-muted-foreground" data-testid="ammo-left">
          {t("result.ammoLeft", { name: r.ammo.name, count: r.ammo.remaining })}
        </p>
      )}

      <details
        open={detailOpen}
        onToggle={(ev) => setDetailOpen(ev.currentTarget.open)}
        className="rounded-lg bg-neu-900/40 px-3 py-2"
      >
        <summary className="cursor-pointer text-body font-medium">{t("fil.details")}</summary>
        <div className="mt-2 flex flex-col gap-3">
          <dl className="grid grid-cols-3 gap-2">
            <Fact label={t("result.d20")}>{r.d20Roll}</Fact>
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
            <Fact label={t("result.margin")}>
              {r.acKnown === false ? "–" : formatSigned(r.hitMargin)}
            </Fact>
          </dl>
          <div>
            <div className={labelClass}>{t("result.modifiers")}</div>
            {r.modifiers.length === 0 ? (
              <p className="text-body text-muted-foreground">{t("result.noModifiers")}</p>
            ) : (
              <ul className="mt-1 flex flex-col gap-1 text-body" data-testid="to-hit-modifiers">
                {r.modifiers.map((m, i) => (
                  <li key={i} className="flex justify-between gap-2">
                    <span className="min-w-0 truncate">{modLabel(t, m)}</span>
                    <span className="tabular-nums">{formatSigned(m.value)}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
          {(r.hit || !settled) && r.damageTerms.length > 0 && (
            <div>
              <div className={labelClass}>{t("result.damageTerms")}</div>
              <ul className="mt-1 flex flex-col gap-1 text-body" data-testid="damage-terms">
                {r.damageTerms.map((d, i) => (
                  <li key={i} className="flex justify-between gap-2">
                    <span className="min-w-0 truncate">{modLabel(t, d, r)}</span>
                    <span className="tabular-nums">
                      {d.term === "spells" || d.term === "dice_multiplier" || d.term === "multiplier"
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
function GmVerdict({ result: r, entryId }: { result: RollResult; entryId: number }) {
  const { t } = useTranslation();
  const verdictLastAttack = useRollStore((s) => s.verdictLastAttack);
  const rolling = useRollStore((s) => s.rolling);
  return (
    <div
      role="group"
      aria-label={t("result.unknownAc.verdictLabel")}
      className="flex flex-wrap items-center gap-2 rounded-lg bg-neu-900/60 p-3"
    >
      <span className="text-body font-medium">{t("result.unknownAc.verdictTitle")}</span>
      <Button
        type="button"
        size="sm"
        variant={r.verdict === "hit" ? "default" : "outline"}
        aria-pressed={r.verdict === "hit"}
        disabled={rolling}
        onClick={() => void verdictLastAttack(entryId, true)}
      >
        {t("result.hit")}
      </Button>
      <Button
        type="button"
        size="sm"
        variant={r.verdict === "miss" ? "default" : "outline"}
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
 * The target AC of the latest attack: − / + around the AC field, Apply. The server recomputes hit,
 * crit and damage with the dice of the attack. Against an unknown AC the field starts empty and
 * Apply gives the GM's AC (it replaces a verdict).
 */
function AdjustAttackAc({ result: r, entryId }: { result: RollResult; entryId: number }) {
  const { t } = useTranslation();
  const adjustLastAttack = useRollStore((s) => s.adjustLastAttack);
  const clearLastAttackAc = useRollStore((s) => s.clearLastAttackAc);
  const rolling = useRollStore((s) => s.rolling);
  const maxAc = useAttackStore((s) => s.view?.maxRealAc);
  const [text, setText] = useState(r.targetAc === null ? "" : String(r.targetAc));
  const unknown = r.targetAc === null;
  const ac = parseTargetAc(text, maxAc);
  const changed = ac !== null && ac !== r.targetAc;
  const step = (by: number) =>
    setText((v) =>
      String(stepTargetAc(parseTargetAc(v, maxAc) ?? r.targetAc ?? UNKNOWN_AC_STEP_BASE, by, maxAc)),
    );

  return (
    <form
      className="flex flex-wrap items-center gap-2 rounded-lg bg-neu-900/60 p-3"
      aria-label={unknown ? t("result.adjustAc.giveLabel") : t("result.adjustAc.label")}
      onSubmit={(ev) => {
        ev.preventDefault();
        if (changed) void adjustLastAttack(entryId, ac);
      }}
    >
      <span className="text-body font-medium">
        {unknown ? t("result.adjustAc.realTitle") : t("result.adjustAc.title")}
      </span>
      <div className="flex items-center gap-1">
        <Button
          type="button"
          size="sm"
          variant="outline"
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
          aria-label={unknown ? t("result.adjustAc.realValue") : t("result.adjustAc.value")}
          aria-invalid={text !== "" && ac === null}
          onChange={(ev) => setText(ev.target.value)}
        />
        <Button
          type="button"
          size="sm"
          variant="outline"
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
          variant="outline"
          disabled={rolling}
          data-testid="ac-unknown"
          onClick={() => void clearLastAttackAc(entryId)}
        >
          {t("result.adjustAc.backToUnknown")}
        </Button>
      )}
      {r.rolledTargetAc !== undefined && (
        <span className="text-body text-muted-foreground" data-testid="ac-corrected">
          {t("result.adjustAc.was", { ac: r.rolledTargetAc })}
        </span>
      )}
      {r.rolledAcUnknown && !unknown && (
        <span className="text-body text-muted-foreground" data-testid="ac-given">
          {t("result.adjustAc.given")}
        </span>
      )}
    </form>
  );
}

/** Save: d20, "total vs target+", the verdict, its modifiers and (latest only) the adjustment row. */
function SaveResult({ save: s, interactive }: { save: SaveView; interactive: boolean }) {
  const { t } = useTranslation();
  const name = t(`saves.categories.${s.category}`, { defaultValue: s.category });
  const mods = s.adjustment
    ? [...s.modifiers, { kind: "adjustment" as const, detail: s.adjustmentReason, value: s.adjustment }]
    : s.modifiers;
  return (
    <div className="flex flex-col gap-3" data-testid="save-result">
      <div className="flex items-center gap-3">
        <HexDie value={s.d20} label={t("fil.d20", { value: s.d20 })} />
        <div className="min-w-0 flex-1">
          <div className="text-key tabular-nums" data-testid="save-vs">
            {t("fil.saveVs", { total: s.total ?? s.d20, target: s.target })}
          </div>
          <div className="text-body text-muted-foreground">{t("result.saveHeading", { name })}</div>
        </div>
        <Verdict ok={s.passed} yes={t("result.passed")} no={t("result.failed")} />
      </div>
      <dl className="grid grid-cols-3 gap-2">
        <Fact label={t("result.d20")}>{s.d20}</Fact>
        <Fact label={t("result.target")}>{s.target}</Fact>
        <Fact label={t("result.margin")}>{s.margin === undefined ? "–" : formatSigned(s.margin)}</Fact>
      </dl>
      {mods.length > 0 && (
        <ul className="flex flex-col gap-1 text-body" data-testid="save-modifiers">
          {mods.map((m, i) => (
            <li key={i} className="flex justify-between gap-2">
              <span className="min-w-0 truncate">{modLabel(t, m)}</span>
              <span className="tabular-nums">{formatSigned(m.value)}</span>
            </li>
          ))}
        </ul>
      )}
      {interactive && s.logEntryId !== undefined && (
        // Keyed by entry and adjustment so a new save, or a popped correction, starts from the
        // adjustment shown.
        <AdjustSave
          key={`${s.logEntryId}:${s.adjustment}:${s.adjustmentReason ?? ""}`}
          save={s}
          entryId={s.logEntryId}
        />
      )}
    </div>
  );
}

/**
 * The adjustment of the latest save: − / + around a signed value starting at the current
 * adjustment, an optional reason, Apply. The server recomputes verdict and margin.
 */
function AdjustSave({ save: s, entryId }: { save: SaveView; entryId: number }) {
  const { t } = useTranslation();
  const adjustLastSave = useRollStore((s) => s.adjustLastSave);
  const rolling = useRollStore((s) => s.rolling);
  const voided = useLogStore((st) =>
    st.entries.some((e) => e.id === entryId && e.status === "voided"),
  );
  const [value, setValue] = useState(s.adjustment);
  const [reason, setReason] = useState(s.adjustmentReason ?? "");
  if (voided) return null;
  const changed = adjustmentChanged(s, value, reason);

  return (
    <form
      className="flex flex-col gap-2 rounded-lg bg-neu-900/60 p-3"
      aria-label={t("result.adjust.label")}
      onSubmit={(ev) => {
        ev.preventDefault();
        if (changed)
          void adjustLastSave(
            entryId,
            s.category as SaveCategory,
            value,
            value === 0 ? "" : reason.trim(),
          );
      }}
    >
      <div className="text-body font-medium">{t("result.adjust.title")}</div>
      <div className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2">
        <div className="flex items-center gap-1">
          <Button
            type="button"
            size="sm"
            variant="outline"
            aria-label={t("result.adjust.decrease")}
            onClick={() => setValue((v) => stepAdjustment(v, -1))}
          >
            −
          </Button>
          <output
            className="w-10 text-center text-key tabular-nums"
            aria-label={t("result.adjust.value")}
            data-testid="save-adjustment"
          >
            {formatSigned(value)}
          </output>
          <Button
            type="button"
            size="sm"
            variant="outline"
            aria-label={t("result.adjust.increase")}
            onClick={() => setValue((v) => stepAdjustment(v, 1))}
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
      </div>
    </form>
  );
}

/** One die as a chip: its maximum in the accent, a 1 dimmed. */
function DieChip({ value, faces, tone: given }: { value: number; faces?: number | null; tone?: DieChipView["tone"] }) {
  const tone = given ?? dieTone(value, faces ?? null);
  return (
    <li
      className={cn(
        "min-w-7 rounded-md px-2 py-0.5 text-center text-body font-semibold tabular-nums",
        tone === "max" ? "bg-amb-800 text-amb-100" : "bg-neu-900",
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
  const faces = r.dice ? dieFaces(r.dice) : null;
  return (
    <div className="flex flex-col gap-3" data-testid="item-result">
      <div className="text-body text-muted-foreground">{t("result.itemHeading", { name: r.name })}</div>
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
            <Total label={t("result.total")} value={r.total} testId="item-total" />
          </div>
          <div className="text-label text-muted-foreground tabular-nums">{itemRollText(r)}</div>
        </>
      ) : (
        <p className="text-body text-muted-foreground">{t("result.noRoll")}</p>
      )}
      {r.save && (
        <p className="text-body font-medium" data-testid="item-save">
          {t("result.save", { name: t(`saves.categories.${r.save}`, { defaultValue: r.save }) })}
        </p>
      )}
      {r.notes && <p className="text-body text-muted-foreground">{r.notes}</p>}
      <p className="text-body text-muted-foreground" data-testid="item-left">
        {r.removed
          ? t("result.leftRemoved", { name: r.name })
          : t("result.left", { name: r.name, count: r.remaining })}
      </p>
    </div>
  );
}

/** A cast from its journal payload: who cast it, the buff, the to-hit roll, the damage, the item. */
function CastEntry({ cast: c }: { cast: CastLogPayload }) {
  const { t } = useTranslation();
  const spell = spellName(t, c.spellId, c.spell);
  const instances = castInstances(c);
  return (
    <div className="flex flex-col gap-3" data-testid="cast-entry">
      <div className="text-body text-muted-foreground">
        {c.action === "received"
          ? t("log.cast.received", { spell, level: c.casterLevel })
          : t("log.cast.cast", { spell })}
      </div>
      {c.mode && (
        <p className="text-body" data-testid="cast-entry-mode">
          {t("result.cast.mode", { mode: modeName(t, c.spellId, c.mode) })}
        </p>
      )}
      {c.attack && (
        <div className="flex items-center gap-3">
          <HexDie value={c.attack.d20} label={t("fil.d20", { value: c.attack.d20 })} />
          <span className="min-w-0 flex-1 text-body">{t("fil.castVsAc", { ac: c.attack.targetAc })}</span>
          <Verdict ok={c.attack.hit} yes={t("result.cast.attackHit")} no={t("result.cast.attackMiss")} />
        </div>
      )}
      {c.buff?.applied && (
        <p className="text-body" data-testid="cast-entry-buff">
          {t("fil.castBuff", {
            buff: c.buff.name,
            counters: counters(t, c.buff.rounds, c.buff.charges),
          })}
          {c.buff.strGain ? <> · {t("log.cast.strGain", { gain: c.buff.strGain })}</> : null}
        </p>
      )}
      {c.damage && (
        <div className="flex items-end justify-between gap-3">
          <div className="flex min-w-0 flex-col gap-2">
            <span className="text-body text-muted-foreground">{c.damage.type}</span>
            {instances && (
              <ul className="flex flex-col gap-1" data-testid="cast-entry-instances">
                {instances.map((inst, i) => (
                  <li key={i} className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <span className="text-label text-muted-foreground">{rollLabel(t, c.damage?.stepsPer, inst.step, i)}</span>
                    {inst.parts ? (
                      <span className="inline-flex flex-wrap items-center gap-x-3 gap-y-1" data-testid="cast-entry-parts">
                        {inst.parts.map((p, j) => (
                          <span key={j} className="inline-flex flex-wrap items-center gap-2">
                            {p.dice.length > 0 ? (
                              <>
                                <ul className="flex flex-wrap gap-2">
                                  {p.dice.map((d, k) => (
                                    <DieChip key={k} value={d.value} tone={d.tone} />
                                  ))}
                                </ul>
                                <span className="text-body font-semibold tabular-nums">= {p.total}</span>
                              </>
                            ) : (
                              <span className="text-body font-semibold tabular-nums">{p.total}</span>
                            )}
                            <span className="text-label text-muted-foreground">
                              {t(`result.cast.partSave.${p.save}`, { defaultValue: p.save })}
                            </span>
                          </span>
                        ))}
                        <span className="text-body font-semibold tabular-nums">Σ {inst.total}</span>
                      </span>
                    ) : inst.step && inst.dice.length === 0 && inst.total === 0 ? (
                      <span className="text-body text-muted-foreground">{t("result.cast.noDamage")}</span>
                    ) : (
                      <>
                        <ul className="flex flex-wrap gap-2">
                          {inst.dice.map((d, j) => (
                            <DieChip key={j} value={d.value} tone={d.tone} />
                          ))}
                        </ul>
                        <span className="text-body font-semibold tabular-nums">= {inst.total}</span>
                      </>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="flex flex-col items-end gap-1">
            <Total label={t("result.cast.damage")} value={c.damage.total} testId="cast-entry-damage" />
            {c.damage.totalOnSave !== undefined && (
              <span className="text-body tabular-nums text-muted-foreground" data-testid="cast-entry-damage-on-save">
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
            effect: t(`result.cast.saveEffects.${c.save.effect}`, { defaultValue: c.save.effect }),
          })}
        </p>
      )}
      {c.item && (
        <p className="text-body">{t("result.cast.itemSpent", { name: c.item.name, left: c.item.remaining })}</p>
      )}
    </div>
  );
}

function InitiativeResult({ d10 }: { d10: number }) {
  const { t } = useTranslation();
  return (
    <div className="flex items-end justify-between gap-3" data-testid="initiative-result">
      <div className="flex min-w-0 flex-col gap-2">
        <span className={labelClass}>{t("turn.initiativeHint")}</span>
        <ul className="flex gap-2">
          <DieChip value={d10} faces={10} />
        </ul>
      </div>
      <Total label={t("turn.initiative")} value={d10} testId="initiative-total" />
    </div>
  );
}

/** Skill check: the die, "roll vs target", the verdict, and what the target is made of. */
function SkillResult({ check: c }: { check: SkillCheck }) {
  const { t } = useTranslation();
  const proficiency = c.kind === "proficiency";
  return (
    <div className="flex flex-col gap-3" data-testid="skill-result">
      <div className="flex items-center gap-3">
        <HexDie value={c.roll} label={t("fil.die", { faces: c.die, value: c.roll })} />
        <div className="min-w-0 flex-1">
          <div className="text-key tabular-nums" data-testid="skill-vs">
            {t(proficiency ? "fil.skillVs" : "fil.skillVsPercent", { roll: c.roll, target: c.target })}
          </div>
          <div className="text-body text-muted-foreground tabular-nums">
            {proficiency
              ? t("result.skill.proficiency", {
                  die: c.die,
                  stat: abilityText(t, c.ability ?? "", c.abilityScore ?? 0, c.modifier ?? 0),
                })
              : t("result.skill.bardSkill", { die: c.die, percent: c.target })}
          </div>
        </div>
        <Verdict ok={c.success} yes={t("result.skill.success")} no={t("result.skill.failure")} />
      </div>
      <dl className="grid grid-cols-3 gap-2">
        <Fact label={t("result.skill.die", { die: c.die })}>{c.roll}</Fact>
        <Fact label={t("result.target")}>{proficiency ? c.target : t("skills.percent", { value: c.target })}</Fact>
      </dl>
    </div>
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
          <span className="text-body font-medium tabular-nums" data-testid="dice-formula">
            {r.formula}
          </span>
          {r.groups.map((g, i) => (
            <div key={i} className="flex flex-wrap items-center gap-2" data-testid="dice-group">
              <span className={cn(labelClass, "tabular-nums")}>
                {g.negative ? "−" : ""}
                {dieName(g.sides)}
              </span>
              <ul className="flex flex-wrap gap-2">
                {g.rolls.map((v, j) => (
                  <DieChip key={j} value={v} faces={g.negative ? null : g.sides} />
                ))}
              </ul>
              <span className="text-body font-semibold tabular-nums" data-testid="dice-subtotal">
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
    <div className="flex items-start justify-between gap-3" data-testid="info-result">
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
            info.value.tone === "damage" ? "text-danger" : "text-heal",
          )}
          data-testid="info-value"
        >
          {formatSigned(info.value.amount)}
        </span>
      )}
    </div>
  );
}
