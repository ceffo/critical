import { afterEach, describe, expect, it } from "vitest";
import { waitFor } from "@testing-library/react";
import { keepLocked, lockWrites, LOCKED } from "./readOnly";

afterEach(() => {
  document.body.innerHTML = "";
});

const page = (html: string) => {
  document.body.innerHTML = html;
  return document.body;
};

describe("lockWrites", () => {
  it("locks every control that is not for moving around", () => {
    const root = page(`
      <button id="b">Damage</button>
      <input id="i" />
      <select id="s"></select>
      <textarea id="t"></textarea>
      <div id="r" role="switch"></div>
      <div id="m" role="menuitem"></div>
    `);
    expect(lockWrites(root)).toBe(6);
    for (const id of ["b", "i", "s", "t"]) {
      const el = root.querySelector<HTMLButtonElement>(`#${id}`)!;
      expect(el.disabled, id).toBe(true);
      expect(el.hasAttribute(LOCKED), id).toBe(true);
    }
    const r = root.querySelector("#r")!;
    expect(r.getAttribute("aria-disabled")).toBe("true");
    expect(r.getAttribute("tabindex")).toBe("-1");
  });

  it("leaves tabs, the split handle, disclosures, the rail and marked controls usable", () => {
    const root = page(`
      <button role="tab" id="tab">Weapons</button>
      <div role="separator" id="sep"></div>
      <details><summary id="sum">More</summary></details>
      <nav><button id="rail">Act</button></nav>
      <div data-view-ok><button id="in">Filter</button></div>
      <button data-view-ok id="mark">Cog</button>
      <button id="write">Roll</button>
    `);
    expect(lockWrites(root)).toBe(1);
    for (const id of ["tab", "rail", "in", "mark"])
      expect(
        root.querySelector<HTMLButtonElement>(`#${id}`)!.disabled,
        id,
      ).toBe(false);
    expect(root.querySelector<HTMLButtonElement>("#write")!.disabled).toBe(
      true,
    );
  });
});

describe("keepLocked", () => {
  it("locks a control rendered later and one whose disabled a render cleared", async () => {
    const root = page(`<button id="a">A</button>`);
    const stop = keepLocked(document);
    expect(root.querySelector<HTMLButtonElement>("#a")!.disabled).toBe(true);

    const form = document.createElement("form");
    form.innerHTML = `<input id="late" /><button id="send">Save</button>`;
    root.append(form);
    await waitFor(() => {
      expect(root.querySelector<HTMLInputElement>("#late")!.disabled).toBe(
        true,
      );
      expect(root.querySelector<HTMLButtonElement>("#send")!.disabled).toBe(
        true,
      );
    });

    const a = root.querySelector<HTMLButtonElement>("#a")!;
    a.disabled = false;
    await waitFor(() => expect(a.disabled).toBe(true));
    stop();
  });
});
