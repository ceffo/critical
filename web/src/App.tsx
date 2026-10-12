import { useEffect } from "react";
import { LeftPanel } from "@/components/panels/LeftPanel";
import { CenterPanel } from "@/components/panels/CenterPanel";
import { RightPanel } from "@/components/panels/RightPanel";
import { useCharacterStore } from "@/stores/character";
import { useTranslation } from "react-i18next";
import { LanguageToggle } from "@/components/LanguageToggle";

function App() {
  const { t } = useTranslation();
  const { character, loadCharacter } = useCharacterStore();

  useEffect(() => {
    loadCharacter();
  }, [loadCharacter]);

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-background">
      {/* Top Header - Persistent Stats */}
      <header className="border-b bg-card px-4 py-2 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-black tracking-tighter uppercase">
            {character?.name || t("app.loading")}
          </h1>
          <div className="flex items-center gap-1">
            <span className="text-[10px] font-bold text-muted-foreground uppercase">
              {t("app.level")}
            </span>
            <span className="text-sm font-black">{character?.level}</span>
          </div>
        </div>

        <div className="flex gap-4">
          {character &&
            [
              { k: "str", l: t("attr.str"), v: character.str },
              { k: "dex", l: t("attr.dex"), v: character.dex },
              { k: "con", l: t("attr.con"), v: character.con },
              { k: "int", l: t("attr.int"), v: character.int },
              { k: "wis", l: t("attr.wis"), v: character.wis },
              { k: "cha", l: t("attr.cha"), v: character.cha },
            ].map((a) => (
              <div
                key={a.k}
                className="flex flex-col items-center leading-none"
              >
                <span className="text-[8px] font-bold text-muted-foreground">
                  {a.l}
                </span>
                <span className="text-sm font-black">{a.v}</span>
              </div>
            ))}
        </div>

        <div className="flex items-center gap-2">
          <div className="text-[10px] font-bold text-muted-foreground uppercase">
            {t("app.thac0")}
          </div>
          <div className="text-sm font-black">{character?.thac0Base}</div>
          <LanguageToggle />
        </div>
      </header>

      <div className="flex-1 grid grid-cols-[280px_1fr_240px] overflow-hidden">
        <aside className="border-r overflow-hidden">
          <LeftPanel />
        </aside>
        <main className="overflow-hidden">
          <CenterPanel />
        </main>
        <aside className="border-l overflow-hidden">
          <RightPanel />
        </aside>
      </div>
    </div>
  );
}

export default App;
