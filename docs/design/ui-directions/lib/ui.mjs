// HTML fragments shared by the direction pages: the pieces every direction shows (HP, turn,
// effects, result, journal, dice, act tabs, lookup, XP) rendered from lib/data.mjs. Each
// direction composes and styles them differently; the content is identical on purpose.
import * as d from "./data.mjs";
import { logoSvg } from "./logos.mjs";

export const esc = (s) =>
  String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

// Small stroke icons (16 px) in place of the app's Phosphor set.
const paths = {
  sword: "M3 13l8-8m0 0h3v3m-3-3l2 2M4 12l-1 1 1 1 1-1",
  bow: "M4 4c5 1 7 3 8 8M4 4l8 8M4 4v2M12 12h-2",
  heart: "M8 13S2 9.5 2 6a3 3 0 0 1 6-1 3 3 0 0 1 6 1c0 3.5-6 7-6 7z",
  bolt: "M9 2L4 9h4l-1 5 5-7H8z",
  dice: "M3 3h10v10H3zM6 6h.01M10 10h.01M8 8h.01",
  shield: "M8 2l5 2v4c0 3-2 5-5 6-3-1-5-3-5-6V4z",
  sparkle: "M8 2v4M8 10v4M2 8h4M10 8h4M5 5l1 1M10 10l1 1M11 5l-1 1M6 10l-1 1",
  flask: "M6 2h4M7 2v4l-4 7h10L9 6V2",
  moon: "M13 9A5 5 0 0 1 7 3a5 5 0 1 0 6 6z",
  gem: "M3 6l3-3h4l3 3-5 7z",
  book: "M3 3h4a2 2 0 0 1 2 2v9a1 1 0 0 0-1-1H3zM13 3H9a2 2 0 0 0-2 2v9a1 1 0 0 1 1-1h5z",
  trend: "M2 12l4-4 3 3 5-6M10 5h4v4",
  bag: "M3 6h10l-1 8H4zM6 6V4a2 2 0 0 1 4 0v2",
  fist: "M4 8V5a1 1 0 0 1 2 0M6 7V4a1 1 0 0 1 2 0M8 7V4a1 1 0 0 1 2 0v4M4 8a2 2 0 0 0 2 2h4a2 2 0 0 0 2-2V7a1 1 0 0 0-2 0M4 8v3a3 3 0 0 0 3 3h2",
  gear: "M8 5a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM8 1v2M8 13v2M1 8h2M13 8h2M3 3l1.5 1.5M11.5 11.5L13 13M3 13l1.5-1.5M11.5 4.5L13 3",
  arrow: "M3 8h10M9 4l4 4-4 4",
  swap: "M3 6h9l-2-2M13 10H4l2 2",
  expand: "M9 3h4v4M13 3L9 7M7 13H3V9M3 13l4-4",
  collapse: "M13 3L9 7M9 7V4M9 7h3M3 13l4-4M7 9v3M7 9H4",
  search: "M7 3a4 4 0 1 0 0 8 4 4 0 0 0 0-8zM10 10l3 3",
  plus: "M8 3v10M3 8h10",
  check: "M3 8l3 3 7-7",
  x: "M4 4l8 8M12 4l-8 8",
  dots: "M4 8h.01M8 8h.01M12 8h.01",
  pencil: "M3 13l1-4 7-7 3 3-7 7z",
  star: "M8 2l1.8 3.8 4.2.6-3 2.9.7 4.2L8 11.5l-3.7 2 .7-4.2-3-2.9 4.2-.6z",
  reroll: "M3 8a5 5 0 0 1 9-3M13 8a5 5 0 0 1-9 3M12 2v3H9M4 14v-3h3",
};
export const icon = (name, cls = "ico") =>
  `<svg class="${cls}" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="${paths[name]}"/></svg>`;

export const brand = (logo) =>
  `<span class="brand">${logoSvg(logo)}<span class="wordmark">Critical<em>!</em></span></span>`;

