import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
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
import { useCreationOptions } from "@/lib/useCreationOptions";
import {
  initialRoster,
  rosterReducer,
  startedWrite,
  type RosterAction,
  type RosterDialog as Opened,
  type RosterState,
} from "@/lib/rosterWrites";
import { identityLine, toPickKey } from "@/lib/rosterCard";
import { CREATE_PATH } from "@/lib/session";
import { cn } from "@/lib/utils";
import type { CharacterList, CharacterSummary } from "@/types/character";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/** The page shows the user's current character: after a switch it loads again on it. */
const reloadOnCurrent = () => globalThis.location.assign("/");

/** Sends one write through the roster: throws its failure, for the form to show. */
type Send = (call: () => Promise<CharacterList>) => Promise<void>;

/**
 * The roster's state (`lib/rosterWrites.ts`), applied at once so a write knows its id before its
 * answer comes back.
 */
function useRosterState(): [
  RosterState,
  (a: RosterAction) => RosterState,
  () => RosterState,
] {
  const ref = useRef(initialRoster);
  const [state, setState] = useState(initialRoster);
  const apply = useCallback((a: RosterAction) => {
    ref.current = rosterReducer(ref.current, a);
    setState(ref.current);
    return ref.current;
  }, []);
  const get = useCallback(() => ref.current, []);
  return [state, apply, get];
}

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
  const [state, apply, current] = useRosterState();
  const { list, opened, pending } = state;
  const [loadError, setLoadError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    document.title = `${t("roster.title")} · ${APP_NAME}`;
  }, [t]);
  useEffect(() => {
    let live = true;
    fetchCharacters().then(
      (l) => live && apply({ type: "loaded", list: l }),
      (e: unknown) => live && setLoadError(message(e)),
    );
    return () => {
      live = false;
    };
  }, [apply]);

  /**
   * One write at a time (`lib/rosterWrites.ts`): its answer replaces the list and closes the dialog it
   * came from, never one opened since. Retiring the last character loads the page again (the intro).
   */
  const write = async (
    fromDialog: boolean,
    call: () => Promise<CharacterList>,
  ): Promise<CharacterList | null> => {
    const before = current();
    const id = startedWrite(before, apply({ type: "start", fromDialog }));
    if (id === null) return null;
    try {
      const l = await call();
      apply({ type: "done", id, list: l });
      if (l.characters.length === 0) reloadOnCurrent();
      return l;
    } catch (e) {
      apply({ type: "failed", id });
      throw e;
    }
  };
  const send: Send = async (call) => {
    await write(true, call);
  };
  const play = async (c: CharacterSummary) => {
    setError(null);
    try {
      if (await write(false, () => switchCharacter(c.id))) reloadOnCurrent();
    } catch (e) {
      setError(message(e));
    }
  };

  return (
    <div
      className="flex h-app flex-col gap-3 overflow-hidden px-3 pt-3"
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
          <p className="text-body text-muted-foreground">{t("app.loading")}</p>
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
                  busy={pending !== null}
                  onPlay={() => void play(c)}
                  onRename={() =>
                    apply({ type: "open", kind: "rename", character: c })
                  }
                  onRetire={() =>
                    apply({ type: "open", kind: "retire", character: c })
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
        onClose={() => apply({ type: "close" })}
        send={send}
      />
    </div>
  );
}

/** One character's card: name, class and level, race · alignment, HP, the tag of what is still to choose, "En jeu" on the current one, its three actions. */
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
  // Race · alignment: what is still to choose is the tag's, not a blank here.
  const identity = identityLine(c, (key, code) =>
    t(`${key}.${code}`, { defaultValue: code }),
  );
  const toPick = toPickKey(c.missing);
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
        {identity && (
          <span
            className="text-body text-ink-muted [overflow-wrap:anywhere]"
            data-testid="roster-identity"
          >
            {identity}
          </span>
        )}
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          {c.currentHp !== undefined && c.maxHp !== undefined && (
            <span className="text-body font-semibold" data-testid="roster-hp">
              {t("roster.hp", { current: c.currentHp, max: c.maxHp })}
            </span>
          )}
          {toPick && (
            <span
              className="rounded-md bg-tint px-1.5 py-0.5 text-label font-semibold text-on-accent"
              data-testid="roster-missing"
            >
              {t(toPick)}
            </span>
          )}
        </div>
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
  send,
}: {
  opened: Opened | null;
  onClose: () => void;
  send: Send;
}) {
  const { t } = useTranslation();
  let body: ReactNode = null;
  if (opened?.kind === "rename")
    body = (
      <RenameForm
        key={opened.token}
        character={opened.character}
        onCancel={onClose}
        send={send}
      />
    );
  if (opened?.kind === "retire")
    body = (
      <RetireConfirm
        key={opened.token}
        character={opened.character}
        current={opened.current}
        onCancel={onClose}
        send={send}
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
  send,
}: {
  character: CharacterSummary;
  /** The one in play: the server then makes the first one left current, which the lead says. */
  current: boolean;
  onCancel: () => void;
  send: Send;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const retire = async () => {
    setBusy(true);
    setError(null);
    try {
      await send(() => retireCharacter(character.id));
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
  send,
}: {
  character: CharacterSummary;
  onCancel: () => void;
  send: Send;
}) {
  const { t } = useTranslation();
  // A failed read leaves the name's length unchecked here; the server still refuses.
  const maxName = useCreationOptions().options?.maxNameLength ?? null;
  const [name, setName] = useState(character.name);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await send(() => renameCharacter(character.id, name));
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
