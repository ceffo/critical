import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { getTheme, nextTheme, setTheme } from "@/lib/theme";

/** One button cycling light → dark → system; shows the current preference. */
export function ThemeToggle() {
  const { t } = useTranslation();
  const [theme, set] = useState(getTheme);
  return (
    <Button
      size="xs"
      variant="outline"
      data-testid="theme-toggle"
      data-theme={theme}
      aria-label={t("theme.label", { theme: t(`theme.${theme}`) })}
      title={t("theme.label", { theme: t(`theme.${theme}`) })}
      onClick={() => {
        const n = nextTheme(theme);
        setTheme(n);
        set(n);
      }}
    >
      {t(`theme.${theme}`)}
    </Button>
  );
}
