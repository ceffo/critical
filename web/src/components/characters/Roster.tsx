import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  Archive,
  ArrowLeft,
  PencilSimple,
  Play,
  UserPlus,
  X,
} from "@phosphor-icons/react";
import {
  fetchCharacters,
  fetchCreationOptions,
  renameCharacter,
  retireCharacter,
  switchCharacter,
} from "@/api/characters";
import { ApiError } from "@/api/http";
import { Wordmark } from "@/components/creation/kit";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { APP_NAME } from "@/lib/brand";
import { navigate } from "@/lib/navigation";
import { CREATE_PATH } from "@/lib/session";
import { cn } from "@/lib/utils";
import type { CharacterList, CharacterSummary } from "@/types/character";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** The page shows the user's current character: after a switch it loads again on it. */
const reloadOnCurrent = () => globalThis.location.assign("/");

/** A dialog the roster opened, on one character. */
type Opened = {
  kind: "rename" | "retire";
  character: CharacterSummary;
  current: boolean;
};

/**
 * "Vos personnages" (ADR 0034 step 7, amending ADR 0029 steps 7.2 and 7.3): a full screen with a card
 * per character, the one in play marked "En jeu", each with Jouer (but the current one), Renommer and
 * Retirer, and a dashed Créer card that opens the creation route. Retour goes back to the game; a
 * user with no current character (theirs retired elsewhere) has no game to go back to, so no Retour.
 * Renaming and retiring answer the list, shown as it is; a switch loads the page again on the
 * character, and so does retiring the last one (the intro is the page of a user with none).
 */
export function Roster({ onBack }: { onBack?: () => void }) {
  const { t } = useTranslation();
  const [list, setList] = useState<CharacterList | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [opened, setOpened] = useState<Opened | null>(null);

  useEffect(() => {
    document.title = `${t("roster.title")} · ${APP_NAME}`;
  }, [t]);
  useEffect(() => {
    let live = true;
    fetchCharacters().then(
      (l) => live && setList(l),
      (e: unknown) => live && setLoadError(message(e)),
    );
    return () => {
      live = false;
    };
  }, []);

  const play = async (c: CharacterSummary) => {
    setBusy(true);
    setError(null);
    try {
      await switchCharacter(c.id);
      reloadOnCurrent();
    } catch (e) {
      setError(message(e));
      setBusy(false);
    }
  };
  const changed = (l: CharacterList) => {
    setOpened(null);
    if (l.characters.length === 0) reloadOnCurrent();
    else setList(l);
  };

  return (
    <div
      className="flex h-dvh flex-col gap-3 overflow-hidden p-3"
      data-screen="roster"
    >
      <header className="zone-card flex shrink-0 items-center gap-4 px-4 py-2">
        <Wordmark />
        <span className="text-title">{t("roster.title")}</span>
        <span className="flex-1" />
        {onBack && (
          <Button
            variant="quiet"
            onClick={onBack}
            aria-label={t("roster.backHint")}
            title={t("roster.backHint")}
          >
            <ArrowLeft aria-hidden />
            {t("roster.back")}
          </Button>
        )}
      </header>
      <main
        className="stage-card min-h-0 flex-1 overflow-y-auto p-4"
        data-testid="roster"
      >
        {loadError ? (
          <p role="alert" className="text-body text-danger">
            {loadError}
          </p>
        ) : !list ? (
          <p className="text-body text-muted-foreground">{t("loading")}</p>
        ) : (
          <div className="flex flex-col gap-3">
            {error && (
              <p role="alert" className="text-body text-danger">
                {error}
              </p>
            )}
            <ul className="grid grid-cols-[repeat(auto-fill,minmax(19rem,1fr))] gap-3">
              {list.characters.map((c) => (
                <RosterCard
                  key={c.id}
                  character={c}
                  current={c.id === list.currentId}
                  busy={busy}
                  onPlay={() => void play(c)}
                  onRename={() =>
                    setOpened({
                      kind: "rename",
                      character: c,
                      current: c.id === list.currentId,
                    })
                  }
                  onRetire={() =>
                    setOpened({
                      kind: "retire",
                      character: c,
                      current: c.id === list.currentId,
                    })
                  }
                />
              ))}
              <li className="flex min-h-36 flex-col items-center justify-center gap-2 rounded-[0.625rem] border-2 border-dashed border-input p-4">
                <Button
                  variant="quiet"
                  size="lg"
                  aria-label={t("characters.newHint")}
                  title={t("characters.newHint")}
                  onClick={() => navigate(CREATE_PATH)}
                >
                  <UserPlus aria-hidden />
                  {t("characters.new")}
                </Button>
                <span className="text-center text-label text-ink-muted">
                  {t("roster.createLead")}
                </span>
              </li>
            </ul>
          </div>
        )}
      </main>
      <RosterDialog
        opened={opened}
        onClose={() => setOpened(null)}
        onChanged={changed}
      />
    </div>
  );
}

