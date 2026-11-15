/**
 * The controls a read-only page (another user's character, ADR 0029 step 11) locks. Every control is
 * locked unless it only moves around the sheet: a tab, the journal's split handle, a disclosure, the
 * rail, or one marked `data-view-ok` (and what is inside a marked box). New controls are therefore
 * locked by default; the page's http layer refuses every write on top of that (`api/http.ts`).
 */
const CONTROLS =
  "button, input, select, textarea, [role=button], [role=switch], [role=checkbox], [role=radio], [role=menuitem], [role=menuitemradio], [role=menuitemcheckbox], [role=option], [contenteditable=true]";

/** What stays usable: moving around, reading and the device's own settings. */
const NAVIGATION =
  "[role=tab], [role=separator], summary, nav, nav *, [data-view-ok], [data-view-ok] *";

/** The attribute a locked control carries (the stylesheet dims it). */
export const LOCKED = "data-view-locked";

/** Locks every write control under root (root itself included); returns how many it locked now. */
export function lockWrites(root: ParentNode): number {
  let n = 0;
  const list: Element[] = [...root.querySelectorAll(CONTROLS)];
  if (root instanceof Element && root.matches(CONTROLS)) list.push(root);
  for (const el of list) {
    if (el.matches(NAVIGATION) || el.hasAttribute(LOCKED)) continue;
    el.setAttribute(LOCKED, "");
    el.setAttribute("aria-disabled", "true");
    if ("disabled" in el) (el as HTMLButtonElement).disabled = true;
    else el.setAttribute("tabindex", "-1");
    n++;
  }
  return n;
}

/**
 * Keeps the document locked while the page runs: React renders new controls (a form that opens, a
 * list that loads) and clears a `disabled` it set itself, so every change is looked at again.
 * Returns the stop.
 */
export function keepLocked(doc: Document): () => void {
  lockWrites(doc.body);
  const relock = (records: MutationRecord[]) => {
    for (const r of records) {
      if (r.type === "attributes") {
        const el = r.target as Element;
        if (el.hasAttribute(LOCKED) && !(el as HTMLButtonElement).disabled) {
          el.removeAttribute(LOCKED);
          lockWrites(el.parentElement ?? doc.body);
        }
        continue;
      }
      r.addedNodes.forEach((node) => {
        if (node instanceof Element) lockWrites(node);
      });
    }
  };
  const mo = new MutationObserver(relock);
  mo.observe(doc.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["disabled"],
  });
  return () => mo.disconnect();
}
