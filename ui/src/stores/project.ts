import { create } from "zustand";

interface ProjectState {
  slug: string;
  setSlug: (s: string) => void;
}

export const useProjectStore = create<ProjectState>((set) => ({
  slug: "_default",
  setSlug: (slug) => set({ slug }),
}));
