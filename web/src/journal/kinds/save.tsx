// The saving throw entry (combatsvc.SaveBreakdown, ADR 0006): the payload in effect is the latest
// adjustment's.
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Shield } from "@phosphor-icons/react";
import { useSpellName, type SpellNamer } from "@/i18n/spellName";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { formatSigned } from "@/lib/format";
import { itemNamer } from "@/lib/itemPowers";
import { stepAdjustment } from "@/lib/saveAdjust";
import type { Immunity, Modifier, SaveBreakdown } from "@/types/combat";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { modLabel } from "../words";
import { AdjustCheck, Fact, HexDie, ResultLine, Verdict } from "../parts";

/** The save's payload in effect (an adjusted save's is the latest amend's), with its entry id. */
export function decodeSave(e: LogEntry): SaveBreakdown | null {
  const p = (e.effectivePayload ?? e.payload) as Partial<SaveBreakdown> | null;
  return p &&
    typeof p.passed === "boolean" &&
    typeof p.d20Roll === "number" &&
    typeof p.rollTotal === "number" &&
    typeof p.adjustedTarget === "number" &&
    typeof p.margin === "number" &&
    typeof p.category === "string"
    ? { ...(p as SaveBreakdown), logEntryId: e.id }
    : null;
}

/** A saving throw as the card shows it, from a save entry's payload or the save endpoint's result. */
export interface SaveView {
  category: string;
  d20: number;
  /** The sheet's target. */
  target: number;
  /** The modifiers applied when rolling (buffs). */
  modifiers: Modifier[];
  /** Moves the target, + easier, − harder (ADR 0006 step 5); 0: none. */
  adjustment: number;
  adjustmentReason?: string;
  /** d20 + modifiers, the adjustment left out. */
  rollTotal: number;
  /** The target the save was judged against: target − adjustment (the server's). */
  adjustedTarget: number;
  passed: boolean;
  /** rollTotal − adjustedTarget, as computed by the server. */
  margin: number;
  /** The spell the save was against (a catalogue id); absent: none named. */
  spellId?: string;
  /** Set when the character was immune to that spell (ADR 0006 step 10.2): nothing was rolled. */
  immunity?: Immunity;
  logEntryId?: number;
}

/** The card's view of a save, from a save entry's payload or the save endpoints' answer (one shape). */
export function saveView(p: SaveBreakdown): SaveView {
  return {
    category: p.category,
    d20: p.d20Roll,
    target: p.target,
    modifiers: p.modifiers ?? [],
    adjustment: p.adjustment ?? 0,
    adjustmentReason: p.adjustmentReason,
    rollTotal: p.rollTotal,
    adjustedTarget: p.adjustedTarget,
    passed: p.passed,
    margin: p.margin,
    spellId: p.spellId,
    immunity: p.immunity,
    logEntryId: p.logEntryId,
  };
}

function categoryName(t: TFunction, category: string): string {
  return t(`saves.categories.${category}`, { defaultValue: category });
}

/** The roll side of a save as the journal words it: "d20 12", or "d20 12 +2 = 14" with buffs. */
function saveRollText(p: SaveBreakdown): string {
  return p.rollTotal === p.d20Roll
    ? `d20 ${p.d20Roll}`
    : `d20 ${p.d20Roll} ${formatSigned(p.rollTotal - p.d20Roll)} = ${p.rollTotal}`;
}

function saveText(
  p: SaveBreakdown,
  t: TFunction,
  spellName: SpellNamer,
): string {
  const verdict = (passed: boolean) =>
    t(passed ? "result.passed" : "result.failed").toUpperCase();
  // ADR 0006 step 10.2: immune to the spell, nothing was rolled.
  if (p.immunity)
    return t("log.saveThrow.immune", {
      name: categoryName(t, p.category),
      spell: spellName(p.spellId ?? "", p.spellId),
      score: p.immunity.score,
      verdict: verdict(p.passed),
    });
  const args = {
    name: categoryName(t, p.category),
    roll: saveRollText(p),
    target: p.target,
  };
  if (p.previousPassed === undefined)
    return t("log.saveThrow.rolled", { ...args, verdict: verdict(p.passed) });
  const change = {
    ...args,
    from: verdict(p.previousPassed),
    to: verdict(p.passed),
  };
  if (p.adjustment === 0) return t("log.saveThrow.removed", change);
  return t("log.saveThrow.adjusted", {
    ...change,
    adjustedTarget: p.adjustedTarget,
    adjustment: formatSigned(p.adjustment),
    reason: p.adjustmentReason ? ` ${p.adjustmentReason}` : "",
  });
}

export const saveKind: KindView<SaveBreakdown, SaveBreakdown> = {
  decode: decodeSave,
  icon: Shield,
  text: (p, w) => saveText(p, w.t, w.spellName),
  // An immune save rolled no d20: no value to show (ADR 0006 step 10.2).
  value: (p) =>
    p.immunity
      ? null
      : { text: String(p.d20Roll), tone: p.passed ? "pass" : "fail" },
  rolled: (p) => p === null || !p.immunity,
  subtitle: (p, w) => categoryName(w.t, p.category),
  short: (p, { t }, _e, value) => ({
    value,
    what: t("lastShort.save", { name: categoryName(t, p.category) }),
    verdict: p.immunity
      ? { text: t("lastShort.verdict.immune"), tone: "pass" }
      : p.passed
        ? { text: t("lastShort.verdict.passed"), tone: "pass" }
        : { text: t("lastShort.verdict.failed"), tone: "fail" },
  }),
  card: ({ payload, interactive }) => (
    <SaveCard save={saveView(payload)} interactive={interactive} />
  ),
  live: (r) => r,
};

/**
 * Save: d20, "roll total vs adjusted target+", the verdict, the buff modifiers, the adjustment on
 * the target side (ADR 0006 step 5) and (latest only) the adjustment row.
 */
function SaveCard({
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
  const spellName = useSpellName();
  const name = categoryName(t, s.category);
  const spell = s.spellId ? spellName(s.spellId, s.spellId) : undefined;
  const heading = spell
    ? t("result.saveAgainst", { name, spell })
    : t("result.saveHeading", { name });
  // ADR 0006 step 10.2: immune to the spell named, the save rolled nothing; the server's reason.
  if (s.immunity)
    return (
      <div className="flex flex-col gap-1" data-testid="save-result">
        <ResultLine
          verdict={
            <Verdict
              ok={s.passed}
              yes={t("result.passed")}
              no={t("result.failed")}
            />
          }
        >
          <span className="text-key" data-testid="save-immune">
            {t("result.immune")}
          </span>
        </ResultLine>
        <div className="text-body text-muted-foreground">{heading}</div>
        <p className="text-body break-words" data-testid="save-immunity">
          {t("result.immunity", {
            score: s.immunity.score,
            spell: spell ?? "",
          })}
        </p>
      </div>
    );
  return (
    <div className="flex flex-col gap-3" data-testid="save-result">
      <div className="flex items-center gap-3">
        <HexDie value={s.d20} label={t("journal.d20", { value: s.d20 })} />
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
              {t("journal.saveVs", {
                total: s.rollTotal,
                target: s.adjustedTarget,
              })}
            </span>
          </ResultLine>
          <div className="text-body text-muted-foreground">{heading}</div>
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
      onApply={(value, reason) => adjustLastSave(entryId, value, reason)}
    />
  );
}
