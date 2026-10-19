import { valueSpans } from "@/lib/valueMarks";

/**
 * A marked text (lib/valueMarks) as React text, each gameplay value in the one value style: the
 * accent colour in semibold (ADR 0021 step 15). Contrast on the card and the raised step is held
 * by theme.test for every accent. The kind is a data attribute, for a style per kind later.
 */
export function ValueText({ text }: { text: string }) {
  return (
    <>
      {valueSpans(text).map((s, i) =>
        s.kind ? (
          <span
            key={i}
            className="font-semibold text-highlight"
            data-value={s.kind}
          >
            {s.text}
          </span>
        ) : (
          s.text
        ),
      )}
    </>
  );
}
