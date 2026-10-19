import { useTranslation } from "react-i18next";
import { useLanguageNames } from "@/lib/useLanguageNames";
import { raceLanguages } from "@/lib/languages";
import type { CreationOptions } from "@/types/character";

/**
 * The languages of a character (ADR 0032 step 4.3): the whole catalogue is offered, the race's PHB
 * list first and the others after it, since the DM may allow any. The race's own language is always
 * known and cannot be unticked (step 4.1). Going over PHB Table 4's number is a warning the server
 * returns (step 4.2), never computed here. Offers nothing
 * without a catalogue (before catalogue format 8, or a release that predates it).
 */
export function LanguagePicker({
  idPrefix,
  options,
  race,
  picked,
  onChange,
}: {
  idPrefix: string;
  options: CreationOptions;
  race: string;
  picked: string[];
  onChange: (codes: string[]) => void;
}) {
  const { t } = useTranslation();
  const name = useLanguageNames(options);
  const catalogue = options.languages ?? [];
  if (catalogue.length === 0) return null;
  const { native, list } = raceLanguages(options, race);
  const inList = catalogue.filter((l) => list.includes(l.code));
  const others = catalogue.filter((l) => !list.includes(l.code));

  const toggle = (code: string, on: boolean) =>
    onChange(
      on
        ? [...picked.filter((l) => l !== code), code]
        : picked.filter((l) => l !== code),
    );
  const group = (title: string, items: typeof catalogue) =>
    items.length === 0 ? null : (
      <div className="flex min-w-0 flex-col gap-1">
        <span className="text-label text-muted-foreground">{title}</span>
        <div className="flex flex-wrap gap-x-4 gap-y-1">
          {items.map((l) => {
            const isNative = l.code === native;
            const id = `${idPrefix}-${l.code}`;
            return (
              <label
                key={l.code}
                htmlFor={id}
                className="flex min-w-0 items-center gap-1.5 text-body"
                title={isNative ? t("details.languageNative") : undefined}
              >
                <input
                  id={id}
                  type="checkbox"
                  className="size-4 shrink-0"
                  checked={isNative || picked.includes(l.code)}
                  disabled={isNative}
                  onChange={(e) => toggle(l.code, e.target.checked)}
                />
                <span className="break-words">{name(l.code)}</span>
              </label>
            );
          })}
        </div>
      </div>
    );

  return (
    <div
      className="flex min-w-0 flex-col gap-2"
      role="group"
      aria-label={t("details.fields.languages")}
      data-testid="language-picker"
    >
      {group(t("details.languagesRaceList"), inList)}
      {group(t("details.languagesOthers"), others)}
    </div>
  );
}
