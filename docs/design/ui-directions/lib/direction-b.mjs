// Direction B, "Anchor column + focus follows touch": no rail and no modes. The character is a
// fixed column on the left (HP, turn, effects, abilities, saves); the three activities are live
// panes side by side and the one you touch widens while the other two fall back to a compact
// summary stacked beside it; the result, the journal and the dice are one band across the bottom.
import * as d from "./data.mjs";
import * as ui from "./ui.mjs";

export const id = "b-focus-follows-touch";
export const name = "B · Anchor column + focus follows touch";
export const logo = "monogram";
export const css = "direction.css";

const anchor = () => `<aside class="anchor">
  <div class="a-brand">${ui.brand(logo)}</div>
  <div class="a-id"><div class="title">${ui.esc(d.character.name)}</div><div class="muted">${d.character.klass} · Level ${d.character.level}</div></div>
  <div class="a-hp">${ui.hpBlock("hero")}</div>
  <div class="a-turn">${ui.turnBlock("headline")}</div>
  <div class="a-effects"><div class="hstack" style="margin-bottom:.25rem"><span class="label">Effects</span><span class="right btn ghost xs">${ui.icon("plus")} Add</span></div>${ui.effectsList()}<div class="hstack" style="margin-top:.375rem;padding:0 .5rem"><span class="muted">Stoneskin · a hit taken is absorbed</span><span class="right btn xs outline-tint">Absorb</span></div></div>
  <div class="a-abilities">${ui.abilities()}</div>
  <div class="a-hpchange"><div class="label" style="margin-bottom:.375rem">HP change</div>${ui.hpChange()}</div>
  <div class="a-saves"><div class="label" style="margin-bottom:.25rem">Saving throws</div>${ui.saves()}</div>
</aside>`;

const paneHead = (title, focused, hint) =>
  `<div class="pane-head"><span class="title">${title}</span>${focused ? '<span class="tag">focused</span>' : `<span class="muted label" style="text-transform:none;letter-spacing:0">${hint}</span>`}</div>`;

const actPane = (focus, tab) =>
  focus
    ? `<section class="pane panel focus">${paneHead("Act", true)}${ui.actTabs(tab)}<div class="scroll-y grow" style="margin-top:.5rem">${ui.actBody(tab, { columns: 2 })}</div></section>`
    : `<section class="pane panel compact">${paneHead("Act", false, "touch to widen")}
  <div class="label">Weapon in hand</div><div class="key">${ui.esc(d.weapons.inHand.name)}</div><div class="muted">AC ${d.weapons.target.ac} · S/M · needs 6</div>
  <span class="btn primary" style="margin-top:.5rem;width:100%">${ui.icon("sword")} Attack</span>
  <div class="hair" style="margin:.75rem 0"></div>
  <div class="row28" style="grid-template-columns:1fr auto"><span>Spells</span><span class="muted">12 left</span></div>
  <div class="row28" style="grid-template-columns:1fr auto"><span>Items</span><span class="muted">3</span></div>
  <div class="row28" style="grid-template-columns:1fr auto"><span>Skills</span><span class="muted">8</span></div>
</section>`;

const lookupPane = (focus) =>
  focus
    ? `<section class="pane panel focus lookup">${paneHead("Spell lookup", true)}${ui.lookupFilter()}<div class="lookup-body"><div class="scroll-y">${ui.lookupList()}</div><div class="scroll-y detail">${ui.lookupDetail()}</div></div></section>`
    : `<section class="pane panel compact">${paneHead("Spell lookup", false, "type to widen")}
  <span class="field" style="display:flex;width:100%">${ui.icon("search")}&nbsp;${d.lookup.query}</span>
  <div class="muted label" style="margin:.5rem 0 .25rem;text-transform:none;letter-spacing:0">${d.lookup.count} spells · Wizard</div>
  <div class="scroll-y grow">${ui.lookupList()}</div>
</section>`;

const xpPane = (focus) =>
  focus
    ? `<section class="pane panel focus xp">${paneHead("Experience", true)}${ui.xpProgress()}<div class="hair" style="margin:.75rem 0"></div><div class="label" style="margin-bottom:.5rem">Add XP</div>${ui.xpForm()}<div class="hair" style="margin:.75rem 0"></div><div class="hstack" style="margin-bottom:.25rem"><span class="label">Ledger</span><span class="right muted">4 entries</span></div><div class="scroll-y grow">${ui.xpLedger()}</div></section>`
    : `<section class="pane panel compact">${paneHead("Experience", false, "touch to widen")}
  <div class="headline">${d.xp.total}</div><div class="muted">XP · level ${d.character.level}</div>
  <div class="xpbar" style="margin-top:.5rem"><i class="fill" style="width:${d.xp.pct}%"></i></div>
  <div class="highlight" style="margin-top:.375rem">Before level ${d.xp.nextLevel} · ${d.xp.toGo} XP</div>
  <span class="btn" style="margin-top:.75rem">${ui.icon("plus")} Add XP</span>
</section>`;

