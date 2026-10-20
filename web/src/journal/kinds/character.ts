// The character's entries: XP awards (ADR 0015 step 4), a level-up (ADR 0030 step 7.3), a spell
// learned (step 20.2), skill points and skill armour (step 17), the proficiency picks (ADR 0037
// step 13), the AC override (ADR 0033 step 13),
// the preparation (ADR 0008) and the long rest (steps 11 and 12).
import {
  ArrowFatLinesUp,
  BookOpen,
  ListNumbers,
  Moon,
  ShieldChevron,
  TrendUp,
} from "@phosphor-icons/react";
import type { TFunction } from "i18next";
import { formatAc, formatSigned } from "@/lib/format";
import type { XpChangeRecord, XpEntry } from "@/types/xp";
import type { LearnResult, LevelUpRecord } from "@/types/character";
import type { LogEntry } from "@/types/log";
import type { KindView } from "../kindView";
import { skillName } from "../words";

/** Payload of xp_add, xp_edit and xp_remove entries (xpsvc changeRecord). */
interface XpPayload {
  entry: XpEntry;
  before?: XpEntry;
}

function decodeXp(e: LogEntry): XpPayload | null {
  const p = e.payload as Partial<XpChangeRecord> | null;
  const now = p?.entry;
  return now && typeof now.total === "number"
    ? { entry: now, before: p.before }
    : null;
}

const reason = (t: TFunction, x: XpEntry) => x.reason || t("xp.defaultReason");

export const xpAddKind: KindView<XpPayload> = {
  decode: decodeXp,
  icon: TrendUp,
  text: ({ entry }, { t }) =>
    t("log.xp.added", { total: entry.total, reason: reason(t, entry) }),
};

export const xpEditKind: KindView<XpPayload> = {
  decode: decodeXp,
  icon: TrendUp,
  text({ entry, before }, { t }) {
    if (!before || before.total === entry.total)
      return t("log.xp.editedSame", {
        total: entry.total,
        reason: reason(t, entry),
      });
    return t("log.xp.edited", {
      from: before.total,
      to: entry.total,
      reason: reason(t, entry),
    });
  },
};

export const xpRemoveKind: KindView<XpPayload> = {
  decode: decodeXp,
  icon: TrendUp,
  text: ({ entry }, { t }) =>
    t("log.xp.removed", { total: entry.total, reason: reason(t, entry) }),
};

/** The payload of a level_up entry (levelupsvc record, ADR 0030 step 7.3), or null. */
export function decodeLevelUp(e: LogEntry): LevelUpRecord | null {
  const p = e.payload as Partial<LevelUpRecord> | null;
  return p && typeof p.gain === "number" && typeof p.level === "number"
    ? (p as LevelUpRecord)
    : null;
}

/** A level-up roll: the die (and the value it counts as from Constitution 20), the Constitution
 * adjustment and the gain the server computed, or the fixed gain above the name level. */
function levelUpText(p: LevelUpRecord, t: TFunction): string {
  const hp = {
    level: p.level,
    gain: p.gain,
    from: p.hpMaxBefore,
    to: p.hpMaxAfter,
  };
  if (p.fixed || p.rolled === null) return t("log.levelUp.fixed", hp);
  const die =
    p.counted !== null && p.counted !== p.rolled
      ? t("log.levelUp.dieRaised", {
          die: p.hitDie,
          rolled: p.rolled,
          counted: p.counted,
        })
      : t("log.levelUp.die", { die: p.hitDie, rolled: p.rolled });
  const con = p.conAdjustment
    ? t("log.levelUp.con", { adjustment: formatSigned(p.conAdjustment) })
    : "";
  const manual = p.manual ? t("log.levelUp.manual") : "";
  return t("log.levelUp.rolled", { ...hp, die, con, manual });
}

export const levelUpKind: KindView<LevelUpRecord> = {
  decode: decodeLevelUp,
  icon: ArrowFatLinesUp,
  text: (p, { t }) => levelUpText(p, t),
  value: (p) => ({ text: formatSigned(p.gain), tone: "heal" }),
};

/** The payload of a spell_learn entry (learnsvc record, ADR 0030 step 20.2), or null. */
export function decodeSpellLearn(e: LogEntry): LearnResult | null {
  const p = e.payload as Partial<LearnResult> | null;
  return p && typeof p.spellId === "string" && typeof p.roll === "number"
    ? (p as LearnResult)
    : null;
}

