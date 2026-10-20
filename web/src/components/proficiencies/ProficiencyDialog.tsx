import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { X } from "@phosphor-icons/react";
import { getProficiencyDescription } from "@/api/proficiencies";
import { SpellDescription } from "@/components/dashboard/SpellDescription";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { localized } from "@/i18n/localized";
import { skillName } from "@/journal/words";
import { formatSigned } from "@/lib/format";
import { useCreationOptionsStore } from "@/stores/creationOptions";
import type { LocalizedText } from "@/types/localized";

/**
 * What the pop-up shows besides the text, all the server's figures (the web computes nothing): the
 * ability and modifier of the check, the slots spent and the extra slots of a proficiency the
 * character has, its cost for the class where the picker knows it. A figure a caller does not have
 * is left out.
 */
export interface ProficiencyFacts {
  id: string;
  /** The check's ability; "" or absent for a proficiency with no check. */
  ability?: string;
  modifier?: number;
  slots?: number;
  extraSlots?: number;
  cost?: number;
}

/** What the description lookup gave for one id. */
type Load =
  { id: string; description: LocalizedText } | { id: string; failed: true };

/**
 * The pop-up of a non-weapon proficiency (ADR 0037 step 19.4), shared by the Skills tab and the
 * picker: its name, the description in the UI language (English where there is none), then the
 * figures. The description is read when the pop-up opens (the API module keeps it); Escape, the
 * close button or a click outside closes it.
 */
export function ProficiencyDialog({
  facts,
  onClose,
}: {
  /** The proficiency shown; null: closed. */
  facts: ProficiencyFacts | null;
  onClose: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [load, setLoad] = useState<Load | null>(null);
  const id = facts?.id;
  useEffect(() => {
    if (id === undefined) return;
    let live = true;
    getProficiencyDescription(id).then(
      (found) => live && setLoad({ id, description: found.description }),
      () => live && setLoad({ id, failed: true }),
    );
    return () => {
      live = false;
    };
  }, [id]);
  // The groups are the catalogue's, read with the creation options (one read per page, shared with
  // the pick editors).
  const groupsOf = useCreationOptionsStore((s) =>
    id === undefined
      ? undefined
      : s.options?.proficiencies?.find((p) => p.id === id)?.groups,
  );
  useEffect(() => {
    if (id !== undefined) void useCreationOptionsStore.getState().load();
  }, [id]);
  const shown = load && load.id === id ? load : null;

  return (
    <Dialog open={facts !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        className="max-w-[44rem] gap-3 overflow-y-auto"
        data-testid="proficiency-details"
      >
        <div className="flex items-start gap-2">
          <DialogTitle className="min-w-0 text-title font-semibold break-words">
            {id === undefined ? "" : skillName(t, id)}
          </DialogTitle>
          <DialogClose
            render={
              <Button
                size="icon-sm"
                variant="ghost"
                className="ml-auto shrink-0"
                aria-label={t("proficiencies.details.close")}
                title={t("proficiencies.details.close")}
              />
            }
          >
            <X aria-hidden />
          </DialogClose>
        </div>
        {facts && (
          <ProficiencyDetail
            facts={facts}
            groups={groupsOf}
            description={
              shown && "description" in shown
                ? localized(i18n.language, shown.description)
                : undefined
            }
            failed={shown !== null && "failed" in shown}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

/**
 * The pop-up's body: the description rendered as a spell description is (paragraphs, lists,
 * tables), then the figures as a term list. `description` undefined while it loads.
 */
export function ProficiencyDetail({
  facts,
  groups,
  description,
  failed = false,
}: {
  facts: ProficiencyFacts;
  groups?: string[];
  description: string | undefined;
  failed?: boolean;
}) {
  const { t } = useTranslation();
  const rows: { key: string; label: string; value: string }[] = [];
  if (facts.ability !== undefined)
    rows.push({
      key: "ability",
      label: t("proficiencies.details.ability"),
      value: facts.ability
        ? t(`attr.${facts.ability}`, { defaultValue: facts.ability })
        : t("proficiencies.noCheck"),
    });
  if (facts.ability && facts.modifier !== undefined)
    rows.push({
      key: "modifier",
      label: t("proficiencies.details.modifier"),
      value: formatSigned(facts.modifier),
    });
  if (facts.cost !== undefined)
    rows.push({
      key: "cost",
      label: t("proficiencies.details.cost"),
      value: String(facts.cost),
    });
  if (facts.slots !== undefined)
    rows.push({
      key: "slots",
      label: t("proficiencies.details.slots"),
      value: String(facts.slots),
    });
  if (facts.extraSlots !== undefined)
    rows.push({
      key: "extra",
      label: t("proficiencies.details.extra"),
      value: String(facts.extraSlots),
    });
  if (groups && groups.length > 0)
    rows.push({
      key: "groups",
      label: t("proficiencies.details.groups"),
      value: groups
        .map((g) => t(`proficiencies.groups.${g}`, { defaultValue: g }))
        .join(", "),
    });
  return (
    <div
      className="flex min-w-0 flex-col gap-4"
      data-testid="proficiency-detail"
    >
      {failed ? (
        <p className="text-body text-danger" role="alert">
          {t("proficiencies.details.failed")}
        </p>
      ) : description === undefined ? (
        <p className="text-body text-muted-foreground">{t("app.loading")}</p>
      ) : description === "" ? (
        <p className="text-body text-muted-foreground">
          {t("proficiencies.details.empty")}
        </p>
      ) : (
        <SpellDescription
          text={description}
          className="text-body"
          testId="proficiency-description"
        />
      )}
      {rows.length > 0 && (
        <dl
          className="grid grid-cols-[repeat(auto-fit,minmax(9rem,1fr))] gap-2"
          data-testid="proficiency-figures"
        >
          {rows.map((r) => (
            <div
              key={r.key}
              className="flex min-w-0 flex-col rounded-md bg-raised px-2.5 py-1"
              data-testid={`proficiency-figure-${r.key}`}
            >
              <dt className="section-label">{r.label}</dt>
              <dd className="text-body break-words tabular-nums">{r.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  );
}

/**
 * A proficiency's name as the button that opens its pop-up: keyboard-focusable, its own target
 * beside the row's other actions. It owns the pop-up's open state.
 */
export function ProficiencyNameButton({
  facts,
  className,
  children,
}: {
  facts: ProficiencyFacts;
  className?: string;
  children?: React.ReactNode;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const name = skillName(t, facts.id);
  const label = t("proficiencies.details.open", { name });
  return (
    <>
      <button
        type="button"
        className={className}
        aria-label={label}
        title={label}
        onClick={() => setOpen(true)}
        data-testid={`proficiency-name-${facts.id}`}
      >
        {children ?? <span className="break-words">{name}</span>}
      </button>
      {open && (
        <ProficiencyDialog facts={facts} onClose={() => setOpen(false)} />
      )}
    </>
  );
}
