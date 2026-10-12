import { useTranslation } from "react-i18next";
import { Package } from "lucide-react";

/** Placeholder: charged items and consumables plug in here (critical-qki.11). */
export function ItemsPanel() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col items-center gap-2 py-10 text-center text-sm text-muted-foreground">
      <Package className="size-6" />
      <p>{t("items.placeholder")}</p>
    </div>
  );
}