export const hpTone = (cur, max) => (cur / max > 0.5 ? "" : cur / max > 0.25 ? "warn" : "low");

/** The HP bar alone: dying zone, fill, ghost of the last damage, scale labels. */
export function hpBar() {
  const { current, max, floor, lastDelta } = d.character.hp;
  const span = max - floor;
  const dying = (-floor / span) * 100;
  const fill = ((current - floor) / span) * 100;
  const ghost = ((current - lastDelta - floor) / span) * 100;
  return `<div class="hpbar"><i class="dying" style="width:${dying.toFixed(2)}%"></i><i class="ghost" style="width:${ghost.toFixed(2)}%"></i><i class="fill ${hpTone(current, max)}" style="width:${fill.toFixed(2)}%"></i></div>
<div class="hpscale"><span style="margin-left:${dying.toFixed(2)}%">0</span><span>${max}</span></div>`;
}

/** The HP block: label, last change, the value over the max and the bar. `size` is the value's role. */
export function hpBlock(size = "headline") {
  const { current, max, lastDelta, lastSource } = d.character.hp;
  return `<div class="hp-block">
  <div class="hstack between"><span class="label">Hit points</span><span class="danger body">${lastDelta} ${esc(lastSource)}</span></div>
  <div class="hstack" style="align-items:baseline;gap:.375rem"><span class="${size}">${current}</span><span class="muted body">/ ${max}</span></div>
  ${hpBar()}
</div>`;
}

export const abilities = () =>
  `<div class="abilities">${d.character.abilities
    .map(([k, v]) => `<div><div class="label">${k}</div><div class="key">${v}</div></div>`)
    .join("")}<div class="thac0"><div class="label">THAC0</div><div class="key">${d.character.thac0}</div></div></div>`;

export const saves = () =>
  `<div class="saves">${d.character.saves
    .map(
      (s) =>
        `<div class="row28" style="grid-template-columns:1fr auto 3rem"><span>${esc(s.name)}</span><span class="muted">${s.target}</span><span class="${s.last ? (s.last.ok ? "success" : "danger") : "muted"}" style="text-align:right">${s.last ? `${s.last.roll} ${s.last.ok ? "✓" : "✗"}` : "–"}</span></div>`,
    )
    .join("")}</div>`;

export const hpChange = () =>
  `<div class="hstack" style="flex-wrap:wrap;gap:.5rem"><span class="field placeholder" style="width:4.5rem">HP</span><span class="btn danger">${icon("sword")} Damage</span><span class="btn success">${icon("heart")} Healing</span></div>
<div class="field placeholder" style="margin-top:.5rem;display:flex">Source (optional): orc axe, potion…</div>`;

export const chargeCard = () =>
  `<div class="step hstack" style="padding:.5rem .75rem;border:1px solid var(--amb-700)"><span class="grow"><span class="title">Stoneskin</span><span class="sub muted" style="display:block;font-size:.6875rem;line-height:1rem">6 layers · a hit taken is absorbed</span></span><span class="btn outline-tint">Absorb</span></div>`;

/** Turn number, initiative and the Next turn button, laid out by the caller's CSS. */
export const turnBlock = (numberRole = "headline") =>
  `<div class="turn-block">
  <div class="turn-num"><span class="label">Turn</span><span class="${numberRole}">${d.turn.number}</span></div>
  <div class="turn-init hstack"><span class="muted">Initiative</span><span class="tag">${d.turn.initiative}</span><span class="muted">d10</span><span class="btn ghost xs icon">${icon("reroll")}</span></div>
  <div class="turn-actions hstack"><span class="btn primary">Next turn ${icon("arrow")}</span><span class="btn icon">${icon("dots")}</span></div>
</div>`;

