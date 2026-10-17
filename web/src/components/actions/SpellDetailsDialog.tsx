import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { SpellDetail } from "@/components/dashboard/SpellDetail";
import { findSpell } from "@/api/spells_db";
import type { Spell } from "@/types/spells";

interface Props {
  /** The spell shown; null: the popup is closed. */
  spell: { id: string; name: string; level: number } | null;
  onClose: () => void;
}

/** What the catalogue answered for one spell id. */
type Load = { id: string; spell: Spell | null } | { id: string; error: string };

/**
 * The Spells tab's details popup (ADR 0027 step 10.4): the lookup's detail view of a prepared spell,
 * fetched from the catalogue at the character's level; Escape, the close button or a click outside
 * closes it.
 */
export function SpellDetailsDialog({ spell, onClose }: Props) {
  const { t } = useTranslation();
  const [load, setLoad] = useState<Load | null>(null);
  const id = spell?.id;
  const name = spell?.name;
  const level = spell?.level;

  useEffect(() => {
    if (id === undefined || name === undefined || level === undefined) return;
    let live = true;
    findSpell(id, name, level).then(
      (found) => live && setLoad({ id, spell: found }),
      (err: unknown) =>
        live &&
        setLoad({
          id,
          error: err instanceof Error ? err.message : String(err),
        }),
    );
    return () => {
      live = false;
    };
  }, [id, name, level]);
  // An answer for another spell is still loading this one's.
  const shown = load && load.id === id ? load : null;

  return (
    <Dialog open={spell !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        className="max-w-[60rem] gap-3 overflow-y-auto"
        data-testid="spell-details"
      >
        <div className="flex items-start gap-2">
          <DialogTitle className="sr-only">
            {t("spellSlots.details", { name: spell?.name ?? "" })}
          </DialogTitle>
          <DialogClose
            render={
              <Button
                size="icon-sm"
                variant="ghost"
                className="ml-auto"
                aria-label={t("spellSlots.close")}
                title={t("spellSlots.close")}
              />
            }
          >
            <X aria-hidden />
          </DialogClose>
        </div>
        {!shown && (
          <p className="text-body text-muted-foreground">{t("loading")}</p>
        )}
        {shown && "error" in shown && (
          <p className="text-body text-danger" role="alert">
            {shown.error}
          </p>
        )}
        {shown &&
          "spell" in shown &&
          (shown.spell ? (
            <SpellDetail key={shown.spell.id} spell={shown.spell} />
          ) : (
            <p className="text-body text-muted-foreground">
              {t("lookup.noMatch")}
            </p>
          ))}
      </DialogContent>
    </Dialog>
  );
}
