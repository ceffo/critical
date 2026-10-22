import { Plus, X } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { draftClasses, type StepProps } from "./flow";
import { Group } from "./kit";

/** A refusal under a field, worded; same look as the class step's. */
function Refusal({ id, text }: { id: string; text: string | null }) {
  if (!text) return null;
  return (
    <p
      id={`${id}-error`}
      role="alert"
      className="text-label text-destructive"
      data-testid="field-error"
    >
      {text}
    </p>
  );
}

/**
 * The class entries of a multi-class or dual-class character (ADR 0042 steps 1 and 4): per class its
 * XP (the level follows from it) and, for the class a dual-class character left, the level it was
 * left at. Which combinations are allowed is the server's verdict, shown at the field it names.
 */
export function ClassRows({
  options,
  draft,
  update,
  errorFor,
  answer,
}: StepProps) {
  const { t } = useTranslation();
  const dual = draft.kind === "dual";
  const max = options.maxClasses ?? 3;
  const used = draftClasses(draft);
  const rows = [
    { cls: draft.cls, xp: draft.xp },
    ...draft.others.map((o) => ({ cls: o.cls, xp: o.xp })),
  ];
  const setRow = (i: number, patch: Partial<{ cls: string; xp: string }>) => {
    if (i === 0) {
      update({
        ...(patch.cls !== undefined && { cls: patch.cls }),
        ...(patch.xp !== undefined && { xp: patch.xp }),
      });
      return;
    }
    update({
      others: draft.others.map((o, j) =>
        j === i - 1 ? { ...o, ...patch } : o,
      ),
    });
  };

  return (
    <Group
      title={t(dual ? "creation.classes.dualTitle" : "creation.classes.title")}
      testId="class-rows"
    >
      <p className="text-body text-ink-muted">
        {t(dual ? "creation.classes.dualLead" : "creation.classes.lead", {
          max,
          prime: options.dualClassOldPrime,
          primeNew: options.dualClassNewPrime,
          min: options.minLeftAtLevel,
        })}
      </p>
      <div className="flex flex-col gap-3">
        {rows.map((r, i) => {
          const field = (name: string) => `classes.${i}.${name}`;
          const id = (name: string) => `new-character-${field(name)}`;
          const classError = errorFor(field("class"));
          const xpError = errorFor(field("xp"));
          const leftError = errorFor(field("leftAtLevel"));
          const unmet =
            answer.latest?.unmetMinimums.find((u) => u.class === r.cls)
              ?.abilities ?? [];
          const left = dual && i === 0;
          return (
            <div
              key={i}
              className="zone-card flex flex-wrap items-start gap-4 p-3"
              data-testid={`class-row-${i}`}
            >
              <div className="flex w-56 min-w-0 flex-col gap-1">
                <label htmlFor={id("class")} className={labelClass}>
                  {dual
                    ? t(
                        i === 0
                          ? "creation.classes.left"
                          : "creation.classes.new",
                      )
                    : t("creation.classes.nth", { n: i + 1 })}
                </label>
                <select
                  id={id("class")}
                  value={r.cls}
                  aria-invalid={classError ? true : undefined}
                  aria-describedby={
                    classError ? `${id("class")}-error` : undefined
                  }
                  className={cn(fieldClass, classError && "border-destructive")}
                  onChange={(e) => setRow(i, { cls: e.target.value })}
                >
                  <option value="">{t("creation.classes.choose")}</option>
                  {options.classes.map((c) => (
                    <option
                      key={c.class}
                      value={c.class}
                      disabled={c.class !== r.cls && used.includes(c.class)}
                    >
                      {t(`details.classes.${c.class}`, {
                        defaultValue: c.class,
                      })}
                    </option>
                  ))}
                </select>
                <Refusal id={id("class")} text={classError} />
              </div>
              {left && (
                <div className="flex w-40 min-w-0 flex-col gap-1">
                  <label htmlFor={id("leftAtLevel")} className={labelClass}>
                    {t("creation.classes.leftAt")}
                  </label>
                  <input
                    id={id("leftAtLevel")}
                    inputMode="numeric"
                    autoComplete="off"
                    value={draft.leftAt}
                    aria-invalid={leftError ? true : undefined}
                    aria-describedby={
                      leftError ? `${id("leftAtLevel")}-error` : undefined
                    }
                    className={cn(
                      fieldClass,
                      leftError && "border-destructive",
                    )}
                    onChange={(e) => update({ leftAt: e.target.value })}
                  />
                  <Refusal id={id("leftAtLevel")} text={leftError} />
                </div>
              )}
              <div className="flex w-44 min-w-0 flex-col gap-1">
                <label htmlFor={id("xp")} className={labelClass}>
                  {left ? t("creation.classes.xpLeft") : t("creation.level.xp")}
                </label>
                <input
                  id={id("xp")}
                  inputMode="numeric"
                  autoComplete="off"
                  value={r.xp}
                  aria-invalid={xpError ? true : undefined}
                  aria-describedby={xpError ? `${id("xp")}-error` : undefined}
                  className={cn(fieldClass, xpError && "border-destructive")}
                  onChange={(e) => setRow(i, { xp: e.target.value })}
                />
                <Refusal id={id("xp")} text={xpError} />
              </div>
              {unmet.length > 0 && (
                <p
                  className="text-label text-danger"
                  data-testid={`row-unmet-${i}`}
                >
                  {t("creation.class.minimumUnmet")}:{" "}
                  {unmet.map((a) => t(`attr.${a}`)).join(", ")}
                </p>
              )}
              {!dual && i >= 2 && (
                <Button
                  variant="quiet"
                  size="sm"
                  className="ml-auto"
                  aria-label={t("creation.classes.remove")}
                  title={t("creation.classes.remove")}
                  onClick={() =>
                    update({
                      others: draft.others.filter((_, j) => j !== i - 1),
                    })
                  }
                >
                  <X aria-hidden />
                </Button>
              )}
            </div>
          );
        })}
      </div>
      {!dual && rows.length < max && (
        <div>
          <Button
            variant="quiet"
            size="sm"
            onClick={() =>
              update({ others: [...draft.others, { cls: "", xp: "" }] })
            }
          >
            <Plus aria-hidden />
            {t("creation.classes.add")}
          </Button>
        </div>
      )}
      <Refusal id="new-character-classes" text={errorFor("classes")} />
    </Group>
  );
}
