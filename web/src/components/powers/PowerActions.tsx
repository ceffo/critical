import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { ApiError } from "@/api/http";
import { stopPower, invokePower, raisePower } from "@/api/psionics";
import { localized } from "@/i18n/localized";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import type { Power } from "@/types/powers";
import { UsePowerForm } from "./UsePowerForm";

/**
 * The actions of one known power in the Powers panel (ADR 0043 steps 10 to 16): Use, which opens the
 * form over the power, and, while a buff row keeps it up, Stop (free, undone by the journal's Undo).
 * Every kind of power gets Use (the server checks and refuses). The row it sits in is the panel's.
 */
export function PowerActions({ power }: { power: Power }) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const busy = useTableStore((s) => s.busy);
  const run = useTableStore((s) => s.run);
  // The row up for this power: a defence reads by its kind (raised this round, standing), any other
  // as maintained.
  const row = useBuffsStore((s) => s.buffs.find((b) => b.powerId === power.id));
  const maintained = !!row;
  const name = localized(i18n.language, power.name);
  const defence = power.kind === "defence_mode";
  const [refusal, setRefusal] = useState<string | null>(null);

  // A defence mode is raised, not used (ADR 0044 step 4): no roll, the cost paid. A refusal (out of
  // combat, too few PSPs) is worded under the row, from the server's code.
  async function raise() {
    setRefusal(null);
    await useTableStore.getState().submit(async () => {
      try {
        await raisePower(power.id);
      } catch (e) {
        if (e instanceof ApiError && e.status < 500) {
          setRefusal(e.message);
          return;
        }
        throw e;
      }
    });
  }

  async function use(req: Parameters<typeof invokePower>[1]) {
    let check = null as Awaited<ReturnType<typeof invokePower>>["check"] | null;
    let refused: string | null = null;
    // submit keeps the busy flag and reads again what the use changed (the pool, the journal, the
    // buffs); a refusal (4xx with a code) is the form's to word, nothing else goes to the notice.
    await useTableStore.getState().submit(async () => {
      try {
        check = (await invokePower(power.id, req)).check;
      } catch (e) {
        if (e instanceof ApiError && e.status < 500) {
          refused = e.message;
          return;
        }
        throw e;
      }
    });
    return check ? { check } : { refused: refused ?? "" };
  }

  return (
    <>
      {maintained && (
        <>
          <span
            className="shrink-0 text-label whitespace-nowrap text-muted-foreground"
            data-testid="power-maintained"
          >
            {row?.defence
              ? t(`powers.defence.kind.${row.defence}`)
              : t("powers.use.maintained")}
          </span>
          <Button
            className="px-2"
            disabled={busy}
            aria-label={t("powers.use.stopNamed", { name })}
            onClick={() => void run(() => stopPower(power.id))}
          >
            {t("powers.use.stop")}
          </Button>
        </>
      )}
      {defence ? (
        <Button
          className="px-2"
          disabled={busy}
          aria-label={t("powers.defence.raiseNamed", { name })}
          onClick={() => void raise()}
        >
          {t("powers.defence.raise")}
        </Button>
      ) : (
        <Button
          className="px-2"
          disabled={busy}
          aria-label={t("powers.use.useNamed", { name })}
          onClick={() => setOpen(true)}
        >
          {t("powers.use.use")}
        </Button>
      )}
      {refusal && (
        <p
          role="alert"
          className="basis-full text-label text-danger"
          data-testid="raise-refused"
        >
          {refusal}
        </p>
      )}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-md">
          <DialogTitle className="section-label mb-3">
            {t("powers.use.title", { name })}
          </DialogTitle>
          <UsePowerForm
            power={power}
            powerName={name}
            busy={busy}
            onUse={use}
          />
        </DialogContent>
      </Dialog>
    </>
  );
}
