// Direction A2, "Timeline + centre stage": the vitals bar is a thin command row (turn, initiative,
// Next turn, HP, last result) over the effects timeline at full width with twelve turn columns
// (the v3 mockup's clock). In Act the action and its result share the centre column, cause over
// effect; the character is a narrow column on the left and the journal a full-height column on
// the right. The lookup detail is the hero: stat tiles, a cast box and the description in two
// text columns. XP is the mockup's workspace: progress and form across the top, ledger below.
import * as d from "./data.mjs";
import * as ui from "./ui.mjs";

export const id = "a2-centre-stage";
export const name = "A2 · Timeline + centre stage";
export const logo = "facet-bare";
export const css = "direction.css";

const RAIL = [
  ["act", "Act", "fist"],
  ["spells", "Spells", "book"],
  ["xp", "Experience", "trend"],
  ["inventory", "Inventory", "bag"],
];

const vitals = () => `<header class="vitals">
  <div class="cmd">
    <div class="v-brand">${ui.brand(logo)}</div>
    <div class="v-turn"><span class="label">Turn</span><span class="hero">${d.turn.number}</span></div>
    <div class="v-init"><span class="hstack"><span class="muted">Initiative</span><span class="tag">${d.turn.initiative}</span><span class="btn ghost xs icon">${ui.icon("reroll")}</span></span><span class="hstack"><span class="btn primary">Next turn ${ui.icon("arrow")}</span><span class="btn icon">${ui.icon("dots")}</span></span></div>
    <div class="v-hp">${ui.hpBlock("headline")}</div>
    <div class="v-result"><span class="label">Last result · ${d.lastResult.when}</span>${ui.resultLine()}</div>
  </div>
  <div class="timeline">${ui.effectsTrack(12)}</div>
</header>`;

const shell = (active, stage) => `<div class="app">
${vitals()}
<div class="below">
  ${ui.rail(active, RAIL)}
  <main class="stage stage-${active}">${stage}</main>
</div>
</div>`;

const character = () =>
  `<aside class="panel moi stack"><div><div class="title">${ui.esc(d.character.name)}</div><div class="muted">${d.character.klass} · Level ${d.character.level}</div></div>${ui.abilities()}<div><div class="label" style="margin-bottom:.375rem">HP change</div>${ui.hpChange()}</div>${ui.chargeCard()}<div><div class="label" style="margin-bottom:.375rem">Saving throws · click to roll</div>${ui.savesButtons()}</div></aside>`;

const act = (tab) =>
  shell(
    "act",
    `${character()}
<div class="centre stack">
  <section class="panel act"><div class="panel-head">${ui.actTabs(tab)}</div><div class="scroll-y grow">${ui.actBody(tab, { columns: 2 })}</div></section>
  <section class="panel result-panel"><div class="panel-head"><span class="label">Result</span><span class="right tag outline">Latest</span></div>${ui.resultCard()}</section>
</div>
<section class="panel fil stack">${ui.journalHead()}<div class="scroll-y grow">${ui.journalList()}</div><div class="hair"></div>${ui.diceComposer()}</section>`,
  );

const lookup = () =>
  shell(
    "spells",
    `<section class="panel lookup-side stack"><div class="panel-head"><span class="title">Spell lookup</span></div>${ui.lookupFilter()}<div class="hair"></div><div class="scroll-y grow">${ui.lookupList()}</div></section>
<section class="panel lookup-detail stack" style="gap:.75rem"><div>${ui.lookupHead()}${ui.lookupTiles()}</div>${ui.castBox()}<div class="scroll-y grow prose-cols">${ui.lookupProse()}</div></section>`,
  );

const xp = () =>
  shell(
    "xp",
    `<section class="panel xp-top"><div class="xp-grid"><div><div class="panel-head"><span class="title">Experience</span><span class="muted">${d.character.klass} · Level ${d.character.level}</span></div>${ui.xpProgress()}</div><div class="xp-form"><div class="panel-head"><span class="title">Add XP</span></div>${ui.xpForm()}<p class="muted" style="margin-top:.75rem">+10 %: the Bard's prime-requisite bonus (DEX 17 and CHA 16, both 16 or more).</p></div></div></section>
<section class="panel xp-ledger stack"><div class="panel-head"><span class="title">Ledger</span><span class="right muted">4 entries</span></div><div class="scroll-y grow">${ui.xpLedger()}</div></section>`,
  );

