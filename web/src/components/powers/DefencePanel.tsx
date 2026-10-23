import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { ApiError } from "@/api/http";
import { defend } from "@/api/psionics";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import { DefendForm } from "./DefendForm";
import { currentDefence } from "./defend";

/**
 * The character's current defence (ADR 0044 step 5) on the Powers panel: the mode raised this
 * round, else the standing one, else none; and Defend, which opens the form the DM's rolls are
 * entered in (step 8). Reads the buff rows the panel already shares; nothing is fetched here.
 */
export function DefencePanel({
  powerName,
}: {
  powerName: (id: string) => string;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const busy = useTableStore((s) => s.busy);
  const row = useBuffsStore((s) => currentDefence(s.buffs));

  async function onDefend(req: Parameters<typeof defend>[0]) {
    let result = null as Awaited<ReturnType<typeof defend>>["defence"] | null;
    let refused = "";
    await useTableStore.getState().submit(async () => {
      try {
        result = (await defend(req)).defence;
      } catch (e) {
        if (e instanceof ApiError && e.status < 500) {
          refused = e.message;
          return;
        }
        throw e;
      }
    });
    return result ? { result } : { refused };
  }

  return (
    <div
      className="flex flex-wrap items-center gap-2 rounded-lg bg-raised px-3 py-2"
      data-testid="current-defence"
    >
      <span className="flex min-w-40 flex-1 flex-col">
        <span className="text-label text-muted-foreground">
          {t("powers.defence.current")}
        </span>
        <span className="text-body" data-defence={row?.defence ?? "none"}>
          {row?.powerId
            ? `${powerName(row.powerId)} · ${t(`powers.defence.kind.${row.defence}`)}`
            : t("powers.defence.none")}
        </span>
      </span>
      <Button className="px-2" disabled={busy} onClick={() => setOpen(true)}>
        {t("powers.defence.defend")}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-md">
          <DialogTitle className="section-label mb-3">
            {t("powers.defence.title")}
          </DialogTitle>
          <DefendForm busy={busy} powerName={powerName} onDefend={onDefend} />
        </DialogContent>
      </Dialog>
    </div>
  );
}
