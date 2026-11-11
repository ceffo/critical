// Setup of the component-test project (`*.dom.test.tsx`): jest-dom matchers, and the DOM emptied
// between tests (Testing Library only does that itself when vitest globals are on).
import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(cleanup);
