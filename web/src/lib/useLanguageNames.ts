import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import type { CreationOptions } from "@/types/character";

/** The name of a catalogue language in the UI language (English when it has none), else its code. */
export function useLanguageNames(
  options: Pick<CreationOptions, "languages"> | null,
): (code: string) => string {
  const { i18n } = useTranslation();
  return (code) => {
    const language = options?.languages?.find((l) => l.code === code);
    return (language && localized(i18n.language, language.name)) || code;
  };
}
