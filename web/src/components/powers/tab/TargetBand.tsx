import { useTranslation } from "react-i18next";
import { Minus, Plus } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { MAX_TARGET_LEN } from "../attack";
import { localized } from "@/i18n/localized";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCatalogue } from "./useCatalogue";
import { stepLevel, type TargetDraft } from "./bar";
import { heldMinds } from "./model";

/**
 * "Target": the opponent an attack is made on, entered once for the attack modes below (ADR 0047
 * item 2): a mind already held (a tangent or a contact row) or a new label, its mind open or closed,
 * its level and its defence. The bar reads it; nothing here sends a request.
 */
export function TargetBand({
  target,
  onChange,
}: {
  target: TargetDraft;
  onChange: (next: TargetDraft) => void;
}) {
  const { t, i18n } = useTranslation();
  const minds = heldMinds(useBuffsStore((s) => s.buffs));
  const defences = useCatalogue("defence_mode");
  const set = (patch: Partial<TargetDraft>) =>
    onChange({ ...target, ...patch });
  return (
    <div
      className="group-band flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg px-3 py-2"
      data-testid="target-band"
    >
      <span className="section-label mr-1">{t("powers.tab.target")}</span>
      <span
        role="radiogroup"
        aria-label={t("powers.tab.heldMinds")}
        className="contents"
      >
        {minds.map((m) => (
          <button
            key={m.name}
            type="button"
            role="radio"
            aria-checked={target.target === m.name}
            data-testid="held-mind"
            className={cn(
              "flex h-8 items-center gap-2 rounded-md px-2.5 text-body",
              target.target === m.name ? "chip chip-chosen" : "chip",
            )}
            onClick={() => set({ target: m.name })}
          >
            {m.name}
            <span className="text-label text-ink-muted">
              {m.tangents
                ? t("powers.tab.tangents", { count: m.tangents })
                : t("powers.tab.contact")}
            </span>
          </button>
        ))}
      </span>
      <input
        className={`${fieldClass} !w-40`}
        autoComplete="off"
        maxLength={MAX_TARGET_LEN}
        aria-label={t("powers.attack.target")}
        placeholder={t("powers.tab.newTarget")}
        data-testid="target-name"
        value={target.target}
        onChange={(e) => set({ target: e.target.value })}
      />
      <span
        role="radiogroup"
        aria-label={t("powers.attack.mind")}
        className="flex items-center gap-1.5"
      >
        <span className={labelClass}>{t("powers.attack.mind")}</span>
        {(["open", "closed"] as const).map((mind) => (
          <button
            key={mind}
            type="button"
            role="radio"
            aria-checked={target.mindSaid && target.mind === mind}
            className={cn(
              "h-7 rounded-md px-2 text-body",
              target.mindSaid && target.mind === mind
                ? "chip chip-chosen"
                : "chip",
            )}
            onClick={() => set({ mind, mindSaid: true })}
          >
            {t(`powers.attack.${mind}`)}
          </button>
        ))}
      </span>
      <span className="flex items-center gap-1">
        <span className={labelClass}>{t("powers.attack.level")}</span>
        <Button
          className="px-1.5"
          aria-label={t("powers.tab.levelLess")}
          onClick={() => set({ level: stepLevel(target.level, -1) })}
        >
          <Minus aria-hidden />
        </Button>
        <input
          className={`${fieldClass} !w-12 text-center tabular-nums`}
          inputMode="numeric"
          autoComplete="off"
          aria-label={t("powers.attack.level")}
          value={target.level}
          onChange={(e) => set({ level: e.target.value })}
        />
        <Button
          className="px-1.5"
          aria-label={t("powers.tab.levelMore")}
          onClick={() => set({ level: stepLevel(target.level, 1) })}
        >
          <Plus aria-hidden />
        </Button>
      </span>
      <label className="flex items-center gap-1.5">
        <span className={labelClass}>{t("powers.attack.defence")}</span>
        <select
          className={`${fieldClass} !w-auto`}
          value={target.defence}
          aria-label={t("powers.attack.defence")}
          onChange={(e) => set({ defence: e.target.value })}
        >
          <option value="">{t("powers.attack.noDefence")}</option>
          {defences.map((d) => (
            <option key={d.id} value={d.id}>
              {localized(i18n.language, d.name)}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}
