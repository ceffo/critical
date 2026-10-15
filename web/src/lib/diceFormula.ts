// Text edits of the composer's formula: add or remove a die, step the constant. The composer only
// builds the string. It rolls and evaluates nothing, and shares no rule with the server (limits,
// totals, validity): the server parses what is sent and answers the roll or the error.

/** The faces of the composer's dice; 100 is written d%. */
export const DICE_SIDES = [4, 6, 8, 10, 12, 20, 100] as const;

/** The die's name in a formula: "d6", "d%". */
export function dieName(sides: number): string {
  return sides === 100 ? "d%" : `d${sides}`;
}

interface Term {
  sign: "+" | "-";
  body: string;
}

/** Splits at + and − (spaces dropped); null when the text has anything else a split cannot read. */
function terms(formula: string): Term[] | null {
  const s = formula.replace(/\s+/g, "").toLowerCase();
  const out: Term[] = [];
  const re = /([+-]?)([^+-]+)/y;
  let pos = 0;
  while (pos < s.length) {
    re.lastIndex = pos;
    const m = re.exec(s);
    if (!m) return null;
    out.push({ sign: m[1] === "-" ? "-" : "+", body: m[2] });
    pos = re.lastIndex;
  }
  return out;
}

function write(ts: Term[]): string {
  return ts.map((t, i) => (i === 0 && t.sign === "+" ? "" : t.sign) + t.body).join("");
}

const dieTerm = (sides: number) => new RegExp(`^(\\d*)d(${sides === 100 ? "%|100" : sides})$`);

function countOfTerm(t: Term, sides: number): number | null {
  if (t.sign !== "+") return null;
  const m = dieTerm(sides).exec(t.body);
  return m ? (m[1] === "" ? 1 : Number(m[1])) : null;
}

/** How many dice of this size the text adds (the badge on the die button). */
export function dieCount(formula: string, sides: number): number {
  const ts = terms(formula);
  if (!ts) return 0;
  return ts.reduce((n, t) => n + (countOfTerm(t, sides) ?? 0), 0);
}

const termBody = (count: number, sides: number) => `${count > 1 ? count : ""}${dieName(sides)}`;

/** One more die of this size. */
export function addDie(formula: string, sides: number): string {
  const ts = terms(formula);
  if (!ts) return `${formula.trim()}+${dieName(sides)}`;
  const i = ts.findIndex((t) => countOfTerm(t, sides) !== null);
  if (i >= 0) ts[i] = { sign: "+", body: termBody((countOfTerm(ts[i], sides) ?? 0) + 1, sides) };
  else {
    // Dice go before the trailing constant.
    const at = ts.length > 0 && /^\d+$/.test(ts[ts.length - 1].body) ? ts.length - 1 : ts.length;
    ts.splice(at, 0, { sign: "+", body: dieName(sides) });
  }
  return write(ts);
}

/** One die of this size fewer (none left: the text is unchanged). */
export function removeDie(formula: string, sides: number): string {
  const ts = terms(formula);
  if (!ts) return formula;
  const i = ts.findIndex((t) => countOfTerm(t, sides) !== null);
  if (i < 0) return formula;
  const left = (countOfTerm(ts[i], sides) ?? 0) - 1;
  if (left <= 0) ts.splice(i, 1);
  else ts[i] = { sign: "+", body: termBody(left, sides) };
  return write(ts);
}

/** The trailing constant moved by delta (+1 / −1); it disappears at 0. */
export function stepConstant(formula: string, delta: number): string {
  const ts = terms(formula) ?? [];
  const last = ts[ts.length - 1];
  if (last && /^\d+$/.test(last.body)) {
    const value = (last.sign === "-" ? -1 : 1) * Number(last.body) + delta;
    ts.pop();
    if (value !== 0) ts.push({ sign: value < 0 ? "-" : "+", body: String(Math.abs(value)) });
  } else {
    ts.push({ sign: delta < 0 ? "-" : "+", body: String(Math.abs(delta)) });
  }
  return write(ts);
}
