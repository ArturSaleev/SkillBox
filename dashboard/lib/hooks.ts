"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import type { SearchFilter, SkillInput } from "@/lib/types";

export function useSkills(filter: SearchFilter = {}) { return useQuery({ queryKey: ["skills", filter], queryFn: () => api.listSkills(filter), staleTime: 300_000 }); }
export function useProjects() { return useQuery({ queryKey: ["projects"], queryFn: () => api.listProjects(), staleTime: 300_000 }); }
export function useSkill(skillId: string) { return useQuery({ queryKey: ["skill", skillId], queryFn: () => api.getSkill(skillId), enabled: Boolean(skillId), staleTime: 300_000 }); }
export function useExecutions(skillId?: string, refetchInterval?: number) { return useQuery({ queryKey: ["executions", skillId], queryFn: () => api.listExecutions(skillId), refetchInterval }); }
export function useStatistics(skillId: string) { return useQuery({ queryKey: ["statistics", skillId], queryFn: () => api.getStatistics(skillId), enabled: Boolean(skillId), staleTime: 300_000 }); }
export function useSearchSkills(filter: SearchFilter) { return useSkills(filter); }
export function useProposals(skillId?: string) { return useQuery({ queryKey: ["proposals", skillId], queryFn: () => api.listProposals(skillId), staleTime: 30_000 }); }
export function useSaveSkill() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (skill: SkillInput) => skill.id ? api.updateSkill(skill) : api.createSkill(skill),
    onSuccess: (skill) => { queryClient.setQueryData(["skill", skill.id], skill); void queryClient.invalidateQueries({ queryKey: ["skills"] }); }
  });
}
