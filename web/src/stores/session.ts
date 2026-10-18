import { create } from "zustand";
import type { Me } from "@/api/auth";

interface SessionStore {
  /** The logged-in user, read once at start-up (`GET /api/auth/me`); null until then or without an answer. */
  me: Me | null;
  setMe: (me: Me | null) => void;
}

export const useSessionStore = create<SessionStore>((set) => ({
  me: null,
  setMe: (me) => set({ me }),
}));
