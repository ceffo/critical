import {
  expect,
  type Locator,
  type BrowserContext,
  type Page,
  type Request,
} from "@playwright/test";

/** The API requests a context has in flight, and when one last started or ended (see `apiSettled`). */
const apiActivity = new WeakMap<
  BrowserContext,
  { inFlight: Set<Request>; last: number }
>();

/** Start counting a context's API requests; the fixtures call it once per context, before any page. */
export function trackApi(context: BrowserContext) {
  const state = { inFlight: new Set<Request>(), last: Date.now() };
  apiActivity.set(context, state);
  const isApi = (r: Request) => new URL(r.url()).pathname.startsWith("/api/");
  context.on("request", (r) => {
    if (!isApi(r)) return;
    state.inFlight.add(r);
    state.last = Date.now();
  });
  const ended = (r: Request) => {
    if (!isApi(r)) return;
    state.inFlight.delete(r);
    state.last = Date.now();
  };
  context.on("requestfinished", ended);
  context.on("requestfailed", ended);
  // A navigation drops the requests of the page it leaves without always reporting them.
  context.on("page", (page) =>
    page.on("framenavigated", (frame) => {
      if (frame !== page.mainFrame()) return;
      for (const r of state.inFlight)
        if (r.frame() === frame) state.inFlight.delete(r);
      state.last = Date.now();
    }),
  );
}

/**
 * Wait until the page has no API request in flight and none has started or ended for `quietMs`: its
 * loads and the refetches that follow a write have landed. A refetch starts the moment the answer
 * that names it arrives, so a short quiet window is enough even with slow answers.
 */
export async function apiSettled(page: Page, quietMs = 250) {
  const state = apiActivity.get(page.context());
  if (!state) throw new Error("trackApi was not called for this context");
  await expect
    .poll(
      () => state.inFlight.size === 0 && Date.now() - state.last >= quietMs,
      {
        intervals: [50],
      },
    )
    .toBe(true);
}

/** Current HP as shown on the character card ("current / max"). */
export async function currentHp(page: Page): Promise<number> {
  const text = (await page.getByTestId("hp-value").textContent()) ?? "";
  const m = /^\s*(-?\d+)\s*\/\s*(\d+)/.exec(text);
  if (!m) throw new Error(`unexpected HP text: ${JSON.stringify(text)}`);
  return Number(m[1]);
}

/** Current turn (round) as shown in the header's turn block. */
export async function currentRound(page: Page): Promise<number> {
  const text = (await page.getByTestId("round-value").textContent()) ?? "";
  if (!/^\d+$/.test(text.trim())) throw new Error(`unexpected round: ${text}`);
  return Number(text);
}

/**
 * The API reads (GET paths, sorted) the page makes while `act` runs and until no request has been in
 * flight for `quietMs`. A write's answer names the state keys it changed and the page reads those
 * again, nothing else (ADR 0018 step 11), so an action's reads are a short, known list.
 */
export async function readsAfter(
  page: Page,
  act: () => Promise<void>,
  quietMs = 600,
): Promise<string[]> {
  const reads: string[] = [];
  // The requests this window saw start: one that began before it and ends inside it must not count.
  const inFlight = new Set<Request>();
  let last = Date.now();
  const started = (r: Request) => {
    const url = new URL(r.url());
    if (!url.pathname.startsWith("/api/")) return;
    inFlight.add(r);
    last = Date.now();
    if (r.method() === "GET") reads.push(url.pathname);
  };
  const ended = (r: Request) => {
    if (!new URL(r.url()).pathname.startsWith("/api/")) return;
    inFlight.delete(r);
    last = Date.now();
  };
  page.on("request", started);
  page.on("requestfinished", ended);
  page.on("requestfailed", ended);
  try {
    await act();
    await expect
      .poll(() => inFlight.size === 0 && Date.now() - last >= quietMs, {
        intervals: [100],
        timeout: 15_000,
      })
      .toBe(true);
  } finally {
    page.off("request", started);
    page.off("requestfinished", ended);
    page.off("requestfailed", ended);
  }
  return reads.sort();
}

/** Open the dashboard and wait until server state is loaded. */
export async function openDashboard(page: Page) {
  await page.goto("/");
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  await expect(page.getByTestId("round-value")).toHaveText(/^\d+$/);
  // The panels load their own state after the first paint (attack form, crit table...): a spec that
  // counts reads or acts at once would race them.
  await apiSettled(page);
}

/**
 * An API call made from the test, with the page's cookies and its Origin, not from inside the page:
 * the page may reload itself ("character changed", api/http.ts) and destroy the context a
 * `page.evaluate` runs in. True when the call succeeded, else the status and body.
 */