/** The effects timeline: a label column and N turn columns from turn-2. */
export function effectsTrack(n = 8) {
  const start = Math.max(1, d.turn.number - 2);
  const cols = Array.from({ length: n }, (_, i) => start + i);
  const head = cols
    .map((t) => `<div class="tcol ${t === d.turn.number ? "now" : t < d.turn.number ? "past" : ""}">T${t}</div>`)
    .join("");
  const rows = d.effects
    .map((e) => {
      let track;
      if (e.kind === "timed") {
        const from = cols.indexOf(d.turn.number) + 1;
        const to = Math.min(n, from + e.left - 1);
        const fade = from + e.left - 1 > n ? "fade" : "";
        track = `<div class="tline" style="grid-column:1 / ${from}"></div><div class="pill ${fade}" style="grid-column:${from} / ${to + 1}">${e.label}</div>`;
      } else if (e.kind === "charges") {
        const from = cols.indexOf(d.turn.number) + 1;
        track = `<div class="hstack" style="grid-column:${from} / -1;gap:.5rem"><span class="pips">${"<i></i>".repeat(e.charges)}${'<i class="spent"></i>'.repeat(3)}</span><span class="muted body nowrap">${e.label}</span></div>`;
      } else {
        const from = cols.indexOf(d.turn.number) + 1;
        track = `<div class="pill dashed" style="grid-column:${from} / span 4">🔒 ${e.label}</div>`;
      }
      return `<div class="trow"><div class="tname hstack"><span class="grow">${esc(e.name)}</span><span class="btn ghost xs icon">${icon("pencil")}</span><span class="btn ghost xs icon">${icon("x")}</span></div><div class="ttrack" style="grid-template-columns:repeat(${n},1fr)">${track}</div></div>`;
    })
    .join("");
  return `<div class="effects-track" style="--cols:${n}">
  <div class="trow thead"><div class="tname hstack"><span class="label">Effects</span><span class="btn ghost xs">${icon("plus")} Add</span></div><div class="ttrack" style="grid-template-columns:repeat(${n},1fr)">${head}</div></div>
  ${rows}
</div>`;
}

/** The effects as a vertical list (name, then its pill / pips / lock). */
export const effectsList = () =>
  `<div class="effects-list">${d.effects
    .map((e) => {
      const state =
        e.kind === "timed"
          ? `<span class="pill">${e.label}</span>`
          : e.kind === "charges"
            ? `<span class="hstack" style="gap:.375rem"><span class="pips">${"<i></i>".repeat(e.charges)}</span><span class="muted label" style="text-transform:none;letter-spacing:0">until spent</span></span>`
            : `<span class="pill dashed">🔒 ${e.label}</span>`;
      return `<div class="row28" style="grid-template-columns:1fr auto"><span>${esc(e.name)}</span>${state}</div>`;
    })
    .join("")}</div>`;

/** The effects as inline pills, for a one-row status strip. */
export const effectsInline = () =>
  `<span class="hstack" style="gap:.375rem;flex-wrap:wrap">${d.effects
    .map((e) =>
      e.kind === "timed"
        ? `<span class="pill">${esc(e.name)} · ${e.label}</span>`
        : e.kind === "charges"
          ? `<span class="pill">${esc(e.name)} · ${e.charges} ◆</span>`
          : `<span class="pill dashed">🔒 ${esc(e.name)} · STR 18</span>`,
    )
    .join("")}</span>`;

