import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Stop } from "@phosphor-icons/react";
import { fetchNextRoundPreview } from "@/api/round";
import { stopPower } from "@/api/psionics";
import { Button } from "@/components/ui/button";
import { localized } from "@/i18n/localized";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";
import type { NextRoundPreview } from "@/types/buffs";
import type { Power } from "@/types/powers";
import { keepsUp, maintainedRows, type MaintainedRow } from "./maintained";
import { useKnownPowers } from "./useKnownPowers";

/** The card for the character's known powers, for the Act stage's right column under the result. */
export function MaintainedPanel() {
  const known = useCharacterStore((s) => s.character?.derived?.knownPowers);
  const powers = useKnownPowers(known?.picks ?? []);
  return <MaintainedCard powers={powers} />;
}

/**
 * The "Maintained" card of Act > Powers (ADR 0047 item 10): every row that keeps a power, a tangent
 * or a contact up, what it costs, Stop, and the PSPs Next Round leaves. The figures are the
 * server's preview, read again whenever the rows, the round or the PSPs change.
 */
export function MaintainedCard({ powers }: { powers: Power[] }) {
  const buffs = useBuffsStore((s) => s.buffs);
  const round = useBuffsStore((s) => s.round);
  const psp = useCharacterStore((s) => s.character?.derived?.psp?.current);
  const busy = useTableStore((s) => s.busy);
  const run = useTableStore((s) => s.run);
  const dismiss = useTableStore((s) => s.dismissBuff);
  const preview = useNextRoundPreview(buffs.some(keepsUp), [buffs, round, psp]);
  const rows = maintainedRows(buffs, preview?.rows ?? []);
  if (rows.length === 0) return null;
  return (
    <MaintainedCardView
      rows={rows}
      powers={powers}
      preview={preview}
      busy={busy}
      onStop={(row) =>
        row.powerId
          ? void run(() => stopPower(row.powerId as string))
          : void dismiss(row.id)
      }
    />
  );
}

/** The card for the rows and preview it is given: no store, no fetch. */
export function MaintainedCardView({
  rows,
  powers,
  preview,
  busy,
  onStop,
}: {
  rows: MaintainedRow[];
  powers: Power[];
  preview: NextRoundPreview | null;
  busy: boolean;
  onStop: (row: MaintainedRow) => void;
}) {
  const { t, i18n } = useTranslation();
  const nameOf = (r: MaintainedRow): string => {
    const p = powers.find((x) => x.id === r.powerId);
    if (p) return localized(i18n.language, p.name);
    if (r.target) {
      return t(
        r.tangent ? "powers.maintained.tangent" : "powers.maintained.contact",
        { target: r.target },
      );
    }
    return r.name;
  };
  return (
    <section
      className="group-band flex flex-col gap-1 rounded-lg px-3 py-2"
      data-testid="maintained-card"
    >
      <h3 className="section-label">{t("powers.maintained.title")}</h3>
      <ul className="flex list-none flex-col">
        {rows.map((r) => {
          const name = nameOf(r);
          return (
            <li
              key={r.id}
              className="flex flex-wrap items-center gap-3 rounded-md px-2.5 py-1 odd:bg-raised/50"
              data-testid="maintained-row"
              data-kind={r.kind}
            >
              <span className="flex min-w-40 flex-1 basis-40 flex-col">
                <span lang={i18n.language} className="text-body font-medium">
                  {name}
                </span>
                <RowNote row={r} />
              </span>
              <span
                className={cn(
                  "text-body tabular-nums",
                  r.kind === "paid" ? "text-foreground" : "text-ink-muted",
                )}
                data-testid="maintained-cost"
              >
                {r.kind === "paid"
                  ? t("lookup.powers.maintenanceValue", {
                      cost: t("lookup.powers.psp", { count: r.cost }),
                      count: r.period,
                    })
                  : t("powers.maintained.free")}
              </span>
              <Button
                className="gap-1 px-2"
                disabled={busy}
                aria-label={t("powers.maintained.stopNamed", { name })}
                onClick={() => onStop(r)}
              >
                <Stop aria-hidden />
                {t("powers.use.stop")}
              </Button>
            </li>
          );
        })}
      </ul>
      {preview && (
        <p
          className="flex flex-wrap items-baseline gap-2 border-t pt-1.5 text-body"
          data-testid="maintained-balance"
        >
          <span className="text-ink-muted">
            {t("powers.maintained.balance")}
          </span>
          <span className="font-medium tabular-nums">
            {t("powers.maintained.balanceValue", {
              before: preview.pspBefore,
              after: preview.pspAfter,
            })}
          </span>
        </p>
      )}
    </section>
  );
}

function RowNote({ row }: { row: MaintainedRow }) {
  const { t } = useTranslation();
  const note = row.ends
    ? t("powers.maintained.ends")
    : row.kind === "standing"
      ? t("powers.maintained.standing")
      : row.kind === "contact-free"
        ? t("powers.maintained.freeWhileUsed")
        : null;
  if (!note) return null;
  return (
    <span
      className={cn("text-label", row.ends ? "text-warning" : "text-ink-muted")}
    >
      {note}
    </span>
  );
}

/**
 * The server's preview of the next Next Round, read again when `deps` change. It is null until the
 * read for the current `deps` arrives and after a failed one: a figure of the old rows never shows.
 */
function useNextRoundPreview(
  on: boolean,
  deps: unknown[],
): NextRoundPreview | null {
  const [read, setRead] = useState<{
    deps: unknown[];
    preview: NextRoundPreview | null;
  } | null>(null);
  useEffect(() => {
    if (!on) return;
    const ctl = new AbortController();
    const done = (preview: NextRoundPreview | null) =>
      !ctl.signal.aborted && setRead({ deps, preview });
    fetchNextRoundPreview({ signal: ctl.signal }).then(done, () => done(null));
    return () => ctl.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [on, ...deps]);
  const current =
    read !== null &&
    read.deps.length === deps.length &&
    read.deps.every((d, i) => Object.is(d, deps[i]));
  return on && current ? read.preview : null;
}
