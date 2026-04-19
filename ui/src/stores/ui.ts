import { create } from "zustand";
import { persist } from "zustand/middleware";

type Theme = "light" | "dark" | "system";

interface UIState {
  sidebarCollapsed: boolean;
  theme: Theme;
  treeDrawerPinned: boolean;
  linkDrawerPinned: boolean;
  setSidebarCollapsed: (v: boolean) => void;
  toggleSidebar: () => void;
  setTheme: (t: Theme) => void;
  setTreeDrawerPinned: (v: boolean) => void;
  setLinkDrawerPinned: (v: boolean) => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,
      theme: "system",
      treeDrawerPinned: false,
      linkDrawerPinned: false,
      setSidebarCollapsed: (sidebarCollapsed) => set({ sidebarCollapsed }),
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
      setTheme: (theme) => set({ theme }),
      setTreeDrawerPinned: (treeDrawerPinned) => set({ treeDrawerPinned }),
      setLinkDrawerPinned: (linkDrawerPinned) => set({ linkDrawerPinned }),
    }),
    { name: "docsiq-ui" },
  ),
);
