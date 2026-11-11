import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import { Toolbar as ToolbarPrimitive } from "@base-ui/react/toolbar";

/**
 * The keyboard half of the WAI-ARIA tabs pattern, from Base UI: a tablist is one Tab stop, Left and
 * Right move focus between its tabs (wrapping), Home and End jump to the first and the last, and a
 * focused tab is the selected one. The panels stay the caller's own (only the active one is
 * rendered), so these wrappers cover the root and the list and the tab, nothing else.
 */
const Tabs = TabsPrimitive.Root;
const TabsTab = TabsPrimitive.Tab;

function TabsList(props: TabsPrimitive.List.Props) {
  return <TabsPrimitive.List activateOnFocus loopFocus {...props} />;
}

/**
 * A vertical group of buttons that is one Tab stop, Up and Down moving between them and Home and
 * End jumping to the ends: the rail's activities. Its items stay plain buttons.
 */
function VerticalButtons({ onKeyDown, ...props }: ToolbarPrimitive.Root.Props) {
  return (
    <ToolbarPrimitive.Root
      orientation="vertical"
      // The toolbar primitive leaves Home and End out; the pattern has them.
      onKeyDown={(ev) => {
        onKeyDown?.(ev);
        if (ev.key !== "Home" && ev.key !== "End") return;
        const buttons = ev.currentTarget.querySelectorAll("button");
        buttons[ev.key === "Home" ? 0 : buttons.length - 1]?.focus();
        ev.preventDefault();
      }}
      {...props}
    />
  );
}
const VerticalButton = ToolbarPrimitive.Button;

export { Tabs, TabsList, TabsTab, VerticalButtons, VerticalButton };
