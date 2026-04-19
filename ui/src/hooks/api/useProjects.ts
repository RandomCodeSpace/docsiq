import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api-client";
import { qk } from "./keys";
import type { Project } from "@/types/api";

export function useProjects() {
  return useQuery({
    queryKey: qk.projects(),
    queryFn: async () => {
      const res = await apiFetch<Project[] | null>("/api/projects");
      return Array.isArray(res) ? res : [];
    },
  });
}