export const spellLearnKind: KindView<LearnResult> = {
  decode: decodeSpellLearn,
  icon: BookOpen,
  // A chance-to-learn roll: the spell, the d100 against the chance, learned or failed.
  text(p, { t, spellName }) {
    const vars = {
      spell: spellName(p.spellId),
      roll: p.roll,
      chance: p.chance,
    };
    return p.learned
      ? t("log.learn.learned", vars)
      : t("log.learn.failed", { ...vars, level: p.retryLevel });
  },
};

/** The points per skill before and after (skillsvc, ADR 0030 step 17). */
interface SkillPointsPayload {
  before: Record<string, number>;
  after: Record<string, number>;
}

export const skillPointsKind: KindView<SkillPointsPayload> = {
  decode(e) {
    const p = e.payload as Partial<SkillPointsPayload> | null;
    return p && p.before && p.after
      ? { before: p.before, after: p.after }
      : null;
  },
  icon: ListNumbers,
  // "Skill points: Open Locks 30 → 45, Pick Pockets 0 → 15": each skill whose points changed.
  text(p, { t }) {
    const skills = [
      ...new Set([...Object.keys(p.before), ...Object.keys(p.after)]),
    ].filter((k) => (p.before[k] ?? 0) !== (p.after[k] ?? 0));
    const changes = skills
      .map((k) =>
        t("log.skillPoints.change", {
          name: skillName(t, k),
          from: p.before[k] ?? 0,
          to: p.after[k] ?? 0,
        }),
      )
      .join(", ");
    return t("log.skillPoints.text", { changes });
  },
};

/** Payload of a proficiencies entry (skillsvc picksRecord): each pool's picks before and after,
 * by id or weapon type and slots; `weapon` null for an entry logged before the weapon pool;
 * `languages` the languages before and after (ADR 0037 step 18), null for an entry logged before
 * they moved to the Skills tab. */
interface PicksPayload {
  before: Record<string, number>;
  after: Record<string, number>;
  weapon: {
    before: Record<string, number>;
    after: Record<string, number>;
  } | null;
  languages: { before: string[]; after: string[] } | null;
}

type PickList =
  | {
      nonweapon?: { id: string; slots: number }[];
      weapon?: { type: string; slots: number }[];
      languages?: string[];
    }
  | undefined;

/** The same languages, in any order (a save that only reorders them logs nothing). */
const sameLanguages = (a: string[], b: string[]) =>
  a.length === b.length && a.every((l) => b.includes(l));

const bySlots = (l: PickList): Record<string, number> | null =>
  l && Array.isArray(l.nonweapon)
    ? Object.fromEntries(l.nonweapon.map((x) => [x.id, x.slots]))
    : null;

const weaponBySlots = (l: PickList): Record<string, number> | null =>
  l && Array.isArray(l.weapon)
    ? Object.fromEntries(l.weapon.map((x) => [x.type, x.slots]))
    : null;

/** The keys whose slots changed between before and after (0: none), in first-seen order. */
const changed = (
  before: Record<string, number>,
  after: Record<string, number>,
) =>
  [...new Set([...Object.keys(before), ...Object.keys(after)])].filter(
    (k) => (before[k] ?? 0) !== (after[k] ?? 0),
  );

