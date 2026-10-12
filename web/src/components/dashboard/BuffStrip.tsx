import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Pencil, Plus, Shield, Trash2, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import {
  BUFF_EFFECT_KEYS,
  type BuffEffect,
  type BuffEffectKey,
  type BuffEffects,
} from "@/types/buffs";
import { captionClass, fieldClass } from "./fieldStyles";

/**
 * Active buffs as badges (remaining rounds / charges, "absorbed a hit" for charge buffs, edit,
 * dismiss), plus a form to add one. Every change is logged server-side and voidable from the log.
 */
export function BuffStrip() {
  const { t } = useTranslation();
  const buffs = useBuffsStore((s) => s.buffs);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const edited = buffs.find((b) => b.id === editing);

  return (
    <div
      className="flex min-h-9 flex-wrap items-center gap-2"
      role="group"
      aria-label={t("buffs.title")}
    >
      <span className={captionClass}>{t("buffs.title")}</span>
      <ul className="flex flex-wrap items-center gap-1.5">
        {buffs.map((b) => (
          <BuffBadge
            key={b.id}
            buff={b}
            onEdit={() => {
              setAdding(false);
              setEditing(b.id);
            }}
          />
        ))}
        {buffs.length === 0 && (
          <li className="text-sm italic text-muted-foreground">
            {t("buffs.none")}
          </li>
        )}
      </ul>
      {edited ? (
        <EditBuffForm
          key={edited.id}
          buff={edited}
          onDone={() => setEditing(null)}
        />
      ) : adding ? (
        <AddBuffForm onDone={() => setAdding(false)} />
      ) : (
        <Button size="sm" variant="outline" onClick={() => setAdding(true)}>
          <Plus />
          {t("buffs.add")}
        </Button>
      )}
    </div>
  );
}

function BuffBadge({
  buff: b,
  onEdit,
}: {
  buff: BuffEffect;
  onEdit: () => void;
}) {
  const { t } = useTranslation();
  const { triggerBuff, dismissBuff } = useBuffsStore();
  const { run, busy } = useTableStore();
  const inactive = b.applies === false;

  return (
    <li data-testid="buff" data-buff={b.name} data-applies={String(!inactive)}>
      <Badge
        variant="outline"
        className={cn(
          "h-7 gap-1.5 pr-1 text-sm",
          inactive && "border-dashed text-muted-foreground",
        )}
        title={
          inactive && b.effects.min_str !== undefined
            ? t("buffs.inactiveHint", { min: b.effects.min_str })
            : undefined
        }
      >
        <span className="font-medium">{b.name}</span>
        {b.duration > 0 && (
          <span
            className="font-mono text-xs text-muted-foreground"
            data-testid="buff-rounds"
          >
            {t("buffs.rounds", { count: b.duration })}
          </span>
        )}
        {b.charges > 0 && (
          <span
            className="font-mono text-xs text-muted-foreground"
            data-testid="buff-charges"
          >
            {t("buffs.charges", { count: b.charges })}
          </span>
        )}
        {inactive && (
          <span className="text-xs italic">
            {t("buffs.inactive", { min: b.effects.min_str ?? "?" })}
          </span>
        )}
        {b.charges > 0 && (
          <Button
            size="xs"
            variant="secondary"
            disabled={busy}
            aria-label={t("buffs.absorb", { name: b.name })}
            title={t("buffs.absorb", { name: b.name })}
            onClick={() => void run(() => triggerBuff(b.id))}
          >
            <Shield />
            {t("buffs.absorbShort")}
          </Button>
        )}
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={t("buffs.edit", { name: b.name })}
          title={t("buffs.edit", { name: b.name })}
          onClick={onEdit}
        >
          <Pencil />
        </Button>
        <Button
          size="icon-xs"
          variant="ghost"
          disabled={busy}
          aria-label={t("buffs.dismiss", { name: b.name })}
          title={t("buffs.dismiss", { name: b.name })}
          onClick={() => void run(() => dismissBuff(b.id))}
        >
          <X />
        </Button>
      </Badge>
    </li>
  );
}

/** Parses a non-negative integer field; NaN when invalid. */
function count(v: string): number {
  return /^\d+$/.test(v.trim()) ? Number(v) : Number.NaN;
}

function EditBuffForm({
  buff,
  onDone,
}: {
  buff: BuffEffect;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const editBuff = useBuffsStore((s) => s.editBuff);
  const { run, busy } = useTableStore();
  const [rounds, setRounds] = useState(String(buff.duration));
  const [charges, setCharges] = useState(String(buff.charges));
  const r = count(rounds);
  const c = count(charges);
  const valid = !Number.isNaN(r) && !Number.isNaN(c);

  async function submit() {
    if (!valid) return;
    const ok = await run(() => editBuff(buff.id, { duration: r, charges: c }));
    if (ok) onDone();
  }

  return (
    <form
      className="flex flex-wrap items-center gap-1.5 rounded-lg border bg-muted/40 p-1.5"
      aria-label={t("buffs.editLabel", { name: buff.name })}
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <span className="px-1 text-sm font-medium">{buff.name}</span>
      <label className="flex items-center gap-1">
        <span className={captionClass}>{t("buffs.roundsLeft")}</span>
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
        <span className={captionClass}>{t("buffs.chargesLeft")}</span>
        <input
          className={`${fieldClass} w-16`}
          type="number"
          min={0}
          value={charges}
          onChange={(e) => setCharges(e.target.value)}
        />
      </label>
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

function AddBuffForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const activateBuff = useBuffsStore((s) => s.activateBuff);
  const { run, busy } = useTableStore();
  const [name, setName] = useState("");
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
    const ok = await run(() =>
      activateBuff({ name: name.trim(), duration: r, charges: c, effects }),
    );
    if (ok) onDone();
  }

  return (
    <form
      className="flex flex-wrap items-center gap-1.5 rounded-lg border bg-muted/40 p-1.5"
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
        <span className={captionClass}>{t("buffs.roundsLeft")}</span>
        <input
          className={`${fieldClass} w-16`}
          type="number"
          min={0}
          value={rounds}
          onChange={(e) => setRounds(e.target.value)}
        />
      </label>
      <label className="flex items-center gap-1" title={t("buffs.chargesHint")}>
        <span className={captionClass}>{t("buffs.chargesLeft")}</span>
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
            <Trash2 />
          </Button>
        </span>
      ))}
      <Button
        size="sm"
        type="button"
        variant="outline"
        onClick={() => setRows([...rows, { key: "weapon_buff", value: "1" }])}
      >
        <Plus />
        {t("buffs.addEffect")}
      </Button>
      <Button size="sm" type="submit" disabled={!valid || busy}>
        {t("buffs.activate")}
      </Button>
      <Button size="sm" type="button" variant="ghost" onClick={onDone}>
        {t("buffs.cancel")}
      </Button>
    </form>
  );
}