export async function apiCall(
  page: Page,
  method: string,
  url: string,
  body: unknown,
): Promise<true | string> {
  const res = await page.request.fetch(url, {
    method,
    data: body,
    headers: { Origin: new URL(page.url()).origin },
  });
  return res.ok() || `${res.status()} ${await res.text()}`;
}

/**
 * Reload the page after the test changed the current character through the API. A request the page had
 * in flight then answers "character changed" and the page reloads itself, which can abort this reload
 * (then that reload is the one wanted); wait for the load.
 */
export async function reloadSettled(page: Page) {
  try {
    await page.reload();
  } catch (e) {
    // The page's own reload got there first: that is the fresh load wanted.
    if (!/ERR_ABORTED|detached|not attached/i.test(String(e))) throw e;
  }
  await page.waitForLoadState("load");
}

/** Mark the window; `expectNoReload` later proves the page was never reloaded. */
export async function markNoReload(page: Page) {
  await page.evaluate(() => {
    (window as unknown as { __noReload: boolean }).__noReload = true;
  });
}

export async function expectNoReload(page: Page) {
  expect(
    await page.evaluate(
      () => (window as unknown as { __noReload?: boolean }).__noReload,
    ),
  ).toBe(true);
}

/** Record damage or healing through the quick HP entry; waits for it to be done unless it is `refused`. */
export async function recordHp(
  page: Page,
  kind: "damage" | "heal",
  amount: number,
  source: string,
  { refused = false }: { refused?: boolean } = {},
) {
  const box = page.getByRole("group", { name: "HP change" });
  await box.getByLabel("Amount").fill(String(amount));
  await box.getByLabel("Source").fill(source);
  await box
    .getByRole("button", {
      name: kind === "damage" ? "Damage" : "Healing",
      exact: true,
    })
    .click();
  // The fields clear once the change and its refetches are done: typing the next change before that
  // would be wiped. A refused change (`refused`) keeps them.
  if (!refused) await expect(box.getByLabel("Amount")).toHaveValue("");
}

export function combatLog(page: Page): Locator {
  return page.getByRole("region", { name: "Journal", exact: true });
}

/** The journal's group of turn n ("Round 3"; turn 0 is "Out of combat"). */
export function roundGroup(page: Page, n: number | string): Locator {
  const turn = Number(n);
  return combatLog(page).getByRole("region", {
    name: turn > 0 ? `Round ${turn}` : "Out of combat",
    exact: true,
  });
}

/** The log entry whose summary contains `source` (use unique sources per test). */
export function logEntry(page: Page, source: string): Locator {
  return combatLog(page).getByRole("listitem").filter({ hasText: source });
}

/** The row of the active buff called `name` in the header's effects timeline. */
export function buffBadge(page: Page, name: string): Locator {
  return page.locator(`[data-testid="buff"][data-buff="${name}"]`);
}

export interface NewBuff {
  name: string;
  rounds?: number;
  charges?: number;
  /** [effect key as on the wire, value], e.g. ["str_bonus", 6]. */
  effects?: [string, number][];
  note?: string;
  /** Tick "Save for reuse". */
  save?: boolean;
}

/** Open the timeline's Add popover (closed beforehand) on one of its tabs; returns the popover. */
export async function openAddEffect(
  page: Page,
  tab: "Received spell" | "Saved effect" | "New effect",
): Promise<Locator> {
  const popover = page.getByTestId("add-effect-popover");
  // A popover still animating out of an earlier step must be gone before it is reopened.
  await expect(popover).toHaveCount(0);
  await page
    .getByRole("group", { name: "Effects" })
    .getByRole("button", { name: "Add effect" })
    .click();
  await popover.getByRole("tab", { name: tab }).click();
  return popover;
}

/** The spell search of the Add popover's "Received spell" tab. */
export async function receivePicker(page: Page): Promise<Locator> {
  const popover = await openAddEffect(page, "Received spell");
  return popover.getByRole("region", { name: "Received spell" });
}

