import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Minus, Plus, ShieldCheck } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/api/http";
import { getPower, getPowerFilterOptions } from "@/api/powers";
import { raisePower } from "@/api/psionics";
import { localized } from "@/i18n/localized";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";
import type { Power, PowerFilterOptions } from "@/types/powers";
import { PowerActions } from "../PowerActions";
import { groupsOf, heldMinds, isStanding, rowTags, takesMind } from "./model";
import { useKnownPowers } from "./useKnownPowers";

/**
 * Act > Powers (ADR 0047): the character's known powers as data. The header says which power the
 * round has used and holds the adjustment; "My defence" raises a defence mode in one tap; "Target"
 * lists the minds held, shown only when a known power takes one; the rows are grouped by
 * discipline, the attack modes first. What a row does is `PowerActions`' (the dialogs). Nothing here
 * names a power: a power added to the catalogue and learned shows with no change.
 */
export function PowersTab() {
  const known = useCharacterStore((s) => s.character?.derived?.knownPowers);
  const powers = useKnownPowers(known?.picks ?? []);
  const options = useDisciplines();
  const used = usePowerById(known?.usedThisRound);
  if (!known) return null;
  return <PowersView powers={powers} used={used} options={options} />;
}

/** The tab for the powers it is given: no fetch, so what it shows is a function of its data. */
export function PowersView({
  powers,
  used,
  options,
}: {
  powers: Power[];
  used?: Power;
  options: PowerFilterOptions | null;
}) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const defences = powers.filter((p) => p.kind === "defence_mode");
  const groups = groupsOf(powers);
  const aimed = powers.some(takesMind);
  const disciplineName = (id: string) => {
    const d = options?.disciplines.find((x) => x.id === id);
    return d ? localized(lang, d.name) : id;
  };

  return (
    <div className="flex flex-col gap-3" data-testid="powers-tab">
      <Header usedName={used ? localized(lang, used.name) : undefined} />
      <DefenceBand defences={defences} />
      {aimed && <TargetBand />}
      {powers.length === 0 && (
        <p className="text-body text-muted-foreground">{t("powers.none")}</p>
      )}
      <div className="columns-1 gap-4 xl:columns-2">
        {groups.map((g) => (
          <section
            key={g.key}
            className="mb-3 flex break-inside-avoid flex-col gap-1"
            data-testid="powers-group"
            data-group={g.key}
          >
            <h3 className="section-label">
              {g.discipline === null
                ? t("powers.tab.attackModes")
                : disciplineName(g.discipline)}
            </h3>
            <ul className="flex list-none flex-col">
              {g.powers.map((p) => (
                <PowerRow key={p.id} power={p} />
              ))}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}

/** One catalogue power by id, read once; undefined for none or until it arrives. */
function usePowerById(id: string | undefined): Power | undefined {
  const [found, setFound] = useState<Power | undefined>();
  useEffect(() => {
    if (!id) return;
    let live = true;
    getPower(id).then(
      (p) => live && setFound(p),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, [id]);
  return id && found?.id === id ? found : undefined;
}

function useDisciplines(): PowerFilterOptions | null {
  const [options, setOptions] = useState<PowerFilterOptions | null>(null);
  useEffect(() => {
    let live = true;
    getPowerFilterOptions().then(
      (o) => live && setOptions(o),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);
  return options;
}

/** The round's power (the book allows one a round: a warning, never a refusal) and the adjustment. */
function Header({ usedName }: { usedName?: string }) {
  const { t } = useTranslation();
  const [adjustment, setAdjustment] = useState(0);
  return (
    <div className="flex flex-wrap items-center justify-end gap-3">
      <span
        className={cn(
          "flex items-center gap-1.5 rounded-md bg-raised px-2 py-0.5 text-label",
          usedName ? "text-warning" : "text-ink-muted",
        )}
        data-testid="power-of-the-round"
      >
        {usedName
          ? t("powers.tab.usedThisRound", { name: usedName })
          : t("powers.tab.roundFree")}
      </span>
      <span className="flex items-center gap-2 text-label text-ink-muted">
        {t("powers.tab.adjustment")}
        <span className="flex items-center gap-1">
          <Button
            variant="quiet"
            className="px-1.5"
            aria-label={t("powers.tab.adjustmentLess")}
            onClick={() => setAdjustment((n) => n - 1)}
          >
            <Minus aria-hidden />
          </Button>
          <output
            className="w-9 text-center text-body text-foreground tabular-nums"
            data-testid="power-adjustment"
          >
            {adjustment > 0 ? `+${adjustment}` : adjustment}
          </output>
          <Button
            variant="quiet"
            className="px-1.5"
            aria-label={t("powers.tab.adjustmentMore")}
            onClick={() => setAdjustment((n) => n + 1)}
          >
            <Plus aria-hidden />
          </Button>
        </span>
      </span>
    </div>
  );
}

/** "My defence": one chip per known defence mode, raised in one tap, no roll. */
function DefenceBand({ defences }: { defences: Power[] }) {
  const { t, i18n } = useTranslation();
  const busy = useTableStore((s) => s.busy);
  const inCombat = useBuffsStore((s) => s.round > 0);
  const rows = useBuffsStore((s) => s.buffs);
  const [refusal, setRefusal] = useState<string | null>(null);

  async function raise(id: string) {
    setRefusal(null);
    await useTableStore.getState().submit(async () => {
      try {
        await raisePower(id);
      } catch (e) {
        if (e instanceof ApiError && e.status < 500) {
          setRefusal(e.message);
          return;
        }
        throw e;
      }
    });
  }

  return (
    <div
      className="group-band flex flex-col gap-2 rounded-lg px-3 py-2"
      data-testid="defence-band"
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className="section-label mr-1">{t("powers.tab.myDefence")}</span>
        {defences.map((p) => {
          const standing = isStanding(p);
          const row = rows.find((b) => b.powerId === p.id && b.defence);
          const name = localized(i18n.language, p.name);
          const outOfCombat = !inCombat && !standing;
          return (
            <button
              key={p.id}
              type="button"
              data-testid="defence-chip"
              data-defence={row?.defence ?? "down"}
              role="switch"
              aria-checked={!!row}
              aria-label={t("powers.defence.raiseNamed", { name })}
              disabled={busy || outOfCombat || (standing && !!row)}
              className={cn(
                "flex h-8 items-center gap-1.5 rounded-md px-2.5 text-body",
                row ? "chip chip-chosen" : "chip",
                outOfCombat && "border border-dashed text-ink-muted",
              )}
              onClick={() => void raise(p.id)}
            >
              <ShieldCheck
                aria-hidden
                weight={row ? "fill" : "regular"}
                className="size-4"
              />
              <span lang={i18n.language}>{name}</span>
              <span className="text-label text-ink-muted">
                {row
                  ? t(`powers.defence.kind.${row.defence}`)
                  : standing
                    ? t("powers.tab.standingFree")
                    : outOfCombat
                      ? `${t("lookup.powers.psp", { count: costOf(p) })} · ${t("powers.tab.inCombat")}`
                      : t("lookup.powers.psp", { count: costOf(p) })}
              </span>
            </button>
          );
        })}
        {defences.length === 0 && (
          <span className="text-body text-ink-muted">
            {t("powers.tab.noDefence")}
          </span>
        )}
      </div>
      {refusal && (
        <p
          role="alert"
          className="text-label text-danger"
          data-testid="raise-refused"
        >
          {refusal}
        </p>
      )}
    </div>
  );
}

const costOf = (p: Power) =>
  typeof p.initial_cost === "number" ? p.initial_cost : 0;

/** "Target": the minds a tangent or contact row holds; the attack names the rest in its dialog. */
function TargetBand() {
  const { t } = useTranslation();
  const minds = heldMinds(useBuffsStore((s) => s.buffs));
  return (
    <div
      className="group-band flex flex-wrap items-center gap-2 rounded-lg px-3 py-2"
      data-testid="target-band"
    >
      <span className="section-label mr-1">{t("powers.tab.target")}</span>
      {minds.map((m) => (
        <span
          key={m.name}
          className="chip flex h-8 items-center gap-2 rounded-md px-2.5 text-body"
          data-testid="held-mind"
        >
          {m.name}
          <span className="text-label text-ink-muted">
            {m.tangents
              ? t("powers.tab.tangents", { count: m.tangents })
              : t("powers.tab.contact")}
          </span>
        </span>
      ))}
      {minds.length === 0 && (
        <span className="text-body text-ink-muted">
          {t("powers.tab.noMind")}
        </span>
      )}
    </div>
  );
}

function PowerRow({ power }: { power: Power }) {
  const { t, i18n } = useTranslation();
  return (
    <li
      className="flex flex-wrap items-center gap-3 rounded-md px-2.5 py-1 odd:bg-raised/50"
      data-testid="power-row"
      data-id={power.id}
    >
      <span className="flex min-w-40 flex-1 basis-40 flex-col">
        <span lang={i18n.language} className="text-body font-medium">
          {localized(i18n.language, power.name)}
        </span>
        <span className="text-label text-ink-muted">
          {rowTags(t, power).join(" · ")}
        </span>
      </span>
      <PowerActions power={power} />
    </li>
  );
}
