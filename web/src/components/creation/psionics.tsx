import { useTranslation } from "react-i18next";
import { labelClass, fieldClass } from "@/components/dashboard/fieldStyles";
import { KnownPowersEditor } from "@/components/powers/KnownPowersPanel";
import { ApiError } from "@/api/http";
import { cn } from "@/lib/utils";
import type { DraftRefusal } from "@/types/character";
import { Group, Note } from "./kit";
import type { StepProps } from "./flow";

/** A draft's known-powers refusal as the editor shows a save's 409: its code, and its figures in the
 * body (the power, the discipline, the slot, the count against the limit). */
function asApiError(r: DraftRefusal): ApiError {
  const body: Record<string, unknown> = {};
  for (const k of ["power", "discipline", "slot", "count", "limit"] as const) {
    if (r[k] !== undefined && r[k] !== "") body[k] = r[k];
  }
  return new ApiError(r.code, 409, r.field, r.code, body);
}

/**
 * The creation's psionics step (ADR 0043 steps 4.4 and 9), for a class listing psionics: the PSP
 * maximum, the server's computed one at the starting level unless the player types his sheet's, and
 * the known powers in the dashboard's editor. The draft (`POST /api/characters/draft`) answers the
 * allowance at the starting level and refuses a list as the editor's save does; Créer sends both.
 * Never blocks Suivant: an empty list is a psionicist who picks later.
 */
export function PsionicsStep(props: StepProps) {
  const { draft, update, errorFor, answer } = props;
  const { t } = useTranslation();
  const computed = answer.latest?.computedPspMax ?? null;
  const known = answer.latest?.derived?.knownPowers ?? null;
  const pspError = errorFor("pspMax");
  const refused = answer.current?.refusals.find(
    (r) => r.field === "psionicPicks",
  );
  // A Créer refusal the draft did not show (a list it answered before the last change).
  const picksError = refused ? null : errorFor("psionicPicks");

  return (
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col gap-5"
      data-testid="psionics-step"
    >
      <Group title={t("creation.psionics.psp")}>
        <div className="flex flex-wrap items-end gap-4 px-3">
          <div className="flex w-40 min-w-0 flex-col gap-1">
            <label htmlFor="new-character-pspMax" className={labelClass}>
              {t("creation.psionics.pspMax")}
            </label>
            <input
              id="new-character-pspMax"
              aria-invalid={pspError ? true : undefined}
              aria-describedby={
                pspError ? "new-character-pspMax-error" : undefined
              }
              className={cn(fieldClass, pspError && "border-destructive")}
              inputMode="numeric"
              autoComplete="off"
              placeholder={computed !== null ? String(computed) : ""}
              value={draft.pspMax}
              onChange={(e) => update({ pspMax: e.target.value })}
            />
          </div>
          <p
            className="pb-1.5 text-body break-words text-ink-muted"
            data-testid="psp-computed"
          >
            {computed !== null
              ? t("creation.psionics.computed", { psp: computed })
              : t("creation.psionics.wait")}
          </p>
        </div>
        {pspError && (
          <p
            id="new-character-pspMax-error"
            role="alert"
            className="px-3 text-label text-destructive"
            data-testid="field-error"
          >
            {pspError}
          </p>
        )}
        <p className="px-3 text-label break-words text-ink-muted">
          {t("creation.psionics.pspEditable")}
        </p>
      </Group>
      {picksError && <Note kind="refusal">{picksError}</Note>}
      {known ? (
        // The editor scrolls its own lists: it gets the step's height, at least enough to pick in.
        <div className="flex min-h-[30rem] flex-1 flex-col">
          <KnownPowersEditor
            known={{ ...known, picks: draft.psionicPicks }}
            busy={false}
            refusal={refused ? asApiError(refused) : null}
            onChange={(psionicPicks) => update({ psionicPicks })}
          />
        </div>
      ) : (
        <Note kind="info" testId="psionics-wait">
          {t("creation.psionics.wait")}
        </Note>
      )}
    </div>
  );
}
