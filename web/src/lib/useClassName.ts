import { useTranslation } from "react-i18next";

/** The name of a class code in the UI language (`details.classes.<code>`); the code itself when the web has no wording. */
export function useClassName(): (code: string) => string {
  const { t, i18n } = useTranslation();
  return (code) =>
    i18n.exists(`details.classes.${code}`)
      ? t(`details.classes.${code}`)
      : code;
}