const band = () => `<footer class="band">
  <div class="panel b-result">${ui.resultCard({ compact: false })}</div>
  <div class="panel b-journal">${ui.journalHead()}<div class="scroll-y grow" style="margin-top:.25rem">${ui.journalList()}</div></div>
  <div class="panel b-dice"><div class="label" style="margin-bottom:.5rem">Dice</div>${ui.diceComposer()}</div>
</footer>`;

const shell = (focus, tab = "weapons") => `<div class="app">
${anchor()}
<main class="canvas">${[["act", () => actPane(focus === "act", tab)], ["lookup", () => lookupPane(focus === "lookup")], ["xp", () => xpPane(focus === "xp")]]
  .sort((a, b) => (a[0] === focus ? -1 : b[0] === focus ? 1 : 0))
  .map(([, r]) => r())
  .join("")}</main>
${band()}
</div>`;

export const pages = {
  "act-weapons": () => shell("act", "weapons"),
  "act-spells": () => shell("act", "spells"),
  lookup: () => shell("lookup"),
  xp: () => shell("xp"),
};

export const stylesheet = `/* Direction B: anchor column, three live panes, a result band. */
.app { height: 100vh; display: grid; grid-template-columns: 19.5rem 1fr; grid-template-rows: 1fr 15.5rem; }
@media (min-height: 1100px) { .app { grid-template-rows: 1fr 21rem; } }
.anchor { grid-row: 1 / 3; background: var(--surface); border-right: 1px solid var(--border); padding: .75rem 1rem; display: flex; flex-direction: column; gap: .625rem; overflow-y: auto; min-height: 0; }
.anchor > div { padding-top: .625rem; border-top: 1px solid var(--border); }
.anchor > .a-brand, .anchor > .a-id { border-top: 0; padding-top: 0; }
.a-hp .hero { color: var(--ink); }
.turn-block { display: grid; grid-template-columns: auto 1fr; column-gap: .75rem; row-gap: .375rem; align-items: center; }
.turn-num { display: flex; align-items: baseline; gap: .375rem; }
.turn-init { grid-column: 2; }
.turn-actions { grid-column: 1 / 3; }
.turn-actions .btn.primary { flex: 1; }
.effects-list .row28 { background: none; }
.saves .row28 { min-height: 1.625rem; }
.canvas { display: grid; grid-template-columns: 1fr 16rem; grid-template-rows: 1fr 1fr; gap: .75rem; padding: .75rem; min-height: 0; min-width: 0; }
.pane { display: flex; flex-direction: column; min-height: 0; }
.pane.focus { grid-column: 1; grid-row: 1 / 3; border-color: var(--tint); box-shadow: inset 0 2px 0 var(--tint); }
.pane.compact { grid-column: 2; }
.pane-head { display: flex; align-items: center; gap: .5rem; margin-bottom: .5rem; }
.pane-head .tag { margin-left: auto; }
.pane-head .muted { margin-left: auto; }
.lookup-body { display: grid; grid-template-columns: 20rem 1fr; gap: 1rem; min-height: 0; flex: 1; margin-top: .5rem; }
.lookup-body .detail { padding-left: 1rem; border-left: 1px solid var(--border); }
.lookup-body .prose { max-width: 70ch; }
.row28.sel { background: var(--amb-900); box-shadow: inset 2px 0 0 var(--tint); }
.weapon-card { margin-top: .75rem; border: 1px solid var(--amb-700); border-radius: .625rem; padding: 1rem; background: linear-gradient(135deg, color-mix(in oklab, var(--tint) 10%, transparent), transparent 60%); }
.tiles { display: grid; grid-template-columns: repeat(4, 1fr); gap: .5rem; margin-top: .75rem; }
.tile { display: flex; flex-direction: column; gap: .125rem; padding: .5rem .625rem; border-radius: .375rem; background: var(--surface); border: 1px solid var(--border); }
.tile.on { border-color: var(--tint); }
.tile.dim { opacity: .55; }
.two-col { display: grid; grid-template-columns: 1fr 1fr; column-gap: 1.5rem; }
.list-row.fresh { background: color-mix(in oklab, var(--tint) 8%, transparent); }
.band { grid-column: 2; display: grid; grid-template-columns: 27rem 1fr 21rem; gap: .75rem; padding: 0 .75rem .75rem; min-height: 0; }
.band > .panel { display: flex; flex-direction: column; min-height: 0; }
.b-result .result .headline { font-size: 1.25rem; line-height: 1.5rem; }
`;
