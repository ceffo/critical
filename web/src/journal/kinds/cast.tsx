// The cast entry (castsvc.CastBreakdown, ADR 0008): a spell cast, received or an effect; the cast
// endpoint answers with the same payload plus the catalogue spell and the slots.
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { MagicWand } from "@phosphor-icons/react";
import { useSpellName, type SpellNamer } from "@/i18n/spellName";
import { type InventoryNamer, useInventoryName } from "@/i18n/inventoryName";
import { splitText } from "@/lib/slotSplit";
import { modeName } from "@/lib/spellMode";
import { rollLabel } from "@/lib/spellDice";
import {
  CastResultView,
  SpellFailureLine,
} from "@/components/dashboard/CastResultView";
import { ExposureControls } from "@/components/dashboard/ExposureControls";
import type { CastBreakdown, CastResult } from "@/types/spells";
import type { LogEntry } from "@/types/log";
import type { KindView, ShortVerdict } from "../kindView";
import { chips, type DieChipView } from "../dice";
import { counters } from "../words";
import { DieChip, HexDie, ResultLine, Total, Verdict } from "../parts";

/** A cast as the card shows it: its payload, and the endpoint's whole answer for this session's
 * cast (the catalogue spell's texts, the slots). */
export interface CastPayload {
  breakdown: CastBreakdown;
  answer: CastResult | null;
}

export function decodeCast(e: LogEntry): CastBreakdown | null {
  const p = e.payload as Partial<CastBreakdown> | null;
  return p && typeof p.action === "string" && typeof p.spellId === "string"
    ? (p as CastBreakdown)
    : null;
}

function castText(
  p: CastBreakdown,
  t: TFunction,
  spellName: SpellNamer,
  names: InventoryNamer,
): string {
  const spell = spellName(p.spellId, p.spell);
  const head = t(`log.cast.${p.action}`, { spell, level: p.casterLevel });
  const details: string[] = [];
  // ADR 0030 step 14: the spell failure d100 first; a failed spell has nothing else.
  if (p.failure)
    details.push(
      t(p.failure.failed ? "log.cast.spellFailed" : "log.cast.spellHeld", {
        roll: p.failure.d100,
        chance: p.failure.chance,
      }),
    );
  if (p.mode) details.push(modeName(t, p.spellId, p.mode));
  if (p.buff?.applied) {
    details.push(counters(t, p.buff.rounds, p.buff.charges));
    if (p.buff.strGain)
      details.push(t("log.cast.strGain", { gain: p.buff.strGain }));
  }
  if (p.damage)
    details.push(
      t("log.cast.damage", { total: p.damage.total, type: p.damage.type }),
    );
  if (p.extraSlots && p.extraSlots.length > 0)
    details.push(
      t("log.cast.extraSlots", { slots: splitText(t, p.extraSlots) }),
    );
  if (p.item)
    details.push(
      t("log.cast.item", {
        name: names.item(p.item.itemId, p.item.name),
        left: p.item.remaining,
      }),
    );
  if (p.attack)
    details.push(
      t(p.attack.hit ? "log.cast.attackHit" : "log.cast.attackMiss", {
        d20: p.attack.d20,
        ac: p.attack.targetAc,
      }),
    );
  return details.length > 0 ? `${head}: ${details.join(", ")}` : head;
}

/** One damage instance of a cast: its dice as chips and its total; `parts` when the damage has parts. */
export interface CastInstanceView {
  dice: DieChipView[];
  total: number;
  parts?: CastPartView[];
  /** The round or instance of a damage made of dice steps; absent without steps. */
  step?: number;
}

/** One part of a damage instance: its dice as chips (none for a flat part), its total and what a save does to it. */
export interface CastPartView {
  dice: DieChipView[];
  total: number;
  save: string;
}

/**
 * The dice instances of a cast's damage, from its payload; null on a cast logged before they were
 * recorded (the card then shows the total only).
 */
export function castInstances(c: CastBreakdown): CastInstanceView[] | null {
  const rolls = c.damage?.rolls;
  if (!rolls || rolls.length === 0) return null;
  const saves = c.damage?.parts;
  return rolls.map((r) => {
    const view: CastInstanceView = { dice: chips(r.dice), total: r.total };
    if (r.step) view.step = r.step;
    if (r.parts && saves && r.parts.length === saves.length) {
      view.parts = r.parts.map((p, i) => ({
        dice: chips(p.dice),
        total: p.total,
        save: saves[i].save,
      }));
    }
    return view;
  });
}

function castVerdict(p: CastBreakdown, t: TFunction): ShortVerdict | null {
  if (p.failure?.failed)
    return { text: t("lastShort.verdict.spellFailed"), tone: "fail" };
  if (!p.attack) return null;
  return p.attack.hit
    ? { text: t("lastShort.verdict.hit"), tone: "pass" }
    : { text: t("lastShort.verdict.miss"), tone: "fail" };
}

export const castKind: KindView<CastPayload, CastResult> = {
  decode(e) {
    const breakdown = decodeCast(e);
    return breakdown ? { breakdown, answer: null } : null;
  },
  icon: MagicWand,
  text: ({ breakdown }, w) => castText(breakdown, w.t, w.spellName, w.names),
  value({ breakdown: c }) {
    // ADR 0030 step 14: a failed spell's d100 is the whole result.
    if (c.failure?.failed)
      return { text: String(c.failure.d100), tone: "fail" };
    if (c.damage) return { text: String(c.damage.total), tone: "ink" };
    if (c.attack)
      return {
        text: String(c.attack.d20),
        tone: c.attack.hit ? "pass" : "fail",
      };
    return null;
  },
  // A cast is in the Rolls filter when it rolled damage or a to-hit d20.
  rolled: (p) =>
    p !== null &&
    (p.breakdown.damage !== undefined || p.breakdown.attack !== undefined),
  subtitle: ({ breakdown: c }, w) => w.spellName(c.spellId, c.spell),
  short: ({ breakdown: c }, w, _e, value) => ({
    value,
    what: w.spellName(c.spellId, c.spell),
    verdict: castVerdict(c, w.t),
  }),
  card: ({ payload, interactive }) =>
    payload.answer ? (
      <CastResultView result={payload.answer} />
    ) : (
      <CastEntry cast={payload.breakdown} interactive={interactive} />
    ),
  live: (r) => ({ breakdown: r, answer: r }),
};

/** A cast from its journal payload: who cast it, the buff, the to-hit roll, the damage, the item. */
function CastEntry({
  cast: c,
  interactive,
}: {
  cast: CastBreakdown;
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
            label={t("journal.d20", { value: c.attack.d20 })}
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
                {t("journal.castVsAc", { ac: c.attack.targetAc })}
              </span>
            </ResultLine>
          </div>
        </div>
      )}
      {c.buff?.applied && (
        <p className="text-body" data-testid="cast-entry-buff">
          {t("journal.castBuff", {
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
