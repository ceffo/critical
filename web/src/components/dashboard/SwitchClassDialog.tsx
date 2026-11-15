import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Warning } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { fetchSwitchTargets, type SwitchTarget } from "@/api/character";
import { useClassName } from "@/lib/useClassName";
import { useCharacterStore } from "@/stores/character";
import { cn } from "@/lib/utils";
import type { Character } from "@/types/character";

/**
 * The school picker (ADR 0050 step 15.7): a class entry switched to another class of its group, a
 * mage becoming a specialist, a specialist a mage or another specialist. Eight-odd cards, one per
 * class the entry could become, each with the school, the schools it opposes, its ability minimum
 * against the sheet's score and the race warning; a card the server would refuse is greyed with its
 * reason and cannot be picked. Then a confirmation that names what is kept, what the opposed spells
 * do and the race warning (shown, never a refusal). Every figure and verdict is the server's
 * (`GET /api/character/class-entry/targets`): the web words them, it compares nothing.
 */
export function SwitchClassDialog({
  character,
  entry,
  open,
  onOpenChange,
}: {
  character: Character;
  /** The class entry (its index in `classes`) the picker switches. */
  entry: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const className = useClassName();
  const doSwitch = useCharacterStore((s) => s.switchEntryClass);
  const [targets, setTargets] = useState<SwitchTarget[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [chosen, setChosen] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const from = character.classes[entry]?.class ?? "";

  // The picker is read when it opens: the scores and the entries are the sheet's of this moment.
  useEffect(() => {
    if (!open) return;
    let live = true;
    fetchSwitchTargets(entry).then(
      (r) => live && setTargets(r.targets),
      () => live && setFailed(true),
    );
    return () => {
      live = false;
    };
  }, [open, entry, from]);

  const target = targets?.find((x) => x.class === chosen);
  const specialist = (target?.schools.length ?? 0) > 0;
  const race = t(`details.races.${character.raceCode}`, {
    defaultValue: character.raceCode,
  });

  const close = (next: boolean) => {
    if (busy) return;
    if (!next) {
      setTargets(null);
      setFailed(false);
      setChosen("");
      setConfirming(false);
      setRefusal(null);
    }
    onOpenChange(next);
  };

  const confirm = async () => {
    setBusy(true);
    const refused = await doSwitch(entry, chosen);
    setBusy(false);
    if (refused) {
      setRefusal(refused.message);
      setConfirming(false);
      return;
    }
    close(false);
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent
        className={cn("gap-3", confirming ? "max-w-md" : "max-w-4xl")}
        aria-label={t("switchClass.title")}
        data-testid="switch-class-dialog"
      >
        <DialogTitle className="section-label">
          {t("switchClass.title")}
        </DialogTitle>
        {confirming && target ? (
          <>
            <div
              className="flex flex-col gap-2 text-body"
              data-testid="switch-class-confirm"
            >
              <p>
                {t("switchClass.confirm", {
                  from: className(from),
                  to: className(chosen),
                })}
              </p>
              {specialist && (
                <p data-testid="switch-class-opposed">
                  {t("switchClass.opposed", { to: className(chosen) })}
                </p>
              )}
              {target.raceWarning && (
                <p
                  className="flex items-start gap-1 text-warning"
                  data-testid="switch-class-race-warning"
                >
                  <Warning aria-hidden className="mt-0.5 size-3.5 shrink-0" />
                  <span className="min-w-0 break-words">
                    {t("switchClass.race", {
                      race,
                      to: className(chosen),
                    })}
                  </span>
                </p>
              )}
              <p className="text-muted-foreground">{t("switchClass.undo")}</p>
            </div>
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="ghost"
                disabled={busy}
                onClick={() => setConfirming(false)}
              >
                {t("switchClass.back")}
              </Button>
              <Button
                type="button"
                disabled={busy}
                onClick={() => void confirm()}
                data-testid="switch-class-confirm-button"
              >
                {t("switchClass.switch", { class: className(chosen) })}
              </Button>
            </div>
          </>
        ) : (
          <>
            <p className="text-body text-muted-foreground">
              {t("switchClass.intro", { class: className(from) })}
            </p>
            {failed && (
              <span
                role="alert"
                className="text-label text-danger"
                data-testid="switch-class-load-error"
              >
                {t("switchClass.loadError")}
              </span>
            )}
            <div
              role="radiogroup"
              aria-label={t("switchClass.title")}
              className="grid min-h-0 grid-cols-1 gap-2 overflow-y-auto sm:grid-cols-2"
              data-testid="switch-class-cards"
            >
              {targets?.map((x) => (
                <SchoolCard
                  key={x.class}
                  target={x}
                  on={x.class === chosen}
                  race={race}
                  onPick={() => {
                    setChosen(x.class);
                    setRefusal(null);
                  }}
                />
              ))}
            </div>
            {refusal && (
              <span
                role="alert"
                className="text-label text-danger"
                data-testid="switch-class-refusal"
              >
                {refusal}
              </span>
            )}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="ghost"
                onClick={() => close(false)}
              >
                {t("details.cancel")}
              </Button>
              <Button
                type="button"
                disabled={target === undefined}
                onClick={() => setConfirming(true)}
                data-testid="switch-class-next"
              >
                {t("switchClass.next")}
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** One class the entry could become: its school, opposed schools, minimums and race note. */
function SchoolCard({
  target: x,
  on,
  race,
  onPick,
}: {
  target: SwitchTarget;
  on: boolean;
  race: string;
  onPick: () => void;
}) {
  const { t } = useTranslation();
  const className = useClassName();
  const school = (code: string) =>
    t(`lookup.schoolNames.${code}`, { defaultValue: code });
  const refused = x.refusal !== "";
  const unmet = x.minimums.filter((m) => !m.met);
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      aria-disabled={refused}
      data-testid={`school-card-${x.class}`}
      onClick={() => {
        if (!refused) onPick();
      }}
      className={cn(
        "flex min-w-0 flex-col gap-0.5 rounded-lg border px-3 py-2 text-left text-body",
        refused
          ? "cursor-not-allowed border-dashed text-ink-muted"
          : "cursor-pointer hover:bg-raised-hover",
        on
          ? "selected border-transparent shadow-[inset_0_0_0_1.5px_var(--tint)]"
          : "border-input",
      )}
    >
      <span className="flex items-baseline justify-between gap-2">
        <span className="min-w-0 break-words text-key">
          {x.schools.length > 0
            ? school(x.schools[0] ?? "")
            : t("switchClass.noSchool")}
        </span>
        <span className="shrink-0 text-label text-ink-muted">
          {className(x.class)}
        </span>
      </span>
      {x.opposedSchools.length > 0 && (
        <span className="text-label">
          <span className="text-ink-muted">
            {t("switchClass.opposedLabel")}
          </span>
          {x.opposedSchools.map(school).join(", ")}
        </span>
      )}
      <span className="flex flex-wrap items-center gap-x-3 text-label">
        {x.minimums.map((m) => (
          <span
            key={m.ability}
            className={cn(
              "inline-flex items-center gap-1 tabular-nums",
              !m.met && "font-semibold text-danger",
            )}
          >
            {m.met ? (
              <Check aria-hidden className="size-3.5 text-success" />
            ) : (
              <Warning aria-hidden className="size-3.5" />
            )}
            {t(`attr.${m.ability}`)} {m.min}
            {!m.met && t("switchClass.needed", { score: m.score })}
          </span>
        ))}
        {unmet.length === 0 && x.refusal !== "" && (
          <span className="inline-flex items-center gap-1 text-danger">
            <Warning aria-hidden className="size-3.5" />
            {t(`switchClass.refusals.${x.refusal}`, {
              defaultValue: x.refusal,
            })}
          </span>
        )}
        {!refused && x.raceWarning && (
          <span className="inline-flex items-center gap-1 text-warning">
            <Warning aria-hidden className="size-3.5" />
            {t("switchClass.raceShort", { race })}
          </span>
        )}
      </span>
    </button>
  );
}