/** The latest result as a full card: d20, hits AC, verdict, damage, corrections. */
export function resultCard({ compact = false, details = false } = {}) {
  const r = d.lastResult;
  const calc = r.calculation
    .map(([k, v]) => `<div class="hstack between"><span class="muted">${k}</span><span>${v}</span></div>`)
    .join("");
  return `<div class="result">
  <div class="hstack" style="gap:.5rem"><span class="tag">${r.kind}</span><span class="title">${esc(r.subject)}</span><span class="right muted label" style="text-transform:none;letter-spacing:0">${r.when}</span></div>
  <div class="hstack" style="gap:.75rem;margin-top:.625rem;align-items:flex-start"><div class="hex"><b>${r.d20}</b></div>
    <div class="grow"><div class="headline">Hits AC ${r.hitsAc}</div><div class="muted">${esc(r.subject)} vs AC ${r.targetAc} · S/M target</div></div>
    <div class="key highlight" style="padding-top:.25rem">${r.verdict.toUpperCase()}</div></div>
  <div class="hstack" style="margin-top:.75rem;gap:.5rem;align-items:flex-end"><span class="label" style="align-self:center">Damage · ${r.damageDie}</span><span class="die-chip">${r.damageRoll}</span><span class="muted">+${r.damageBonus}</span><span class="right hero highlight" style="font-size:2rem;line-height:2rem">${r.damageTotal}</span></div>
  ${
    compact
      ? ""
      : `${details ? `<div class="step" style="margin-top:.75rem;padding:.5rem .75rem;display:grid;gap:.25rem">${calc}</div>` : `<div class="muted" style="margin-top:.5rem">▸ Calculation details</div>`}
  <div class="hstack" style="margin-top:.5rem;gap:.5rem"><span class="muted">Target AC</span><span class="stepper"><span class="btn xs icon">−</span><span class="field">${r.targetAc}</span><span class="btn xs icon">+</span></span><span class="btn">Apply</span><span class="btn ghost">AC unknown again</span></div>`
  }
</div>`;
}

/** The latest result as a one-line summary (status strip / dock). */
export const resultLine = () => {
  const r = d.lastResult;
  return `<span class="hstack" style="gap:.5rem"><span class="hex" style="width:1.75rem;height:2rem"><b style="font-size:.8125rem;line-height:1rem">${r.d20}</b></span><span><span class="key">Hits AC ${r.hitsAc}</span> <span class="highlight key">${r.verdict.toUpperCase()}</span></span><span class="muted">·</span><span class="key highlight">${r.damageTotal}</span><span class="muted">dmg</span></span>`;
};

export const journalIcon = (name) => icon(name);

/** The journal, grouped by turn, newest first, one line per entry. */
export function journalList({ limit = Infinity } = {}) {
  const groups = [];
  for (const e of d.journal.slice(0, limit)) {
    const g = groups.at(-1);
    if (g && g.turn === e.turn) g.rows.push(e);
    else groups.push({ turn: e.turn, rows: [e] });
  }
  return groups
    .map(
      (g) =>
        `<div class="journal-group"><span class="label">${g.turn === 0 ? "Out of combat" : `Turn ${g.turn}`}</span>${g.turn === d.turn.number ? '<span class="tag">current</span>' : ""}</div>` +
        g.rows
          .map(
            (e) =>
              `<div class="journal-row ${e.current ? "current" : ""} ${e.reroll ? "show-hover" : ""}">${icon(e.icon)}<span class="txt">${esc(e.text)}<span class="time">${e.time}</span>${e.reroll ? `<span class="reroll">${icon("reroll", "")} Reroll</span>` : ""}</span><span class="val ${e.tone ?? ""}">${e.value ?? ""}</span></div>`,
          )
          .join(""),
    )
    .join("");
}

export const journalHead = () =>
  `<div class="hstack"><span class="title">Journal</span><span class="seg" style="height:1.5rem"><span class="on">All</span><span>Rolls</span></span><span class="right btn xs">Undo</span><span class="btn xs ghost">Redo</span></div>`;

/** The dice composer: dice, formula, favourites. */
export const diceComposer = ({ favourites = true } = {}) =>
  `<div class="dice">
  <div class="dice-row">${["d4", "d6", "d8", "d10", "d12", "d20", "d%"].map((x) => `<span class="chip ${x === "d6" ? "on" : ""}">${x}${x === "d6" ? '<span class="n">×3</span>' : ""}</span>`).join("")}</div>
  <div class="hstack" style="margin-top:.5rem;gap:.375rem"><span class="field grow" style="display:flex">${icon("dice")}&nbsp;${d.dice.formula}</span><span class="btn xs">−1</span><span class="btn xs">+1</span><span class="btn xs ghost icon">${icon("x")}</span><span class="btn outline-tint">Roll</span></div>
  ${favourites ? `<div class="favs" style="margin-top:.5rem">${d.dice.favourites.map(([n, f]) => `<span class="chip">${icon("star")} ${n} <span class="muted">${f}</span></span>`).join("")}<span class="btn ghost xs icon">${icon("star")}</span></div>` : ""}
</div>`;

