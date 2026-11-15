import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ArrowLeft, Copy, Eye } from "@phosphor-icons/react";
import { copyCharacter, fetchOthers } from "@/api/characters";
import { Button } from "@/components/ui/button";
import { copyName } from "@/lib/copyName";
import { useCreationOptions } from "@/lib/useCreationOptions";
import { whoIs } from "@/lib/viewBanner";
import { ROSTER_PATH } from "@/lib/session";
import { useCharacterStore } from "@/stores/character";
import type { SharedCharacter } from "@/types/character";

/**
 * The banner of a read-only page (ADR 0029 step 11): it names the character and its owner and gives the
 * way back to the user's own character (a full load, so nothing of the viewed sheet stays in the page).
 * It is the one thing on the page that says why every control is locked.
 */
export function ViewBanner({ id }: { id: number }) {
  const { t } = useTranslation();
  const [others, setOthers] = useState<SharedCharacter[] | null>(null);
  const sheetName = useCharacterStore((s) => s.character?.name);
  useEffect(() => {
    let live = true;
    fetchOthers().then(
      (l) => live && setOthers(l.characters),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);
  const who = whoIs(others, id);
  const name = who.name ?? sheetName ?? "";
  const owner = who.owner || t("view.unnamedOwner");
  // A failed read leaves the name's length unchecked here; the server still refuses.
  const maxName = useCreationOptions().options?.maxNameLength ?? null;
  const [copying, setCopying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  /** Copies the viewed character into the user's own (ADR 0029 step 12) and shows their roster, where it is, not current. */
  const copy = async () => {
    setCopying(true);
    setError(null);
    try {
      await copyCharacter(
        id,
        copyName(name, (n) => t("characters.copyName", { name: n }), maxName),
      );
      globalThis.location.assign(ROSTER_PATH);
    } catch (e) {
      setCopying(false);
      setError(e instanceof Error ? e.message : String(e));
    }
  };
  return (
    <div
      role="status"
      data-view-ok
      data-testid="view-banner"
      className="zone-card flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 bg-selected! px-4 py-2 shadow-[inset_6px_0_0_var(--tint)]!"
    >
      <Eye aria-hidden className="size-5 shrink-0 text-tint" />
      <p className="min-w-0 flex-1 text-body [overflow-wrap:anywhere]">
        <span className="font-semibold">
          {t("view.banner", { name, owner })}
        </span>{" "}
        <span className="text-ink-muted">{t("view.readOnly")}</span>
        {error && (
          <span role="alert" className="block text-danger">
            {error}
          </span>
        )}
      </p>
      <Button
        variant="quiet"
        data-view-ok
        data-testid="view-copy"
        disabled={copying || !name}
        aria-label={t("characters.copyHint", { name })}
        title={t("characters.copyHint", { name })}
        onClick={() => void copy()}
      >
        <Copy aria-hidden />
        {t("characters.copy")}
      </Button>
      <Button
        variant="main"
        data-testid="view-back"
        aria-label={t("view.backHint")}
        title={t("view.backHint")}
        onClick={() => globalThis.location.assign("/")}
      >
        <ArrowLeft aria-hidden />
        {t("view.back")}
      </Button>
    </div>
  );
}
