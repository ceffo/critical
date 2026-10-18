import { renderToStaticMarkup } from "react-dom/server";
import { isValidElement, type ReactElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { SizeChips } from "./SizeChips";

const props = {
  label: "Target size",
  names: { sm: "Small or medium", l: "Large" },
  shorts: { sm: "S/M", l: "L" },
};

type ElProps = { children?: ReactNode; onClick?: () => void };

/** Every button element of a rendered tree, in order. */
function buttons(node: ReactNode, out: ReactElement<ElProps>[] = []) {
  if (!isValidElement(node)) return out;
  const el = node as ReactElement<ElProps>;
  if (el.type === "button") out.push(el);
  const kids = el.props.children;
  for (const k of Array.isArray(kids) ? kids.flat() : [kids]) buttons(k, out);
  return out;
}

describe("SizeChips", () => {
  it("shows both sizes, the chosen one checked", () => {
    const html = renderToStaticMarkup(
      <SizeChips {...props} value="l" onChange={() => {}} />,
    );
    expect(html).toContain('role="radiogroup"');
    expect(html).toMatch(
      /aria-checked="false"[^>]*aria-label="Small or medium"/,
    );
    expect(html).toMatch(/aria-checked="true"[^>]*aria-label="Large"/);
    expect(html).toContain(">S/M<");
    expect(html).toContain(">L<");
  });

  it("one click on a chip chooses that size", () => {
    const onChange = vi.fn();
    const tree = SizeChips({ ...props, value: "sm", onChange });
    const [sm, l] = buttons(tree);
    l?.props.onClick?.();
    sm?.props.onClick?.();
    expect(onChange.mock.calls).toEqual([["l"], ["sm"]]);
  });
});