export const proficienciesKind: KindView<PicksPayload> = {
  decode(e) {
    const p = e.payload as { before?: PickList; after?: PickList } | null;
    const before = bySlots(p?.before);
    const after = bySlots(p?.after);
    if (!before || !after) return null;
    const wb = weaponBySlots(p?.before);
    const wa = weaponBySlots(p?.after);
    const lb = p?.before?.languages;
    const la = p?.after?.languages;
    return {
      before,
      after,
      weapon: wb || wa ? { before: wb ?? {}, after: wa ?? {} } : null,
      languages:
        Array.isArray(lb) && Array.isArray(la)
          ? { before: lb, after: la }
          : null,
    };
  },
  icon: ListNumbers,
  // "Proficiency slots: Tumbling 1 → 2, Tracking 0 → 2": each pick whose slots changed (0: none);
  // then "Weapon proficiency slots: Long sword 1 → 2 (specialization)". A weapon pick of more than
  // one slot is a specialization: the server refuses any other count (ADR 0037 step 9). Then
  // "Languages: 2 → 3", the count of the languages known (ADR 0037 step 18), when they changed.
  text(p, { t }) {
    const parts: string[] = [];
    const ids = changed(p.before, p.after);
    const l = p.languages;
    const languagesChanged = !!l && !sameLanguages(l.before, l.after);
    if (ids.length > 0 || (!p.weapon && !languagesChanged))
      parts.push(
        t("log.proficiencies.text", {
          changes: ids
            .map((k) =>
              t("log.proficiencies.change", {
                name: skillName(t, k),
                from: p.before[k] ?? 0,
                to: p.after[k] ?? 0,
              }),
            )
            .join(", "),
        }),
      );
    const w = p.weapon;
    const types = w ? changed(w.before, w.after) : [];
    if (w && types.length > 0)
      parts.push(
        t("log.proficiencies.weaponText", {
          changes: types
            .map((k) => {
              const to = w.after[k] ?? 0;
              const args = {
                name: t(`weapons.types.${k}`, { defaultValue: k }),
                from: w.before[k] ?? 0,
                to,
              };
              return to > 1
                ? t("log.proficiencies.specialized", args)
                : t("log.proficiencies.change", args);
            })
            .join(", "),
        }),
      );
    if (l && languagesChanged)
      parts.push(
        t("log.proficiencies.languages", {
          from: l.before.length,
          to: l.after.length,
        }),
      );
    return parts.join(" · ");
  },
};

export const skillArmorKind: KindView<{ before?: string; after: string }> = {
  decode(e) {
    const p = e.payload as { before?: string; after?: string } | null;
    return p && typeof p.after === "string"
      ? { before: p.before, after: p.after }
      : null;
  },
  icon: ListNumbers,
  // "Skill armour: None → Elven chain" (charactersvc, ADR 0030 step 1): the armour before and after.
  text(p, { t }) {
    const name = (a: string) =>
      t(`characters.armors.${a || "leather"}`, { defaultValue: a });
    return t("log.skillArmor.text", {
      from: name(p.before ?? ""),
      to: name(p.after),
    });
  },
};

/** The AC override after the change; null once cleared (acsvc, ADR 0033 step 13). */
interface AcOverridePayload {
  after?: { value: number; label?: string } | null;
}

export const acOverrideKind: KindView<AcOverridePayload> = {
  decode: (e) => e.payload as AcOverridePayload | null,
  icon: ShieldChevron,
  // "AC overridden: 4 (Shield spell)" / "AC override cleared".
  text(p, { t }) {
    if (!p.after) return t("log.acOverride.cleared");
    return p.after.label
      ? t("log.acOverride.setWith", {
          value: formatAc(p.after.value),
          label: p.after.label,
        })
      : t("log.acOverride.set", { value: formatAc(p.after.value) });
  },
};

/** Payload of spell_prep entries (prepsvc prepRecord): each spell once, with its copies. */
interface PrepPayload {
  spells: { id: string; name: string; count: number }[];
}

export const spellPrepKind: KindView<PrepPayload> = {
  decode(e) {
    const p = e.payload as Partial<PrepPayload> | null;
    return p && Array.isArray(p.spells) ? { spells: p.spells } : null;
  },
  icon: BookOpen,
  text(p, { t, spellName }) {
    if (p.spells.length === 0) return t("log.prepNone");
    const spells = p.spells
      .map((s) => {
        const name = spellName(s.id, s.name);
        return s.count > 1 ? `${name} ×${s.count}` : name;
      })
      .join(", ");
    return t("log.prep", { spells });
  },
};

/** Payload of long_rest entries (restsvc restRecord). */
interface RestPayload {
  slotsReset: number;
  itemsRefilled?: string[];
}

export const longRestKind: KindView<RestPayload> = {
  decode(e) {
    const p = e.payload as Partial<RestPayload> | null;
    return p && typeof p.slotsReset === "number" ? (p as RestPayload) : null;
  },
  icon: Moon,
  text: (p, { t }) =>
    t("log.rest", {
      slots: p.slotsReset,
      items: p.itemsRefilled?.length ?? 0,
    }),
};