/** Add a buff through the Add popover's New effect tab and wait for its row. */
export async function addBuff(page: Page, buff: NewBuff) {
  const popover = await openAddEffect(page, "New effect");
  const form = popover.getByRole("form", { name: "Add effect" });
  await form.getByLabel("Effect name").fill(buff.name);
  await form.getByLabel("Rounds").fill(String(buff.rounds ?? 0));
  await form.getByLabel("Charges").fill(String(buff.charges ?? 0));
  for (const [i, [key, value]] of (buff.effects ?? []).entries()) {
    await form.getByRole("button", { name: "Effect", exact: true }).click();
    const row = form.getByTestId("effect-row").nth(i);
    await row.getByLabel("Effect", { exact: true }).selectOption(key);
    await row.getByLabel("Effect value").fill(String(value));
  }
  if (buff.note) await form.getByLabel("Note").fill(buff.note);
  if (buff.save) await form.getByLabel("Save for reuse").check();
  await form.getByRole("button", { name: "Activate" }).click();
  // Past three effects the new row sits behind the timeline's expander: open it (left open).
  const closed = page.locator(
    "[data-testid=effects-expander][aria-expanded=false]",
  );
  await expect(buffBadge(page, buff.name).or(closed)).toBeVisible();
  if (await closed.isVisible()) await closed.click();
  await expect(buffBadge(page, buff.name)).toBeVisible();
  // The write's refetches are done: the store ignores an action while one is running.
  await apiSettled(page);
}

/** Undo the log entry whose text contains `text`: it must be the one on top; the journal's one
 * Undo button pops it. */
export async function undoEntry(page: Page, text: string) {
  const entry = logEntry(page, text);
  // The journal has read the entry: Undo sends the top this page last read.
  await expect(entry).toBeVisible();
  await page.getByTestId("undo").click();
  await expect(entry).toHaveAttribute("data-status", "voided");
}

