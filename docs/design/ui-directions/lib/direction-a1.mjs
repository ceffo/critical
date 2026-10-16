// Direction A1, "Character strip": A's vitals bar grows a second row that carries the whole
// character (abilities, HP change, the Stoneskin charges, the five saves as buttons), so the
// character column disappears and the stage below is two wide columns: the activity and the
// result + journal. The lookup is a faceted browser; XP is progress + form beside the ledger.
import * as d from "./data.mjs";
import * as ui from "./ui.mjs";

export const id = "a1-character-strip";
export const name = "A1 · Character strip";
export const logo = "facet-spine";
export const css = "direction.css";

const RAIL = [
  ["act", "Act", "fist"],
  ["spells", "Spells", "book"],
  ["xp", "Experience", "trend"],
  ["inventory", "Inventory", "bag"],
];

const vitals = () => `<header class="vitals">
  <div class="row1">
    <div class="v-brand"><div>${ui.brand(logo)}<div class="muted" style="margin-top:.25rem">${ui.esc(d.character.name)} · ${d.character.klass} ${d.character.level}</div></div></div>
    <div class="v-turn">${ui.turnBlock("hero")}</div>
    <div class="v-effects">${ui.effectsTrack(8, { absorb: true })}</div>
    <div class="v-hp">${ui.hpBlock("headline")}</div>
    <div class="v-result"><div class="hstack"><span class="label">Last result</span><span class="muted label" style="text-transform:none;letter-spacing:0">${d.lastResult.when}</span></div>${ui.resultLine()}<div class="muted" style="margin-top:.25rem">${ui.esc(d.lastResult.subject)} vs AC ${d.lastResult.targetAc} · ${d.lastResult.damageDie} → ${d.lastResult.damageRoll} + ${d.lastResult.damageBonus}</div></div>
  </div>
  <div class="row2">
    <div class="v-abil">${ui.abilitiesInline()}</div>
    <div class="v-hpchange"><span class="label">HP change</span>${ui.hpChangeInline()}</div>
    <div class="v-saves"><span class="label">Saves · click to roll</span>${ui.savesButtons({ inline: true, short: true })}</div>
  </div>
</header>`;

const shell = (active, stage) => `<div class="app">
${vitals()}
<div class="below">
  ${ui.rail(active, RAIL)}
  <main class="stage stage-${active}">${stage}</main>
</div>
</div>`;

const act = (tab) =>
  shell(
    "act",
    `<section class="panel act"><div class="panel-head"><span class="title">Act</span>${ui.actTabs(tab)}</div><div class="scroll-y grow">${ui.actBody(tab, { columns: 3 })}</div></section>
<section class="panel fil stack"><div class="panel-head"><span class="label">Result</span><span class="right tag outline">Latest</span></div>${ui.resultCard({ details: true })}<div class="hair" style="margin:.25rem 0"></div>${ui.journalHead()}<div class="scroll-y grow">${ui.journalList()}</div><div class="hair"></div>${ui.diceComposer()}</section>`,
  );

const lookup = () =>
  shell(
    "spells",
    `<section class="panel facets stack" style="gap:.5rem"><div class="panel-head" style="margin-bottom:0"><span class="title">Spell lookup</span></div>${ui.lookupFacets()}</section>
<section class="panel results stack"><div class="panel-head"><span class="label">${d.lookup.count} spells · ${d.lookup.klass} · “${d.lookup.query}”</span><span class="muted">sorted by level</span></div><div class="scroll-y grow">${ui.lookupRows()}</div></section>
<section class="panel detail stack" style="gap:.75rem"><div>${ui.lookupHead()}${ui.lookupStats()}</div>${ui.castBox()}<div class="scroll-y grow">${ui.lookupProse()}</div></section>`,
  );

const xp = () =>
  shell(
    "xp",
    `<div class="stack xp-left">
  <section class="panel"><div class="panel-head"><span class="title">Experience</span><span class="muted">${d.character.klass} · Level ${d.character.level}</span></div>${ui.xpProgress()}</section>
  <section class="panel"><div class="panel-head"><span class="title">Add XP</span></div>${ui.xpForm()}<p class="muted" style="margin-top:.75rem">+10 %: the Bard's prime-requisite bonus (DEX 17 and CHA 16, both 16 or more).</p></section>
</div>
<section class="panel xp-ledger stack"><div class="panel-head"><span class="title">Ledger</span><span class="right muted">4 entries</span></div><div class="scroll-y grow">${ui.xpLedger()}</div></section>`,
  );

export const pages = {
  "act-weapons": () => act("weapons"),
  "act-spells": () => act("spells"),
  lookup,
  xp,
};