/** The act tab bar with the chosen tab. */
export const actTabs = (active) =>
  `<div class="tabs"><span class="tab ${active === "weapons" ? "on" : ""}">Weapons</span><span class="tab ${active === "spells" ? "on" : ""}">Spells <span class="count">12</span></span><span class="tab ${active === "items" ? "on" : ""}">Items <span class="count">3</span></span><span class="tab ${active === "skills" ? "on" : ""}">Skills</span></div>`;

/** The Weapons tab: target bar and the weapon in hand. */
export function weaponsTab() {
  const w = d.weapons.inHand;
  const t = d.weapons.target;
  return `<div class="target step" style="padding:.625rem .75rem">
  <div class="hstack" style="gap:1rem;flex-wrap:wrap">
    <span class="label">Target</span>
    <span class="hstack"><span class="muted">AC</span><span class="stepper"><span class="btn xs icon">−</span><span class="field">${t.ac}</span><span class="btn xs icon">+</span></span><span class="btn ghost xs">Unknown</span></span>
    <span class="hstack"><span class="muted">Size</span><span class="seg"><span class="on">S / M</span><span>L</span></span></span>
    <span class="hstack"><span class="muted">Situational</span><span class="stepper"><span class="btn xs icon">−</span><span class="field">${t.situational}</span><span class="btn xs icon">+</span></span></span>
  </div>
</div>
<div class="weapon-card">
  <div class="hstack" style="align-items:flex-start"><div class="grow"><div class="label">Weapon in hand</div><div class="key" style="font-size:1.25rem;line-height:1.5rem">${esc(w.name)}</div><div class="muted">${w.kind}</div></div><span class="btn primary">${icon("sword")} Attack</span></div>
  <div class="tiles">
    <div class="tile on"><span class="label">Damage S/M</span><span class="key">${w.damageSm}</span></div>
    <div class="tile dim"><span class="label">Damage L</span><span class="key">${w.damageL}</span></div>
    <div class="tile"><span class="label">To hit / dmg</span><span class="key">${w.bonus}</span></div>
    <div class="tile"><span class="label">Effective THAC0</span><span class="key">${w.effectiveThac0}</span><span class="muted label" style="text-transform:none;letter-spacing:0">${w.needs}</span></div>
  </div>
</div>
<div class="hstack" style="gap:.5rem;margin-top:.625rem"><span class="btn">${icon("swap")} Switch to ${d.weapons.previous.name} <span class="muted">${d.weapons.previous.dice}</span></span><span class="btn ghost">Other weapon ▾</span><span class="right muted body">▸ Crit table (house rule)</span></div>`;
}

/** The Spells tab: one group per level with slot pips, 40 px rows. `columns` splits the levels. */
export function spellsTab({ columns = 1 } = {}) {
  const groups = d.spellLevels.map(
    (l) => `<div class="spell-group">
  <div class="hstack" style="margin:.5rem 0 .25rem"><span class="label magic">Level ${l.level}</span><span class="pips" style="margin-left:.25rem">${"<i></i>".repeat(l.left)}${'<i class="spent"></i>'.repeat(l.slots - l.left)}</span><span class="right muted label" style="text-transform:none;letter-spacing:0">${l.left} / ${l.slots} left</span></div>
  ${l.spells
    .map(
      (s) =>
        `<div class="list-row" style="grid-template-columns:1fr auto"><span>${esc(s.name)}${s.dice ? `<span class="sub magic">${s.dice}</span>` : ""}</span>${s.used ? '<span class="muted" style="text-decoration:line-through">Used</span>' : '<span class="btn outline-tint">Cast</span>'}</div>`,
    )
    .join("")}
</div>`,
  );
  const head = `<div class="hstack" style="gap:.75rem;flex-wrap:wrap"><span class="hstack" style="gap:.375rem"><span class="field" style="width:1rem;height:1rem;padding:0"></span><span>Buff spells go on an ally</span></span><span class="right btn">Prepare spells</span><span class="btn">${icon("moon")} Long rest</span></div>`;
  if (columns === 1) return head + groups.join("");
  const half = Math.ceil(groups.length / 2);
  return head + `<div class="two-col"><div>${groups.slice(0, half).join("")}</div><div>${groups.slice(half).join("")}</div></div>`;
}