export const pages = {
  "act-weapons": () => act("weapons"),
  "act-spells": () => act("spells"),
  lookup,
  xp,
};

export const stylesheet = `/* Direction A2: a thin command row, the effects timeline at full width, act + result in the centre. */
.app { height: 100vh; display: flex; flex-direction: column; }
.vitals { border-bottom: 1px solid var(--border); background: var(--surface); }
.cmd { display: grid; grid-template-columns: auto auto auto 16rem 1fr; align-items: stretch; }
.cmd > div { padding: .5rem 1rem; border-right: 1px solid var(--border); min-width: 0; display: flex; align-items: center; gap: .75rem; }
.cmd > div:last-child { border-right: 0; }
@media (max-width: 1500px) { .cmd > div { padding: .5rem .75rem; } .v-brand .wordmark { display: none; } }
.v-turn { gap: .5rem !important; }
.v-turn .hero { font-size: 2rem; line-height: 2.25rem; }
.v-init { flex-direction: column; align-items: flex-start !important; gap: .25rem !important; }
.v-hp { flex-direction: column; align-items: stretch !important; justify-content: center; }
.v-hp .hp-block .headline { font-size: 1.25rem; line-height: 1.5rem; }
.v-result { flex-direction: column; align-items: flex-start !important; justify-content: center; gap: .25rem !important; }
.timeline { padding: .375rem 1rem .5rem; border-top: 1px solid var(--border); }
.effects-track { width: 100%; display: grid; row-gap: .125rem; }
.trow { display: grid; grid-template-columns: 11rem 1fr; align-items: center; min-height: 1.5rem; }
.ttrack { display: grid; align-items: center; min-height: 1.5rem; }
.tcol { text-align: center; font-size: .6875rem; color: var(--ink-muted); }
.tcol.now { color: var(--tint); font-weight: 600; background: var(--amb-900); border-radius: .25rem; }
.tcol.past { opacity: .4; }
.tline { height: 2px; background: var(--neu-700); align-self: center; }
.pill.fade { mask-image: linear-gradient(90deg, #000 70%, transparent); }
.below { flex: 1; display: flex; min-height: 0; }
.rail { width: 4.25rem; display: flex; flex-direction: column; align-items: stretch; padding: .5rem 0; border-right: 1px solid var(--border); background: var(--surface); }
.rail-item { display: flex; flex-direction: column; align-items: center; gap: .25rem; padding: .625rem 0; font-size: .6875rem; color: var(--ink-muted); border-left: 2px solid transparent; }
.rail-item.on { color: var(--tint); border-left-color: var(--tint); background: var(--amb-900); }
.stage { flex: 1; min-width: 0; min-height: 0; display: grid; grid-auto-rows: minmax(0, 1fr); gap: .75rem; padding: .75rem; }
.stage-act { grid-template-columns: 17.5rem minmax(30rem, 1fr) minmax(22rem, 26rem); }
.centre { min-height: 0; }
.centre .panel.act { flex: 0 1 auto; }
.panel.act { display: flex; flex-direction: column; }
.result-panel { border-color: var(--amb-700); flex-shrink: 0; }
.weapon-card { margin-top: .75rem; border: 1px solid var(--amb-700); border-radius: .625rem; padding: 1rem; background: linear-gradient(135deg, color-mix(in oklab, var(--tint) 10%, transparent), transparent 60%); }
.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; margin-top: .75rem; }
.tile { display: flex; flex-direction: column; gap: .125rem; padding: .5rem .625rem; border-radius: .375rem; background: var(--surface); border: 1px solid var(--border); }
.tile.on { border-color: var(--tint); }
.tile.dim { opacity: .55; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1.5rem; }
.saves-btns { display: grid; gap: .25rem; }
.save-btn { display: flex; align-items: center; gap: .5rem; height: 2rem; padding: 0 .625rem; border: 1px solid var(--border); border-radius: .375rem; }
.stage-spells { grid-template-columns: minmax(18rem, 22rem) 1fr; }
.stat-tiles { grid-template-columns: repeat(5, 1fr); }
.prose-cols .prose { columns: 2; column-gap: 2rem; max-width: 140ch; }
.prose-cols .prose p { break-inside: avoid; }
.row28.sel { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.stage-xp { grid-template-rows: auto minmax(0, 1fr); }
.xp-grid { display: grid; grid-template-columns: 1fr 24rem; gap: 2rem; }
.list-row.fresh { background: color-mix(in oklab, var(--tint) 8%, transparent); }
`;
