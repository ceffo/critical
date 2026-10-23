import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { primaryEntry } from "@/lib/classEntries";
import { useClassName } from "@/lib/useClassName";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { useCharacterStore } from "@/stores/character";
import type { Character } from "@/types/character";

/**
 * The character takes a new class (ADR 0042 step 3): pick the class among the catalogue's, then a
 * confirmation that says what changes (the old class is frozen at its level, the XP from now on goes
 * to the new class). The server decides whether it is allowed: a refusal shows in the dialog in the
 * code's wording and nothing changes. The class list is the creation options the page already holds;
 * only the classes the character does not already have are offered.
 */
export function TakeClassDialog({
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
  const take = useCharacterStore((s) => s.takeNewClass);
  const { options } = useCreationOptions();
  const [chosen, setChosen] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);

  const have = new Set(character.classes.map((e) => e.class));
  const offered = (options?.classes ?? [])
    .map((c) => c.class)
    .filter((c) => !have.has(c));
  const from = primaryEntry(character.classes)?.class ?? "";
  // A single-class sheet has no `derived.classes`: its level is the realized one.
  const level =
    character.derived.classes?.find((e) => e.class === from)?.level ??
    character.derived.realizedLevel;

  const close = (next: boolean) => {
    if (busy) return;
    if (!next) {
      setChosen("");
      setConfirming(false);
      setRefusal(null);
    }
    onOpenChange(next);
  };

  const confirm = async () => {
    setBusy(true);
    const refused = await take(chosen);
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
        aria-label={t("takeClass.title")}
        data-testid="take-class-dialog"
      >
        <DialogTitle className="section-label">
          {t("takeClass.title")}
        </DialogTitle>
        {confirming ? (
          <>
            <p className="text-body" data-testid="take-class-confirm">
              {t("takeClass.confirm", {
                from: className(from),
                level,
                to: className(chosen),
              })}
            </p>
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="ghost"
                disabled={busy}
                onClick={() => setConfirming(false)}
              >
                {t("takeClass.back")}
              </Button>
              <Button
                type="button"
                disabled={busy}
                onClick={() => void confirm()}
                data-testid="take-class-confirm-button"
              >
                {t("takeClass.take", { class: className(chosen) })}
              </Button>
            </div>
          </>
        ) : (
          <>
            <div className="flex min-w-0 flex-col gap-1 text-body">
              <label
                htmlFor="take-class-pick"
                className="text-label text-muted-foreground"
              >
                {t("takeClass.pick")}
              </label>
              <select
                id="take-class-pick"
                value={chosen}
                className={fieldClass}
                data-testid="take-class-pick"
                onChange={(e) => {
                  setChosen(e.target.value);
                  setRefusal(null);
                }}
              >
                <option value="">{t("takeClass.placeholder")}</option>
                {offered.map((c) => (
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
                data-testid="take-class-refusal"
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
                data-testid="take-class-next"
              >
                {t("takeClass.next")}
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