/** One character's card: name, class and level, "En jeu" on the current one, its three actions. */
function RosterCard({
  character: c,
  current,
  busy,
  onPlay,
  onRename,
  onRetire,
}: {
  character: CharacterSummary;
  current: boolean;
  busy: boolean;
  onPlay: () => void;
  onRename: () => void;
  onRetire: () => void;
}) {
  const { t } = useTranslation();
  return (
    <li
      className={cn("zone-card flex flex-col", current && "latest")}
      data-testid="roster-card"
      data-current={current || undefined}
    >
      <div className="flex flex-col gap-1 px-4 pt-3">
        <div className="flex items-start gap-2">
          <span className="min-w-0 flex-1 text-key [overflow-wrap:anywhere]">
            {c.name}
          </span>
          {current && (
            <span className="shrink-0 rounded-md bg-tint px-1.5 py-0.5 text-label font-semibold text-on-accent">
              {t("characters.current")}
            </span>
          )}
        </div>
        <span className="text-body">
          {t("characters.classLevel", {
            className: t(`details.classes.${c.class}`, {
              defaultValue: c.class,
            }),
            level: c.level,
          })}
        </span>
      </div>
      <div className="mt-auto flex flex-wrap items-center gap-2 px-4 pt-3 pb-3">
        {!current && (
          <Button
            variant="main"
            disabled={busy}
            aria-label={t("characters.playHint", { name: c.name })}
            title={t("characters.playHint", { name: c.name })}
            onClick={onPlay}
          >
            <Play aria-hidden />
            {t("characters.play")}
          </Button>
        )}
        <span className="flex-1" />
        <Button
          variant="quiet"
          disabled={busy}
          aria-label={t("roster.renameHint", { name: c.name })}
          title={t("roster.renameHint", { name: c.name })}
          onClick={onRename}
        >
          <PencilSimple aria-hidden />
          {t("characters.rename")}
        </Button>
        <Button
          variant="destructive"
          disabled={busy}
          aria-label={t("characters.retireHint", { name: c.name })}
          title={t("characters.retireHint", { name: c.name })}
          onClick={onRetire}
        >
          <Archive aria-hidden />
          {t("characters.retire")}
        </Button>
      </div>
    </li>
  );
}

/** The rename or retire dialog over the roster, on the character whose button opened it. */
function RosterDialog({
  opened,
  onClose,
  onChanged,
}: {
  opened: Opened | null;
  onClose: () => void;
  onChanged: (l: CharacterList) => void;
}) {
  const { t } = useTranslation();
  let body: ReactNode = null;
  if (opened?.kind === "rename")
    body = (
      <RenameForm
        character={opened.character}
        onCancel={onClose}
        onDone={onChanged}
      />
    );
  if (opened?.kind === "retire")
    body = (
      <RetireConfirm
        character={opened.character}
        current={opened.current}
        onCancel={onClose}
        onDone={onChanged}
      />
    );
  return (
    <Dialog open={opened !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-w-[40rem] gap-4 overflow-y-auto"
        data-testid="character-dialog"
      >
        <div className="flex items-start gap-2">
          <DialogTitle className="section-label">
            {opened?.kind === "rename"
              ? t("characters.renameTitle")
              : t("characters.retireTitle")}
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
        {body}
      </DialogContent>
    </Dialog>
  );
}

/** Retiring a character, confirmed: it is hidden from the lists, never deleted (ADR 0029 step 7.3). */
function RetireConfirm({
  character,
  current,
  onCancel,
  onDone,
}: {
  character: CharacterSummary;
  /** The one in play: the server then makes the first one left current, which the lead says. */
  current: boolean;
  onCancel: () => void;
  onDone: (l: CharacterList) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const retire = async () => {
    setBusy(true);
    setError(null);
    try {
      onDone(await retireCharacter(character.id));
    } catch (e) {
      setError(message(e));
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-col gap-4" data-testid="retire-confirm">
      <p className="text-body [overflow-wrap:anywhere]">
        {t(current ? "characters.retireLead" : "characters.retireLeadOther", {
          name: character.name,
        })}
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
          aria-label={t("characters.retireHint", { name: character.name })}
          title={t("characters.retireHint", { name: character.name })}
          onClick={() => void retire()}
        >
          <Archive aria-hidden />
          {t("characters.retire")}
        </Button>
      </div>
    </div>
  );
}

/**
 * Renaming a character (ADR 0029 step 7.3): its name, edited in place. The server trims and checks it
 * and refuses with a code the form words.
 */
function RenameForm({
  character,
  onCancel,
  onDone,
}: {
  character: CharacterSummary;
  onCancel: () => void;
  onDone: (l: CharacterList) => void;
}) {
  const { t } = useTranslation();
  const [maxName, setMaxName] = useState<number | null>(null);
  const [name, setName] = useState(character.name);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    fetchCreationOptions().then(
      (o) => live && setMaxName(o.maxNameLength),
      () => {},
    );
    return () => {
      live = false;
    };
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      onDone(await renameCharacter(character.id, name));
    } catch (err) {
      setBusy(false);
      setError(
        err instanceof ApiError && err.field === "name"
          ? t(`characters.errors.${err.message}`, {
              max: maxName,
              defaultValue: err.message,
            })
          : message(err),
      );
    }
  };

  return (
    <form
      noValidate
      onSubmit={(e) => void submit(e)}
      className="flex flex-col gap-4"
      data-testid="rename-form"
    >
      <div className="flex min-w-0 flex-col gap-1">
        <label htmlFor="rename-character-name" className={labelClass}>
          {t("characters.name")}
        </label>
        <input
          id="rename-character-name"
          autoComplete="off"
          autoFocus
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? "rename-character-error" : undefined}
          className={cn(fieldClass, error && "border-destructive")}
          value={name}
          onChange={(e) => {
            setName(e.target.value);
            setError(null);
          }}
        />
        {error && (
          <p
            id="rename-character-error"
            role="alert"
            className="text-label text-destructive"
            data-testid="field-error"
          >
            {error}
          </p>
        )}
      </div>
      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button type="button" variant="quiet" onClick={onCancel}>
          {t("characters.cancel")}
        </Button>
        <Button
          type="submit"
          variant="main"
          disabled={busy}
          aria-label={t("characters.renameHint", { name: character.name })}
          title={t("characters.renameHint", { name: character.name })}
        >
          <PencilSimple aria-hidden />
          {t("characters.rename")}
        </Button>
      </div>
    </form>
  );
}