export const itemsTab = () =>
  d.items
    .map(
      (it) =>
        `<div class="list-row" style="grid-template-columns:1fr 4rem auto"><span>${esc(it.name)}<span class="sub">${it.sub}</span></span><span class="magic key">${it.dice}</span><span class="btn outline-tint">${it.verb}</span></div>`,
    )
    .join("") + `<div class="hstack" style="margin-top:.5rem"><span class="btn ghost">Manage the inventory ${icon("arrow")}</span></div>`;

export const skillsTab = () =>
  `<div class="label" style="margin:.5rem 0 .25rem">Proficiencies · d20 ≤ value, 20 always fails</div>${d.skills.proficiencies
    .map(([n, a, t]) => `<div class="row28" style="grid-template-columns:1fr auto 3rem 3rem"><span>${n}</span><span class="muted">${a}</span><span class="key" style="text-align:right">${t}</span><span class="muted" style="text-align:right">–</span></div>`)
    .join("")}<div class="label" style="margin:.75rem 0 .25rem">Bard skills · d100 ≤ percentage</div>${d.skills.bard
    .map(([n, p]) => `<div class="row28" style="grid-template-columns:1fr 3rem 3rem"><span>${n}</span><span class="key" style="text-align:right">${p}%</span><span class="muted" style="text-align:right">–</span></div>`)
    .join("")}`;

export const actBody = (tab, opts = {}) =>
  ({ weapons: weaponsTab, spells: () => spellsTab(opts), items: itemsTab, skills: skillsTab })[tab]();

/** The lookup filter: search, school, class, levels, count. */
export const lookupFilter = ({ wrap = true } = {}) =>
  `<div class="lookup-filter" style="display:flex;gap:.5rem;flex-wrap:${wrap ? "wrap" : "nowrap"};align-items:center">
  <span class="field" style="display:flex;flex:1;min-width:12rem">${icon("search")}&nbsp;${d.lookup.query}<span class="right muted">✕</span></span>
  <span class="field placeholder" style="width:9rem">Any school ▾</span>
  <span class="seg">${["Mine", "Wizard", "Priest", "All"].map((c) => `<span class="${c === d.lookup.klass ? "on" : ""}">${c}</span>`).join("")}</span>
  <span class="hstack" style="gap:.25rem">${Array.from({ length: 10 }, (_, i) => `<span class="chip" style="padding:0 .5rem">${i}</span>`).join("")}</span>
  <span class="right muted nowrap">${d.lookup.count} spells · <span class="tint">Reset</span></span>
</div>`;

/** The lookup result list: level and name, the name wrapping when it must. */
export const lookupList = () =>
  d.lookup.results
    .map(
      (r) =>
        `<div class="row28 ${r.selected ? "sel" : ""}" style="grid-template-columns:1.25rem 1fr;min-height:2rem;padding:.25rem .5rem"><span class="magic key" style="font-size:.8125rem">${r.level}</span><span>${esc(r.name)}</span></div>`,
    )
    .join("");

