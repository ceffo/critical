import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { SpellDescription } from "./SpellDescription";

const TEXT = [
  "First paragraph.",
  "- one\n- two",
  "| Roll | Effect |\n| --- | --- |\n| 1 | Red |\n| 2 | Orange |",
  "Last paragraph.",
].join("\n\n");

const html = (text = TEXT, className?: string) =>
  renderToStaticMarkup(<SpellDescription text={text} className={className} />);

describe("SpellDescription (one readable column)", () => {
  it("is one column capped at 75 characters, whatever the caller", () => {
    const out = html(TEXT, "mt-2");
    expect(out).toContain("max-w-[75ch]");
    expect(out).toContain("flex-col");
    expect(out).not.toMatch(/columns|break-inside|column-span/);
  });

  it("spaces blocks 16 px apart, further than the lines of a paragraph", () => {
    const out = html();
    expect(out).toContain("gap-4");
    expect(out).toContain("leading-relaxed");
  });

  it("gives table cells room and a raised header, and scrolls a wide table in its own box", () => {
    const out = html();
    expect(out).toContain("overflow-x-auto");
    expect(out.match(/<th[^>]*px-3 py-2/g)).toHaveLength(2);
    expect(out.match(/<td[^>]*px-3 py-2/g)).toHaveLength(4);
    expect(out).toContain("bg-raised");
  });

  it("renders paragraphs, a list and a table in order", () => {
    const out = html();
    const at = (s: string) => out.indexOf(s);
    expect(at("First paragraph.")).toBeLessThan(at("<ul"));
    expect(at("<ul")).toBeLessThan(at("<table"));
    expect(at("<table")).toBeLessThan(at("Last paragraph."));
  });
});
