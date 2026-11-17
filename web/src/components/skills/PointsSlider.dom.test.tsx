import { describe, expect, it } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { useState } from "react";
import { renderApp } from "@/test/render";
import { PointsSlider } from "./PointsSlider";

function Harness({
  start,
  limit,
  reach,
}: {
  start: string;
  limit: number;
  reach: number;
}) {
  const [v, setV] = useState(start);
  return (
    <PointsSlider
      id="pick_pockets"
      scope="set"
      name="Pick pockets"
      value={v}
      limit={limit}
      reach={reach}
      onChange={setV}
    />
  );
}

const box = () => screen.getByTestId("thief-points-pick_pockets");
const slider = () => screen.getByTestId("thief-points-slider-pick_pockets");

describe("PointsSlider", () => {
  it("reads its name and its value aloud", async () => {
    await renderApp(<Harness start="20" limit={30} reach={30} />);
    expect(slider()).toHaveAttribute("aria-valuetext", "20 of 30 points");
    expect(slider()).toHaveAttribute("max", "30");
    expect(box()).toHaveValue("20");
  });

  it("moves by 5 with PageUp and PageDown, never below 0", async () => {
    const { user } = await renderApp(
      <Harness start="3" limit={30} reach={30} />,
    );
    slider().focus();
    await user.keyboard("{PageUp}");
    expect(box()).toHaveValue("8");
    await user.keyboard("{PageDown}{PageDown}");
    expect(box()).toHaveValue("0");
  });

  it("stops a move at what the points left reach, and at the limit", async () => {
    const { user } = await renderApp(
      <Harness start="10" limit={30} reach={12} />,
    );
    slider().focus();
    await user.keyboard("{PageUp}");
    expect(box()).toHaveValue("12");
    fireEvent.change(slider(), { target: { value: "30" } });
    expect(box()).toHaveValue("12");
  });

  it("follows a native move (arrows, Home, End) into the box", async () => {
    await renderApp(<Harness start="" limit={30} reach={30} />);
    fireEvent.change(slider(), { target: { value: "7" } });
    expect(box()).toHaveValue("7");
    fireEvent.change(slider(), { target: { value: "0" } });
    expect(box()).toHaveValue("0");
  });

  it("is disabled when the skill has no room", async () => {
    await renderApp(<Harness start="0" limit={0} reach={0} />);
    expect(slider()).toBeDisabled();
  });
});