export const stylesheet = `/* Direction A1: a two-row vitals bar that holds the whole character; two wide columns below. */
.app { height: 100vh; display: flex; flex-direction: column; }
.vitals { border-bottom: 1px solid var(--border); background: var(--surface); }
.row1 { display: grid; grid-template-columns: auto auto minmax(20rem, 1fr) 15rem 18rem; align-items: stretch; }
.row1 > div, .row2 > div { padding: .5rem 1rem; border-right: 1px solid var(--border); min-width: 0; }
.row1 > div:last-child, .row2 > div:last-child { border-right: 0; }
.row2 { display: grid; grid-template-columns: auto 1fr auto; align-items: center; border-top: 1px solid var(--border); background: color-mix(in oklab, var(--surface) 70%, var(--ground)); }
.row2 .label { margin-right: .5rem; }
.v-who { display: flex; flex-direction: column; justify-content: center; }
.v-hpchange, .v-saves { display: flex; align-items: center; gap: .25rem; }
.v-charge { display: flex; align-items: center; }
@media (max-width: 1500px) { .row1 > div, .row2 > div { padding: .5rem .75rem; } .v-brand .wordmark { display: none; } .v-hpchange .field.grow { display: none !important; } .row1 { grid-template-columns: auto auto minmax(16rem, 1fr) 13rem 17rem; } }
.v-brand { display: flex; align-items: center; }
.v-turn { display: flex; align-items: center; }
.turn-block { display: grid; grid-template-columns: auto auto; column-gap: .75rem; row-gap: .25rem; align-items: center; }
.turn-num { display: flex; flex-direction: column; grid-row: 1 / 3; }
.turn-num .label { margin-bottom: -.25rem; }
.v-effects { display: flex; align-items: center; }
.effects-track { width: 100%; display: grid; row-gap: .125rem; }
.trow { display: grid; grid-template-columns: 9rem 1fr; align-items: center; min-height: 1.5rem; }
.ttrack { display: grid; align-items: center; min-height: 1.5rem; }
.tcol { text-align: center; font-size: .6875rem; color: var(--ink-muted); }
.tcol.now { color: var(--tint); font-weight: 600; background: var(--amb-900); border-radius: .25rem; }
.tcol.past { opacity: .4; }
.tline { height: 2px; background: var(--neu-700); align-self: center; }
.pill.fade { mask-image: linear-gradient(90deg, #000 70%, transparent); }
.v-hp, .v-result { display: flex; flex-direction: column; justify-content: center; }
.abil-inline { display: flex; gap: .875rem; }
.abil-inline > span { display: flex; flex-direction: column; align-items: center; line-height: 1; }
.abil-inline .label { letter-spacing: .04em; margin: 0; }
.abil-inline .thac0 { padding: 0 .5rem; background: var(--neu-900); border-radius: .375rem; }
.saves-btns.inline { display: flex; gap: .375rem; }
.save-btn { display: inline-flex; align-items: center; gap: .5rem; height: 1.75rem; padding: 0 .625rem; border: 1px solid var(--border); border-radius: .375rem; white-space: nowrap; }
.below { flex: 1; display: flex; min-height: 0; }
.rail { width: 4.25rem; display: flex; flex-direction: column; align-items: stretch; padding: .5rem 0; border-right: 1px solid var(--border); background: var(--surface); }
.rail-item { display: flex; flex-direction: column; align-items: center; gap: .25rem; padding: .625rem 0; font-size: .6875rem; color: var(--ink-muted); border-left: 2px solid transparent; }
.rail-item.on { color: var(--tint); border-left-color: var(--tint); background: var(--amb-900); }
.stage { flex: 1; min-width: 0; min-height: 0; display: grid; grid-auto-rows: minmax(0, 1fr); gap: .75rem; padding: .75rem; }
.stage-act { grid-template-columns: minmax(30rem, 1.15fr) minmax(28rem, 1fr); }
.panel.act { display: flex; flex-direction: column; }
.weapon-card { margin-top: .75rem; border: 1px solid var(--amb-700); border-radius: .625rem; padding: 1rem; background: linear-gradient(135deg, color-mix(in oklab, var(--tint) 10%, transparent), transparent 60%); }
.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; margin-top: .75rem; }
.tile { display: flex; flex-direction: column; gap: .125rem; padding: .5rem .625rem; border-radius: .375rem; background: var(--surface); border: 1px solid var(--border); }
.tile.on { border-color: var(--tint); }
.tile.dim { opacity: .55; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1.5rem; }
.three-col { display: grid; grid-template-columns: 1fr 1fr 1fr; column-gap: 1.5rem; }
.stage-spells { grid-template-columns: 13rem minmax(22rem, 1fr) minmax(26rem, 1.2fr); }
.facet-row { display: flex; align-items: center; gap: .5rem; min-height: 1.625rem; padding: 0 .5rem; border-radius: .375rem; color: var(--ink-muted); }
.facet-row.on { background: var(--amb-900); color: var(--ink); font-weight: 500; box-shadow: inset 2px 0 0 var(--tint); }
.list-row.sel { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.detail .prose { max-width: 72ch; }
.stage-xp { grid-template-columns: 30rem 1fr; }
.list-row.fresh { background: color-mix(in oklab, var(--tint) 8%, transparent); }
`;