/** The lookup detail: name, meta, stat rows, dice rows and the description. */
export function lookupDetail({ split = false } = {}) {
  const s = d.lookup.selected;
  const stats = `<div class="stat-rows">${s.rows.map(([k, v, raw]) => `<div class="muted">${k}</div><div><span class="key" style="font-weight:500;font-size:.875rem">${esc(v)}</span>${raw !== v ? `<span class="raw">${esc(raw)}</span>` : ""}</div>`).join("")}</div>
  <div class="muted" style="margin-top:.625rem">At level ${d.character.level} · no dice to roll</div>`;
  const prose = `<div class="label" style="margin-bottom:.375rem">Description</div><div class="prose">${s.description.map((p) => `<p>${esc(p)}</p>`).join("")}</div>`;
  const head = `<div class="title" style="font-size:1.125rem;line-height:1.5rem">${esc(s.name)}</div><div class="muted" style="margin-bottom:.75rem">${s.meta}</div>`;
  return split
    ? `${head}<div class="detail-split"><div>${stats}</div><div class="scroll-y">${prose}</div></div>`
    : `${head}${stats}<div style="margin-top:1rem">${prose}</div>`;
}

export function xpProgress({ hero = true } = {}) {
  const x = d.xp;
  return `<div class="hstack" style="align-items:baseline;gap:.5rem"><span class="${hero ? "hero" : "headline"}">${x.total}</span><span class="muted">XP</span><span class="right highlight">Before level ${x.nextLevel} · ${x.toGo} XP</span></div>
<div class="xpbar" style="margin-top:.5rem"><i class="fill" style="width:${x.pct}%"></i></div>
<div class="hstack between muted label" style="text-transform:none;letter-spacing:0;margin-top:.25rem"><span>${x.floor}</span><span>${x.pct}%</span><span>${x.ceiling}</span></div>
<div class="levels" style="margin-top:.75rem">${x.levels.map(([l, v]) => `<div class="${l === 15 ? "now" : l === 16 ? "next" : ""}"><div class="key">${l}</div><div class="label">${v}</div></div>`).join("")}</div>`;
}

export const xpForm = () =>
  `<div class="hstack" style="gap:.5rem;flex-wrap:wrap"><span class="field placeholder" style="width:6rem">Amount</span><span class="field placeholder grow" style="display:flex;min-width:10rem">Monster, quest, performance…</span><span class="field placeholder" style="width:5rem">Session</span><span class="hstack" style="gap:.375rem"><span class="field" style="width:1rem;height:1rem;padding:0;background:var(--tint)"></span>+10%</span><span class="btn primary">${icon("plus")} ${d.xp.preview}</span></div>`;

export const xpLedger = () =>
  `<div class="row28 muted label" style="grid-template-columns:1fr 5rem 4rem 5.5rem 3.5rem;background:none"><span>Reason</span><span style="text-align:right">Base</span><span style="text-align:right">Bonus</span><span style="text-align:right">Total</span><span></span></div>` +
  d.xp.ledger
    .map(
      (e) =>
        `<div class="list-row ${e.fresh ? "fresh" : ""}" style="grid-template-columns:1fr 5rem 4rem 5.5rem 3.5rem;min-height:2.25rem"><span>${esc(e.reason)}${e.meta ? `<span class="sub">${e.meta}</span>` : ""}</span><span style="text-align:right">${e.base}</span><span style="text-align:right" class="highlight">${e.bonus}</span><span class="key" style="text-align:right">${e.total}</span><span class="hstack" style="justify-content:flex-end;gap:.125rem">${e.readonly ? "" : `<span class="btn ghost xs icon">${icon("pencil")}</span><span class="btn ghost xs icon">${icon("x")}</span>`}</span></div>`,
    )
    .join("");

export const rail = (active, items) =>
  `<nav class="rail">${items.map(([id, name, ic]) => `<span class="rail-item ${id === active ? "on" : ""}">${icon(ic)}<span>${name}</span></span>`).join("")}<span class="rail-item" style="margin-top:auto">${icon("gear")}</span></nav>`;

/** A whole page: the shared head, a direction stylesheet and the body. */
export const page = ({ title, css, body, theme = "theme-dark" }) =>
  `<!doctype html>
<html lang="en" class="${theme}">
<head>
<meta charset="utf-8">
<title>${esc(title)}</title>
<link rel="stylesheet" href="../shared/tokens.css">
<link rel="stylesheet" href="../shared/base.css">
<link rel="stylesheet" href="${css}">
</head>
<body>
${body}
</body>
</html>
`;
