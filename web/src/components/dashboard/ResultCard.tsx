import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ArrowUp } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useJournalStore } from "@/stores/journal";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { cn } from "@/lib/utils";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import { cardContent, cardSource } from "@/journal/card";
import { InfoBody } from "@/journal/parts";

/**
 * The result panel of the Act activity (ADR 0022 step 2.5): the latest result by default, or the
 * journal entry the player selected (read-only, accent ring, "Latest" returns). The body is the
 * entry kind's (the journal registry), from the entry's payload or this session's roll response;
 * corrections only on the latest result.
 */
export function ResultCard() {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const last = useRollStore((s) => s.last);
  const entries = useLogStore((s) => s.entries);
  const selectedId = useJournalStore((s) => s.selectedId);
  const showLatest = useJournalStore((s) => s.showLatest);
  const source = cardSource(last, entries, selectedId);

  let body: ReactNode = (
    <p className="py-8 text-center text-body text-muted-foreground">
      {t("result.empty")}
    </p>
  );
  let header: ReactNode = null;
  const readOnly = source.kind === "entry" && !source.latest;
  if (source.kind !== "empty") {
    const c = cardContent(source, t, i18n.language, spellName, names);
    header = (
      <div
        className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1"
        data-testid="card-header"
      >
        <span className="chip shrink-0 px-1.5 py-0.5 text-label font-medium">
          {c.tag}
        </span>
        <span
          className="min-w-0 flex-1 basis-24 break-words text-body font-medium"
          title={c.subtitle}
        >
          {c.subtitle}
        </span>
        {c.entry?.status === "voided" && (
          <span className="shrink-0 text-label text-danger">
            {t("log.status.voided")}
          </span>
        )}
        {c.when && (
          <span
            className="shrink-0 text-label text-muted-foreground tabular-nums"
            data-testid="card-when"
          >
            {c.when}
          </span>
        )}
        {readOnly && (
          <Button size="xs" className="ml-auto shrink-0" onClick={showLatest}>
            {t("journal.latest")}
            <ArrowUp aria-hidden />
          </Button>
        )}
      </div>
    );
    body =
      c.view?.card && c.payload !== null ? (
        c.view.card({
          payload: c.payload,
          entry: c.entry,
          text: c.text,
          interactive: !readOnly,
        })
      ) : (
        <InfoBody text={c.text} />
      );
  }

  return (
    <section
      className={cn(
        // The "latest" mark (ADR 0026 step 5.2) while the card shows the latest result; an older
        // entry picked in the journal shows on the plain card.
        "zone-card flex min-h-0 min-w-0 flex-col",
        !readOnly && "latest",
      )}
      aria-label={t("journal.card")}
      data-readonly={readOnly}
    >
      {/* The header band (ADR 0026 step 6.2): kind, name and time; it replaces the top padding. */}
      <div className="card-band flex min-h-10 shrink-0 items-center gap-2 px-4 py-2">
        {header ?? <h2 className="section-label">{t("journal.card")}</h2>}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 pt-3 pb-3.5">
        {body}
      </div>
    </section>
  );
}
