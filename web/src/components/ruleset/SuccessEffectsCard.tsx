import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { saveSuccessEffects } from "@/api/ruleset";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import type { SuccessEffect } from "@/types/ruleset";

interface Draft {
  proficiencyId: string;
  ac: string;
  rounds: string;
}

const toDrafts = (effects: SuccessEffect[]): Draft[] =>
  effects.map((e) => ({
    proficiencyId: e.proficiencyId,
    ac: String(e.ac),
    rounds: String(e.rounds),
  }));

/** Empty or non-numeric text becomes NaN, which the server refuses. */
const num = (s: string): number => (s.trim() === "" ? Number.NaN : Number(s));

/**
 * The buff a successful check of a proficiency puts on the character (ADR 0041 step 2.4): the AC
 * change (descending AC, negative betters it) and the rounds it lasts. The proficiency is worded
 * from its id; its values are edited here and the server validates them on save.
 */
export function SuccessEffectsCard({ initial }: { initial: SuccessEffect[] }) {
  const { t } = useTranslation();
  const [rows, setRows] = useState<Draft[]>(() => toDrafts(initial));
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  function edit(i: number, field: "ac" | "rounds", value: string) {
    setSaved(false);
    setRows((rs) => rs.map((r, j) => (j === i ? { ...r, [field]: value } : r)));
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      const next = await saveSuccessEffects(
        rows.map((r) => ({
          proficiencyId: r.proficiencyId,
          ac: num(r.ac),
          rounds: num(r.rounds),
        })),
      );
      setRows(toDrafts(next));
      setSaved(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  const nameOf = (id: string) => t(`skills.names.${id}`, { defaultValue: id });

  return (
    <section
      aria-labelledby="ruleset-effects"
      className="zone-card flex min-w-0 flex-col"
      data-testid="ruleset-success-effects"
    >
      <div className="card-band flex items-center px-4 py-2">
        <h2 id="ruleset-effects" className="section-label">
          {t("ruleset.effects.title")}
        </h2>
      </div>
      <div className="flex flex-col gap-2 px-4 pt-3 pb-3">
        <p className="text-body text-muted-foreground">
          {t("ruleset.effects.help")}
        </p>
        {rows.length === 0 ? (
          <p className="text-body text-muted-foreground">
            {t("ruleset.effects.none")}
          </p>
        ) : (
          <div className="grid grid-cols-[minmax(0,2fr)_1fr_1fr] items-end gap-2">
            <span className={labelClass}>
              {t("ruleset.effects.proficiency")}
            </span>
            <span className={labelClass}>{t("ruleset.effects.ac")}</span>
            <span className={labelClass}>{t("ruleset.effects.rounds")}</span>
            {rows.map((r, i) => (
              <div
                key={r.proficiencyId}
                role="group"
                aria-label={nameOf(r.proficiencyId)}
                className="contents"
              >
                <span className="py-1 text-body">
                  {nameOf(r.proficiencyId)}
                </span>
                {(["ac", "rounds"] as const).map((f) => (
                  <input
                    key={f}
                    className={fieldClass}
                    type="number"
                    inputMode="numeric"
                    aria-label={t(`ruleset.effects.${f}`)}
                    value={r[f]}
                    onChange={(e) => edit(i, f, e.target.value)}
                  />
                ))}
              </div>
            ))}
          </div>
        )}
        {error && (
          <p role="alert" className="text-body text-danger">
            {error}
          </p>
        )}
        {saved && (
          <p role="status" className="text-body text-muted-foreground">
            {t("ruleset.effects.saved")}
          </p>
        )}
        {rows.length > 0 && (
          <div className="flex gap-2">
            <Button
              type="button"
              disabled={saving}
              title={t("ruleset.effects.saveHint")}
              aria-label={t("ruleset.effects.saveHint")}
              onClick={() => void save()}
            >
              {t("ruleset.effects.save")}
            </Button>
          </div>
        )}
      </div>
    </section>
  );
}
