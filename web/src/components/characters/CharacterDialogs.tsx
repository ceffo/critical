import { useEffect, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Archive, Play, X } from "@phosphor-icons/react";
import {
  fetchCharacters,
  retireCharacter,
  switchCharacter,
} from "@/api/characters";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { CharacterForm } from "./CharacterForm";
import type { CharacterList, CharacterSummary } from "@/types/character";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** The page shows the user's current character: after a change it loads again on it. */
const reloadOnCurrent = () => globalThis.location.assign("/");

/** The user's characters, read when shown. */
function useCharacters(): {
  list: CharacterList | null;
  error: string | null;
} {
  const [list, setList] = useState<CharacterList | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let live = true;
    fetchCharacters().then(
      (l) => live && setList(l),
      (e: unknown) => live && setError(message(e)),
    );
    return () => {
      live = false;
    };
  }, []);
  return { list, error };
}

/** "Bard · level 3": the class worded, the level the server derived. */
function useClassLevel() {
  const { t } = useTranslation();
  return (c: CharacterSummary) =>
    t("characters.classLevel", {
      className: t(`details.classes.${c.class}`, { defaultValue: c.class }),
      level: c.level,
    });
}

/**
 * The user's characters with a Play button on each but the current one (ADR 0029 step 7.2): it makes
 * that one current and the page loads again on it.
 */
export function CharacterList() {
  const { t } = useTranslation();
  const { list, error: loadError } = useCharacters();
  const [busy, setBusy] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const classLevel = useClassLevel();

  const play = async (c: CharacterSummary) => {
    setBusy(c.id);
    setError(null);
    try {
      await switchCharacter(c.id);
      reloadOnCurrent();
    } catch (e) {
      setError(message(e));
      setBusy(null);
    }
  };

  if (loadError)
    return (
      <p role="alert" className="text-body text-danger">
        {loadError}
      </p>
    );
  if (!list)
    return <p className="text-body text-muted-foreground">{t("loading")}</p>;
  if (list.characters.length === 0)
    return (
      <p className="text-body text-muted-foreground">{t("characters.none")}</p>
    );
  return (
    <div className="flex flex-col gap-2">
      <ul className="flex flex-col gap-1" data-testid="character-list">
        {list.characters.map((c) => {
          const current = c.id === list.currentId;
          return (
            <li
              key={c.id}
              className={
                current
                  ? "selected flex items-center gap-3 rounded-lg px-3 py-2"
                  : "flex items-center gap-3 rounded-lg bg-raised px-3 py-2"
              }
              data-current={current || undefined}
            >
              <div className="flex min-w-0 flex-1 flex-col">
                <span className="text-body font-medium [overflow-wrap:anywhere]">
                  {c.name}
                </span>
                <span className="text-label text-muted-foreground">
                  {classLevel(c)}
                </span>
              </div>
              {current ? (
                <span className="text-label text-muted-foreground">
                  {t("characters.current")}
                </span>
              ) : (
                <Button
                  variant="quiet"
                  disabled={busy !== null}
                  aria-label={t("characters.playHint", { name: c.name })}
                  title={t("characters.playHint", { name: c.name })}
                  onClick={() => void play(c)}
                >
                  <Play aria-hidden />
                  {t("characters.play")}
                </Button>
              )}
            </li>
          );
        })}
      </ul>
      {error && (
        <p role="alert" className="text-body text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

/** Retiring the current character, confirmed: it is hidden from the lists, never deleted (step 7.3). */
function RetireConfirm({ onCancel }: { onCancel: () => void }) {
  const { t } = useTranslation();
  const { list, error: loadError } = useCharacters();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const current = list?.characters.find((c) => c.id === list.currentId);

  const retire = async (c: CharacterSummary) => {
    setBusy(true);
    setError(null);
    try {
      await retireCharacter(c.id);
      reloadOnCurrent();
    } catch (e) {
      setError(message(e));
      setBusy(false);
    }
  };

  if (loadError)
    return (
      <p role="alert" className="text-body text-danger">
        {loadError}
      </p>
    );
  if (!list)
    return <p className="text-body text-muted-foreground">{t("loading")}</p>;
  if (!current)
    return (
      <p className="text-body text-muted-foreground">{t("characters.none")}</p>
    );
  return (
    <div className="flex flex-col gap-4" data-testid="retire-confirm">
      <p className="text-body [overflow-wrap:anywhere]">
        {t("characters.retireLead", { name: current.name })}
      </p>
      {error && (
        <p role="alert" className="text-body text-danger">
          {error}
        </p>
      )}
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button variant="quiet" onClick={onCancel}>
          {t("characters.cancel")}
        </Button>
        <Button
          variant="destructive"
          disabled={busy}
          aria-label={t("characters.retireHint", { name: current.name })}
          title={t("characters.retireHint", { name: current.name })}
          onClick={() => void retire(current)}
        >
          <Archive aria-hidden />
          {t("characters.retire")}
        </Button>
      </div>
    </div>
  );
}

/** Which character dialog the cog menu opened. */
export type CharacterDialog = "switch" | "new" | "retire";

/** One of the cog menu's character dialogs (ADR 0029 step 7.2: rare actions, in the cog menu). */
export function CharacterDialogs({
  open,
  onClose,
}: {
  open: CharacterDialog | null;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const titles: Record<CharacterDialog, string> = {
    switch: t("characters.switchTitle"),
    new: t("characters.newTitle"),
    retire: t("characters.retireTitle"),
  };
  const bodies: Record<CharacterDialog, ReactNode> = {
    switch: <CharacterList />,
    new: <CharacterForm onCancel={onClose} />,
    retire: <RetireConfirm onCancel={onClose} />,
  };
  return (
    <Dialog open={open !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-w-[40rem] gap-4 overflow-y-auto"
        data-testid="character-dialog"
      >
        <div className="flex items-start gap-2">
          <DialogTitle className="section-label">
            {open ? titles[open] : ""}
          </DialogTitle>
          <DialogClose
            render={
              <Button
                size="icon-sm"
                variant="ghost"
                className="ml-auto"
                aria-label={t("characters.close")}
                title={t("characters.close")}
              />
            }
          >
            <X aria-hidden />
          </DialogClose>
        </div>
        {open && bodies[open]}
      </DialogContent>
    </Dialog>
  );
}
