import { afterEach, describe, expect, it, vi } from "vitest";
import { act, screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { MinWidthGate } from "./MinWidthGate";

/** A window whose width the test can change: matchMedia answers for it and tells its listeners. */
function windowOf(width: number) {
  let current = width;
  const listeners = new Set<() => void>();
  vi.stubGlobal("matchMedia", (query: string) => ({
    get matches() {
      const min = Number(/min-width: (\d+)px/.exec(query)![1]);
      return current >= min;
    },
    addEventListener: (_: string, l: () => void) => listeners.add(l),
    removeEventListener: (_: string, l: () => void) => listeners.delete(l),
  }));
  return (w: number) => {
    current = w;
    act(() => listeners.forEach((l) => l()));
  };
}

afterEach(() => vi.unstubAllGlobals());

describe("MinWidthGate", () => {
  it("shows the app at 1024 and the message at 1023, in English and French", async () => {
    windowOf(1024);
    const en = await renderApp(
      <MinWidthGate>
        <i data-testid="app" />
      </MinWidthGate>,
    );
    expect(screen.getByTestId("app")).toBeTruthy();
    en.unmount();
    windowOf(1023);
    const gate = (
      <MinWidthGate>
        <i data-testid="app" />
      </MinWidthGate>
    );
    const view = await renderApp(gate);
    expect(screen.queryByTestId("app")).toBeNull();
    expect(screen.getByRole("heading").textContent).toBe(
      "This app needs a wider window",
    );
    view.unmount();
    await renderApp(gate, { lng: "fr" });
    expect(screen.getByRole("heading").textContent).toBe(
      "Cette application a besoin d'une fenêtre plus large",
    );
  });

  it("follows a resize across the threshold", async () => {
    const resize = windowOf(1280);
    await renderApp(
      <MinWidthGate>
        <i data-testid="app" />
      </MinWidthGate>,
    );
    resize(800);
    expect(screen.queryByTestId("app")).toBeNull();
    expect(screen.getByTestId("min-width-message")).toBeTruthy();
    resize(1024);
    expect(screen.getByTestId("app")).toBeTruthy();
    expect(screen.queryByTestId("min-width-message")).toBeNull();
  });
});
