import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Moon } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { useBuffsStore } from "@/stores/buffs";

/**
 * The Spells tab's Long rest. Out of combat (round 0) one click rests; with a fight running (the
 * round counter above 0) the click opens a confirmation that says what is refilled, and nothing
 * changes until it is confirmed. Cancel and Escape leave the slots as they are, and focus goes
 * back to this button.
 */
export function LongRestButton({
  busy,
  onRest,
}: {
  busy: boolean;
  onRest: () => void;
}) {
  const { t } = useTranslation();
  const round = useBuffsStore((s) => s.round);
  const [asking, setAsking] = useState(false);

  return (
    <>
      <Button
        size="sm"
        title={t("spellSlots.longRestLabel")}
        aria-label={t("spellSlots.longRestLabel")}
        onClick={() => (round > 0 ? setAsking(true) : onRest())}
        disabled={busy}
        data-testid="long-rest"
      >
        <Moon aria-hidden />
        {t("spellSlots.longRest")}
      </Button>
      <Dialog open={asking} onOpenChange={setAsking}>
        <DialogContent
          className="max-w-[28rem] gap-3"
          data-testid="long-rest-confirm"
        >
          <DialogTitle className="section-label">
            {t("spellSlots.restConfirm.title")}
          </DialogTitle>
          <p className="text-body [overflow-wrap:anywhere]">
            {t("spellSlots.restConfirm.lead", { round })}
          </p>
          <p className="text-body [overflow-wrap:anywhere] text-muted-foreground">
            {t("spellSlots.restConfirm.refills")}
          </p>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <DialogClose render={<Button variant="quiet" />}>
              {t("spellSlots.restConfirm.cancel")}
            </DialogClose>
            <Button
              variant="main"
              disabled={busy}
              data-testid="long-rest-go"
              onClick={() => {
                setAsking(false);
                onRest();
              }}
            >
              <Moon aria-hidden />
              {t("spellSlots.restConfirm.confirm")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