/** The journal's Undo tooltip names `text` (the entry on top), or, with `not`, does not. */
export async function expectUndoOn(page: Page, text: string, not = false) {
  const undo = page.getByTestId("undo");
  const re = new RegExp(text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  if (not) await expect(undo).not.toHaveAttribute("title", re);
  else await expect(undo).toHaveAttribute("title", re);
}

/** Effective STR as shown in the header. */
export async function effectiveStr(page: Page): Promise<number> {
  return Number((await page.getByTestId("str-value").textContent()) ?? "NaN");
}

/** Focus an activity from the rail (English labels). */
export async function openWorkspace(
  page: Page,
  name: "Act" | "Inventory" | "Spells" | "Experience",
) {
  await page
    .getByRole("navigation", { name: "Activities" })
    .getByRole("button", { name, exact: true })
    .click();
}

/** Open the settings popover from the dice footer's cog; returns the popover. */
export async function openSettings(page: Page): Promise<Locator> {
  await page.getByTestId("settings-trigger").click();
  const popover = page.getByTestId("settings-popover");
  await expect(popover).toBeVisible();
  return popover;
}

/** Pick one option in the settings popover (by its visible name), then close it. */
export async function chooseSetting(page: Page, option: string) {
  const popover = await openSettings(page);
  const radio = popover.getByRole("radio", { name: option, exact: true });
  await radio.click();
  await expect(radio).toHaveAttribute("aria-checked", "true");
  await page.keyboard.press("Escape");
  await expect(popover).toBeHidden();
}

/**
 * Pick a theme in the settings popover (by its visible name, in either language), then close it.
 * The list shows the themes of the appearance in effect (ADR 0026 step 4.6), so pick the
 * appearance first.
 */
export async function chooseTheme(page: Page, name: string) {
  const popover = await openSettings(page);
  const radio = popover
    .getByTestId("settings-theme")
    .getByRole("radio", { name, exact: true });
  await radio.click();
  await expect(radio).toHaveAttribute("aria-checked", "true");
  await page.keyboard.press("Escape");
  await expect(popover).toBeHidden();
}

/** Switch the UI language through the settings popover. */
export async function chooseLanguage(page: Page, lang: "en" | "fr") {
  await chooseSetting(page, lang === "fr" ? "Français" : "English");
}

/** Next turn from the turn block (English UI). */
export async function nextRound(page: Page) {
  await page.getByRole("button", { name: "Next round" }).click();
}

/** Open the turn block's ⋯ menu and pick an action (English UI). */
export async function roundMenu(
  page: Page,
  item: "Time passes…" | "Set round number…" | "End combat",
) {
  await page.getByRole("button", { name: "More round actions" }).click();
  await page.getByRole("menuitem", { name: item }).click();
}

/**
 * Undo the Next turn on top: its advance and the initiative it rolled are one transaction
 * (ADR 0018), so the journal's one Undo pops both. `advance` is the advance entry's text.
 */
export async function undoNextRound(page: Page, advance: Locator) {
  const id = await page
    .getByTestId("initiative-value")
    .getAttribute("data-entry-id");
  const initiative = page.locator(`li[data-entry-id="${id}"]`);
  await page.getByTestId("undo").click();
  await expect(initiative).toHaveAttribute("data-status", "voided");
  await expect(advance).toHaveAttribute("data-status", "voided");
}

/** The Weapons tab's attack form (target bar, weapon in hand, switching). */
export function attackForm(page: Page): Locator {
  return page.getByRole("form", { name: "Attack" });
}

/** Open the Agir Weapons tab and wait for the weapon in hand (English UI). */
export async function openWeapons(page: Page) {
  await page.getByRole("tab", { name: "Weapons" }).click();
  await expect(attackForm(page).getByTestId("weapon-in-hand")).not.toBeEmpty();
}

/**
 * Put a weapon in hand unless it already is: the previous weapon's button when it is that one, else
 * the drop-down of the other weapons; never attacks.
 */
export async function chooseWeapon(page: Page, name: string) {
  const held = attackForm(page).getByTestId("weapon-in-hand");
  await expect(held).not.toBeEmpty();
  if ((await held.textContent()) === name) return;
  const group = attackForm(page).getByRole("group", { name: "Other weapons" });
  const previous = group.getByRole("button", {
    name: `Switch to ${name}`,
    exact: true,
  });
  if (await previous.count()) await previous.click();
  else
    await group
      .getByRole("combobox", { name: "Other weapon" })
      .selectOption({ label: name });
  await expect(held).toHaveText(name);
}

/** Show the target bar's situational controls (ADR 0027 step 7.2) unless shown. */
export async function openTargetModifiers(page: Page) {
  const toggle = attackForm(page).getByRole("button", {
    name: "Situational modifier",
  });
  if ((await toggle.getAttribute("aria-expanded")) !== "true")
    await toggle.click();
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
}

/** The ammunition chips of the weapon in hand. */
export function ammoChips(page: Page): Locator {
  return attackForm(page).getByRole("radiogroup", { name: "Ammunition" });
}

/** Choose the ammunition called `name` ("Arrows +3") and wait until it is the choice. */
export async function chooseAmmo(page: Page, name: string) {
  const chip = ammoChips(page).getByRole("radio", { name: `${name},` });
  await chip.click();
  await expect(chip).toHaveAttribute("aria-checked", "true");
}

/** Count left of the chosen ammunition, as its chip shows it. */
export async function chosenAmmoCount(page: Page): Promise<number> {
  const text = await ammoChips(page)
    .locator('[aria-checked="true"]')
    .getByTestId("ammo-count")
    .textContent();
  if (!/^\d+$/.test(text ?? ""))
    throw new Error(`unexpected ammo count: ${JSON.stringify(text)}`);
  return Number(text);
}

/**
 * Send every attack with target AC `ac`, beyond the form's ±20 (AC 1000 needs so low a roll that
 * every attack crits). The form still shows its own AC.
 */
export async function forceAttackAc(page: Page, ac: number) {
  await page.route("**/api/roll/attack", async (route) => {
    const body = route.request().postDataJSON() as Record<string, unknown>;
    await route.continue({
      postData: JSON.stringify({ ...body, target_ac: ac }),
    });
  });
}

/**
 * Opens the result card's one-line correction fold whose line reads `line` ("Adjust after the
 * roll", "Correct the target AC", "Give the real AC") if it is closed, and returns its form.
 */
export async function openFold(
  scope: Locator,
  line: string,
  form: string = line,
): Promise<Locator> {
  const fold = scope
    .locator("details")
    .filter({ has: scope.page().locator("summary", { hasText: line }) });
  if (!(await fold.evaluate((d) => (d as HTMLDetailsElement).open)))
    await fold.locator("summary").click();
  const found = scope.getByRole("form", { name: form });
  await expect(found).toBeVisible();
  return found;
}

/**
 * A spell level's slot dots on the Spells tab: `left` available then `total - left` used, then any
 * number of empty (unprepared) dots.
 */
export async function expectSlotDots(
  region: Locator,
  level: number,
  left: number,
  total: number,
) {
  await expect(region.getByTestId(`dots-${level}`)).toHaveAttribute(
    "data-state",
    new RegExp(`^a{${left}}u{${total - left}}e*$`),
  );
}

/** A character to create through the creation route, French labels (the page is in French). */
export interface FlowCharacter {
  name: string;
  /** The class and race cards' names, e.g. "Guerrier", "Humain". */
  className: string;
  race: string;
  /** The six scores by their short label (FOR, DEX ...), typed. */
  scores: Record<string, string>;
  hpMax: string;
  /** Typed as XP; the starting level 1 when absent. */
  xp?: string;
  /** Fills the class extras on the "Points de vie et talents" step. */
  extras?: (step: Locator) => Promise<void>;
  /** Ticks the starting spells on a mage's or a bard's "Grimoire" step (left as offered when
   * absent). */
  spellbook?: (step: Locator) => Promise<void>;
}

/**
 * Creates a character through the creation route from the cog menu (ADR 0034 step 2), typing every
 * value, then picks the first alignment and sex offered, and Créer: the page loads on it.
 */
/** Open the roster, "Vos personnages", from the character card's header (ADR 0034 step 7.1). */
export async function openRoster(page: Page) {
  await page.getByTestId("roster-open").click();
  const roster = page.getByTestId("roster");
  await expect(roster.getByTestId("roster-card").first()).toBeVisible();
  return roster;
}

export async function createInFlow(page: Page, c: FlowCharacter) {
  const roster = await openRoster(page);
  await roster
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  const step = page.getByTestId("creation-step");
  const next = () =>
    step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  await step.getByRole("radio", { name: c.className, exact: true }).click();
  if (c.xp !== undefined) {
    await step.getByRole("radio", { name: "XP", exact: true }).click();
    await step.getByLabel("XP de départ").fill(c.xp);
  }
  await next();
  await step.getByRole("radio", { name: c.race, exact: true }).click();
  await next();
  await step.getByRole("radio", { name: "Saisir" }).click();
  for (const [ability, score] of Object.entries(c.scores))
    await step.getByLabel(ability, { exact: true }).fill(score);
  await next();
  await step.getByLabel("PV max").fill(c.hpMax);
  if (c.extras) await c.extras(step);
  await next();
  // The proficiency step (ADR 0037 step 14): nothing picked, the slots left for later.
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await next();
  // A mage's or a bard's spellbook step (ADR 0030 step 20.7): the spells as offered, or ticked.
  await expect(step).not.toHaveAttribute("data-step", "proficiencies");
  if ((await step.getAttribute("data-step")) === "spellbook") {
    if (c.spellbook) await c.spellbook(step);
    await next();
  }
  await step.getByLabel("Nom", { exact: true }).fill(c.name);
  // The first alignment the class admits (a refused one is labelled "… : interdit au …").
  await step
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: /^[^:]+$/ })
    .first()
    .click();
  await step
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio")
    .first()
    .click();
  await next();
  await step
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();
  await expect(page.getByTestId("character")).toContainText(c.name);
}

