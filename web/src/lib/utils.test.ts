import { describe, expect, it } from "vitest";
import { cn } from "./utils";

describe("cn with the type scale tokens", () => {
  it("keeps a role size next to a text colour", () => {
    expect(cn("text-body", "text-muted-foreground")).toBe("text-body text-muted-foreground");
    expect(cn("text-headline text-danger")).toBe("text-headline text-danger");
  });

  it("lets a later size replace an earlier one", () => {
    expect(cn("text-sm", "text-label")).toBe("text-label");
    expect(cn("text-body", "text-title")).toBe("text-title");
  });
});
