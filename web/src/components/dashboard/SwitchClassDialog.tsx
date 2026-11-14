import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Warning } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { useClassName } from "@/lib/useClassName";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { useCharacterStore } from "@/stores/character";
import { switchableEntries, switchTargets } from "@/lib/classSwitch";
import type { Character } from "@/types/character";

/**
 * A class entry switched to another class of its group (ADR 0050 step 15): a mage becomes a
 * specialist, a specialist a mage or another specialist. Pick the entry (only when several can
 * switch) and the class, then a confirmation that names what is kept, what the opposed spells do,
 * and the race warning when the race is not among Table 22's for the class: shown, never a refusal.
 * The server decides whether the switch is allowed and a refusal shows in its wording.
 */
export function SwitchClassDialog({
  character,
  open,
  onOpenChange,
}: {
  character: Character;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const className = useClassName();
  const doSwitch = useCharacterStore((s) => s.switchEntryClass);
  const { options } = useCreationOptions();
  const entries = switchableEntries(character, options);
  const [picked, setPicked] = useState<number | null>(null);
  const [chosen, setChosen] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);

  const entry =
    picked !== null && entries.includes(picked) ? picked : entries[0];
  const from = entry === undefined ? "" : character.classes[entry]?.class;
  const targets =
    entry === undefined ? [] : switchTargets(character, options, entry);
  const specialist = options?.specialistRaces?.[chosen];
  // PHB Table 22's races for the class; none listed for a mage, which is no specialist.
  const raceWarning =
    specialist !== undefined && !specialist.includes(character.raceCode);
  const race = t(`details.races.${character.raceCode}`, {
    defaultValue: character.raceCode,
  });

  const close = (next: boolean) => {
    if (busy) return;
    if (!next) {
      setPicked(null);
      setChosen("");
      setConfirming(false);
      setRefusal(null);
    }
    onOpenChange(next);
  };

  const confirm = async () => {
    if (entry === undefined) return;
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
        className="max-w-md gap-3"
        aria-label={t("switchClass.title")}
        data-testid="switch-class-dialog"
      >
        <DialogTitle className="section-label">
          {t("switchClass.title")}
        </DialogTitle>
        {confirming ? (
          <>
            <div
              className="flex flex-col gap-2 text-body"
              data-testid="switch-class-confirm"
            >
              <p>
                {t("switchClass.confirm", {
                  from: className(from ?? ""),
                  to: className(chosen),
                })}
              </p>
              {specialist !== undefined && (
                <p data-testid="switch-class-opposed">
                  {t("switchClass.opposed", { to: className(chosen) })}
                </p>
              )}
              {raceWarning && (
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
            {entries.length > 1 && (
              <div className="flex min-w-0 flex-col gap-1 text-body">
                <label
                  htmlFor="switch-class-entry"
                  className="text-label text-muted-foreground"
                >
                  {t("switchClass.entry")}
                </label>
                <select
                  id="switch-class-entry"
                  value={entry}
                  className={fieldClass}
                  data-testid="switch-class-entry"
                  onChange={(e) => {
                    setPicked(Number(e.target.value));
                    setChosen("");
                    setRefusal(null);
                  }}
                >
                  {entries.map((i) => (
                    <option key={i} value={i}>
                      {className(character.classes[i]?.class ?? "")}
                    </option>
                  ))}
                </select>
              </div>
            )}
            <div className="flex min-w-0 flex-col gap-1 text-body">
              <label
                htmlFor="switch-class-pick"
                className="text-label text-muted-foreground"
              >
                {t("switchClass.pick")}
              </label>
              <select
                id="switch-class-pick"
                value={chosen}
                className={fieldClass}
                data-testid="switch-class-pick"
                onChange={(e) => {
                  setChosen(e.target.value);
                  setRefusal(null);
                }}
              >
                <option value="">{t("switchClass.placeholder")}</option>
                {targets.map((c) => (
                  <option key={c} value={c}>
                    {className(c)}
                  </option>
                ))}
              </select>
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
                disabled={chosen === ""}
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
