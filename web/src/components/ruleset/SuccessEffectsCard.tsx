import { useTranslation } from "react-i18next";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import type { EffectDraft } from "./rulesetDraft";
import { UnsavedMark } from "./UnsavedMark";

/** Wide enough for the largest value: a two-digit round count or a signed AC. */
const cellClass = `${fieldClass} !w-16`;

/**
 * The buff a successful check of a proficiency puts on the character (ADR 0041 step 2.4): the AC
 * change (descending AC, negative betters it) and the rounds it lasts. The proficiency is worded
 * from its id; its values are edited here, sent with the page's Save, and the server validates them.
 * A player reads them (`editable` false): read-only fields.
 */
export function SuccessEffectsCard({
  rows,
  onChange,
  error,
  dirty,
  editable = true,
}: {
  rows: EffectDraft[];
  onChange: (rows: EffectDraft[]) => void;
  error: string;
  dirty: boolean;
  editable?: boolean;
}) {
  const { t } = useTranslation();

  function edit(i: number, field: "ac" | "rounds", value: string) {
    onChange(rows.map((r, j) => (j === i ? { ...r, [field]: value } : r)));
  }

  const nameOf = (id: string) => t(`skills.names.${id}`, { defaultValue: id });

  return (
    <section
      aria-labelledby="ruleset-effects"
      className="zone-card flex min-w-0 flex-col"
      data-testid="ruleset-success-effects"
    >
      <div className="card-band flex items-center gap-2 px-4 py-2">
        <h2 id="ruleset-effects" className="section-label">
          {t("ruleset.effects.title")}
        </h2>
        <UnsavedMark show={dirty} />
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
          <div className="grid grid-cols-[minmax(0,1fr)_4rem_4rem] items-end gap-2">
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
                    className={cellClass}
                    type="number"
                    inputMode="numeric"
                    aria-label={t(`ruleset.effects.${f}`)}
                    value={r[f]}
                    readOnly={!editable}
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
      </div>
    </section>
  );
}
