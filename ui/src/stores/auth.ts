import { create } from "zustand";

// authRequired flips to true the first time any React Query sees a 401
// from /api/* or /mcp/*. Stays true until the user takes a sign-in
// action (e.g. reload after OOB provisioning). Not persisted — if the
// tab closes, the next session's cookies dictate the state afresh.
interface AuthState {
  authRequired: boolean;
  signalUnauthorized: () => void;
  clear: () => void;
}

export const useAuthStore = create<AuthState>()((set) => ({
  authRequired: false,
  signalUnauthorized: () => set({ authRequired: true }),
  clear: () => set({ authRequired: false }),
}));
