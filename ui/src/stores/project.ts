import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

interface ProjectState {
  slug: string;
  setSlug: (s: string) => void;
}

export const useProjectStore = create<ProjectState>()(
  persist(
    (set) => ({
      slug: "_default",
      setSlug: (slug) => set({ slug }),
    }),
    {
      name: "docsiq.project",
      storage: createJSONStorage(() => localStorage),
      version: 1,
    },
  ),
);