/**
 * Ajouter > Saisir (ADR 0035 item 10): opens Add in the Inventaire and its typed tab, the homebrew
 * form (Catalogue is the first tab).
 */
export async function openTypedAdd(
  scope: Page | Locator,
  lang: "en" | "fr" = "en",
): Promise<void> {
  await scope
    .getByRole("button", {
      name: lang === "fr" ? "Ajouter un objet" : "Add item",
    })
    .click();
  await scope
    .getByRole("tab", { name: lang === "fr" ? "Saisir" : "Type in" })
    .click();
}

/**
 * Attacks the mind `target` from the Powers tab with Mind Thrust and expects a tangent row to be kept
 * up (the Maintained card shows). The e2e dice are seeded, so the outcome of the attack is fixed.
 */
export async function keepATangent(page: Page, tab: Locator, target: string) {
  await tab.getByTestId("target-name").fill(target);
  const attack = /^(Attack with|Attaquer avec) (Mind Thrust|Pression mentale)$/;
  await tab.getByRole("button", { name: attack }).first().click();
  const bar = page.getByTestId("action-bar");
  // The e2e dice are on a fixed seed: this first Mind Thrust leaves a tangent, no retry loop.
  await bar.getByRole("button", { name: attack }).click();
  await apiSettled(page);
  await expect(page.getByTestId("maintained-card")).toBeVisible();
}

/** Sorts > Pouvoirs: browse the powers catalogue and the known powers (a psionicist's). */
export async function openSortsPowers(page: Page) {
  await page
    .getByTestId("rail")
    .getByRole("button", { name: /^(Spells|Sorts)$/ })
    .click();
  await page.getByRole("tab", { name: /^(Powers|Pouvoirs)$/ }).click();
  return page.getByTestId("known-powers");
}

/** Act > Powers: the tab where the known powers are played. */
export async function openActPowers(page: Page) {
  await page
    .getByTestId("rail")
    .getByRole("button", { name: /^(Act|Agir)$/ })
    .click();
  await page.getByRole("tab", { name: /^(Powers|Pouvoirs)$/ }).click();
  return page.getByTestId("powers-tab");
}
