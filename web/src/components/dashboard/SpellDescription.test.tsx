import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { SpellDescription } from "./SpellDescription";
import { mark } from "@/lib/valueMarks";

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

describe("SpellDescription (gameplay values)", () => {
  const marked = (s: string) => `${mark("dice", s)}`;
  const text = [
    `Deals ${marked("1d6")} damage.`,
    `- lasts ${mark("time", "2 rounds")}`,
    `| Roll | Effect |\n| --- | --- |\n| 1 | ${mark("distance", "10 feet")} |`,
  ].join("\n\n");

  it("shows a value in the value style in a paragraph, a list item and a table cell", () => {
    const out = renderToStaticMarkup(<SpellDescription text={text} />);
    expect(out).toMatch(
      /<p[^>]*>Deals <span[^>]*text-highlight[^>]*data-value="dice">1d6<\/span> damage\./,
    );
    expect(out).toMatch(
      /<li>lasts <span[^>]*data-value="time">2 rounds<\/span><\/li>/,
    );
    expect(out).toMatch(
      /<td[^>]*>1<\/td><td[^>]*><span[^>]*data-value="distance">10 feet<\/span><\/td>/,
    );
  });

  it("is the same style for every kind, and plain text is untouched", () => {
    const out = renderToStaticMarkup(<SpellDescription text={text} />);
    const classes = new Set(out.match(/class="[^"]*text-highlight[^"]*"/g));
    expect(classes.size).toBe(1);
    expect(html("Just words.")).not.toContain("<span");
  });
});
