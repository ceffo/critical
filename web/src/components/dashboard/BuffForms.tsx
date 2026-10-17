import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { buffName, useSpellName } from "@/i18n/spellName";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import {
  BUFF_EFFECT_KEYS,
  type BuffEffect,
  type BuffEffectKey,
  type BuffEffects,
} from "@/types/buffs";
import { fieldClass } from "./fieldStyles";

// The add and edit forms of the effects timeline. Every change is logged server-side and voidable
// from the log; the server stamps the round an effect starts in.

/** Field caption: sentence case, readable size (ADR 0009 house rules). */
const labelClass = "text-label text-muted-foreground";

/** Parses a non-negative integer field; NaN when invalid. */
function count(v: string): number {
  return /^\d+$/.test(v.trim()) ? Number(v) : Number.NaN;
}

export function EditBuffForm({
  buff,
  onDone,
}: {
  buff: BuffEffect;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const name = buffName(useSpellName(), buff);
  const editBuff = useBuffsStore((s) => s.editBuff);
  const { run, busy } = useTableStore();
  const [rounds, setRounds] = useState(String(buff.duration));
  const [charges, setCharges] = useState(String(buff.charges));
  const [note, setNote] = useState(buff.note);
  const r = count(rounds);
  const c = count(charges);
  const valid = !Number.isNaN(r) && !Number.isNaN(c);

  async function submit() {
    if (!valid) return;
    const ok = await run(() =>
      editBuff(buff.id, { duration: r, charges: c, note }),
    );
    if (ok) onDone();
  }

  return (
    <form
      className="flex flex-wrap items-center gap-2 rounded-lg bg-raised p-2"
      aria-label={t("buffs.editLabel", { name })}
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <span className="px-1 text-body font-medium">{name}</span>
      <label className="flex items-center gap-1">
        <span className={labelClass}>{t("buffs.roundsLeft")}</span>
        <input
          className={`${fieldClass} w-16`}
          type="number"
          min={0}
          value={rounds}
          onChange={(e) => setRounds(e.target.value)}
          autoFocus
        />
      </label>
      <label className="flex items-center gap-1">
        <span className={labelClass}>{t("buffs.chargesLeft")}</span>
        <input
          className={`${fieldClass} w-16`}
          type="number"
          min={0}
          value={charges}
          onChange={(e) => setCharges(e.target.value)}
        />
      </label>
      <input
        className={`${fieldClass} w-56`}
        aria-label={t("buffs.note")}
        placeholder={t("buffs.notePlaceholder")}
        value={note}
        onChange={(e) => setNote(e.target.value)}
      />
      <Button size="sm" type="submit" disabled={!valid || busy}>
        {t("buffs.save")}
      </Button>
      <Button size="sm" type="button" variant="ghost" onClick={onDone}>
        {t("buffs.cancel")}
      </Button>
    </form>
  );
}

interface EffectRow {
  key: BuffEffectKey;
  value: string;
}

export function AddBuffForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const { activateBuff, saveBuffTemplate } = useBuffsStore();
  const { run, busy } = useTableStore();
  const [name, setName] = useState("");
  const [note, setNote] = useState("");
  const [saveIt, setSaveIt] = useState(false);
  const [rounds, setRounds] = useState("0");
  const [charges, setCharges] = useState("0");
  const [rows, setRows] = useState<EffectRow[]>([]);

  const r = count(rounds);
  const c = count(charges);
  const parsed = rows.map((row) => Number.parseFloat(row.value));
  const valid =
    name.trim() !== "" &&
    !Number.isNaN(r) &&
    !Number.isNaN(c) &&
    parsed.every((v) => !Number.isNaN(v));

  function setRow(i: number, row: Partial<EffectRow>) {
    setRows(rows.map((old, j) => (j === i ? { ...old, ...row } : old)));
  }

  async function submit() {
    if (!valid) return;
    const effects: BuffEffects = {};
    rows.forEach((row, i) => {
      effects[row.key] = parsed[i];
    });
    const buff = { name: name.trim(), duration: r, charges: c, effects, note };
    if (!(await run(() => activateBuff(buff)))) return;
    if (saveIt && !(await run(() => saveBuffTemplate(buff)))) {
      // The buff is active already: close the form so a second Add cannot
      // activate it again, and say what was not saved.
      useTableStore.setState({
        error: `${useTableStore.getState().error ?? ""} — ${t("buffs.saveFailedAfterAdd", { name: buff.name })}`,
      });
    }
    onDone();
  }

  return (
    <form
      className="flex flex-wrap items-center gap-2"
      aria-label={t("buffs.add")}
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <input
        className={`${fieldClass} w-40`}
        aria-label={t("buffs.name")}
        placeholder={t("buffs.name")}
        value={name}
        onChange={(e) => setName(e.target.value)}
        autoFocus
      />
      <label className="flex items-center gap-1" title={t("buffs.roundsHint")}>
        <span className={labelClass}>{t("buffs.roundsLeft")}</span>
        <input
          className={`${fieldClass} w-16`}
          type="number"
          min={0}
          value={rounds}
          onChange={(e) => setRounds(e.target.value)}
        />
      </label>
      <label className="flex items-center gap-1" title={t("buffs.chargesHint")}>
        <span className={labelClass}>{t("buffs.chargesLeft")}</span>
        <input
          className={`${fieldClass} w-16`}
          type="number"
          min={0}
          value={charges}
          onChange={(e) => setCharges(e.target.value)}
        />
      </label>
      {rows.map((row, i) => (
        <span
          key={i}
          className="flex items-center gap-1"
          data-testid="effect-row"
        >
          <select
            className={`${fieldClass} w-36`}
            aria-label={t("buffs.effect")}
            value={row.key}
            onChange={(e) =>
              setRow(i, { key: e.target.value as BuffEffectKey })
            }
          >
            {BUFF_EFFECT_KEYS.map((k) => (
              <option key={k} value={k}>
                {t(`buffs.effects.${k}`)}
              </option>
            ))}
          </select>
          <input
            className={`${fieldClass} w-16`}
            type="number"
            step="any"
            aria-label={t("buffs.effectValue")}
            value={row.value}
            onChange={(e) => setRow(i, { value: e.target.value })}
          />
          <Button
            size="icon-xs"
            type="button"
            variant="ghost"
            aria-label={t("buffs.removeEffect")}
            title={t("buffs.removeEffect")}
            onClick={() => setRows(rows.filter((_, j) => j !== i))}
          >
            <Trash />
          </Button>
        </span>
      ))}
      <Button
        size="sm"
        type="button"
        onClick={() => setRows([...rows, { key: "weapon_buff", value: "1" }])}
      >
        <Plus />
        {t("buffs.addEffect")}
      </Button>
      <input
        className={`${fieldClass} w-56`}
        aria-label={t("buffs.note")}
        placeholder={t("buffs.notePlaceholder")}
        value={note}
        onChange={(e) => setNote(e.target.value)}
      />
      <label className="flex items-center gap-1 text-body">
        <input
          type="checkbox"
          checked={saveIt}
          onChange={(e) => setSaveIt(e.target.checked)}
        />
        {t("buffs.saveForReuse")}
      </label>
      <Button size="sm" type="submit" disabled={!valid || busy}>
        {t("buffs.activate")}
      </Button>
      <Button size="sm" type="button" variant="ghost" onClick={onDone}>
        {t("buffs.cancel")}
      </Button>
    </form>
  );
}
