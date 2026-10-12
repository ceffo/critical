import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { LANGUAGES, setLanguage, type Language } from "@/i18n";

export function LanguageToggle() {
  const { t, i18n } = useTranslation();
  return (
    <div
      className="flex items-center gap-1"
      role="group"
      aria-label={t("language.label")}
    >
      {LANGUAGES.map((lng: Language) => (
        <Button
          key={lng}
          size="xs"
          variant={i18n.resolvedLanguage === lng ? "default" : "ghost"}
          title={t(`language.${lng}`)}
          onClick={() => void setLanguage(lng)}
        >
          {lng.toUpperCase()}
        </Button>
      ))}
    </div>
  );
}
