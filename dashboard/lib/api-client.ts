import axios, { type AxiosInstance } from "axios";
import type { Execution, PreparedSkill, Project, SearchFilter, Skill, SkillInput, SkillProposal, Statistics, ValidationResult } from "@/lib/types";

interface RpcEnvelope<T> { jsonrpc: "2.0"; id: number; result?: T; error?: { code: number; message: string } }
interface ToolResult<T> { content: Array<{ type: "text"; text: string }>; structuredContent: T; isError: boolean }
interface ScopedClient { client: AxiosInstance; initialize?: Promise<void> }

const configuredBaseUrl = (process.env.NEXT_PUBLIC_API_URL ?? "").replace(/\/$/, "");
const adminClient = axios.create({ baseURL: `${configuredBaseUrl}/admin/api`, timeout: 15000 });
const scopedClients = new Map<string, ScopedClient>();
let requestId = 0;

async function rpc<T>(client: AxiosInstance, method: string, params: Record<string, unknown>): Promise<T> {
  const { data } = await client.post<RpcEnvelope<T>>("", { jsonrpc: "2.0", id: ++requestId, method, params });
  if (data.error) throw new Error(data.error.message);
  if (data.result === undefined) throw new Error("SkillBox returned an empty response");
  return data.result;
}

function scopedClient(projectId: string, profile: "teacher" | "student") {
  if (!projectId) throw new Error("Skill is not connected to an MCP project");
  const key = `${profile}:${projectId}`;
  let scoped = scopedClients.get(key);
  if (!scoped) {
    const projectBaseUrl = `${configuredBaseUrl}/mcp/${encodeURIComponent(projectId)}`;
    scoped = {
      client: axios.create({
        baseURL: profile === "teacher" ? `${projectBaseUrl}/teacher` : projectBaseUrl,
        timeout: 15000,
        headers: { "Content-Type": "application/json" }
      })
    };
    scopedClients.set(key, scoped);
  }
  return scoped;
}

async function callTool<T>(projectId: string, profile: "teacher" | "student", name: string, args: Record<string, unknown> = {}): Promise<T> {
  const scoped = scopedClient(projectId, profile);
  scoped.initialize ??= rpc<unknown>(scoped.client, "initialize", {}).then(() => undefined).catch((error: unknown) => {
    scoped.initialize = undefined;
    throw error;
  });
  await scoped.initialize;
  const result = await rpc<ToolResult<T>>(scoped.client, "tools/call", { name, arguments: args });
  return result.structuredContent;
}

function matchesFilter(skill: Skill, filter: SearchFilter) {
  const task = filter.task?.trim().toLowerCase();
  if (task) {
    const searchable = [skill.name, skill.slug, skill.description, skill.purpose, ...skill.keywords, ...skill.tags].join(" ").toLowerCase();
    if (!searchable.includes(task)) return false;
  }
  if (filter.status && skill.status !== filter.status) return false;
  if (filter.scopes?.length && !filter.scopes.includes(skill.scope)) return false;
  if (filter.domains?.length && !filter.domains.some((value) => skill.domains.includes(value))) return false;
  if (filter.intents?.length && !filter.intents.some((value) => skill.intents.includes(value))) return false;
  return true;
}

function normalizeSkill(skill: Skill): Skill {
  return {
    ...skill,
    success_criteria: skill.success_criteria ?? [],
    domains: skill.domains ?? [],
    intents: skill.intents ?? [],
    object_types: skill.object_types ?? [],
    tags: skill.tags ?? [],
    keywords: skill.keywords ?? [],
    capabilities: skill.capabilities ?? [],
    compatibility: skill.compatibility ?? [],
    steps: skill.steps ?? [],
    tools: skill.tools ?? [],
    context_requirements: skill.context_requirements ?? [],
    dependencies: skill.dependencies ?? [],
    examples: skill.examples ?? []
  };
}

export const api = {
  async listProjects(): Promise<Project[]> {
    const { data } = await adminClient.get<{ projects: Project[] }>("/projects");
    return data.projects ?? [];
  },
  async listSkills(filter: SearchFilter = {}): Promise<Skill[]> {
    const { data } = await adminClient.get<{ skills: Skill[] }>("/skills");
    return (data.skills ?? []).map(normalizeSkill).filter((skill) => matchesFilter(skill, filter)).slice(0, filter.limit ?? 500);
  },
  async getSkill(skillId: string): Promise<Skill> {
    const { data } = await adminClient.get<Skill>(`/skills/${encodeURIComponent(skillId)}`);
    return normalizeSkill(data);
  },
  async createSkill(skill: SkillInput): Promise<Skill> {
    const { mcp_project: projectId, ...payload } = skill;
    const created = await callTool<Skill>(projectId, "teacher", "create_skill_draft", payload as Record<string, unknown>);
    return this.getSkill(created.id);
  },
  async updateSkill(skill: SkillInput, changeSummary = "Updated from Dashboard"): Promise<Skill> {
    const { mcp_project: projectId, ...payload } = skill;
    const updated = await callTool<Skill>(projectId, "teacher", "update_skill_draft", { skill: payload, change_summary: changeSummary });
    return this.getSkill(updated.id);
  },
  validateSkill: (skill: Skill) => callTool<ValidationResult>(skill.mcp_project, "teacher", "validate_skill", { skill_id: skill.id }),
  prepareSkill: (skill: Skill) => callTool<PreparedSkill>(skill.mcp_project, "student", "prepare_skill", { task: "Dashboard preview", skill_id: skill.id, model: { provider: "dashboard", name: "preview", context_window: 32000 }, max_skill_tokens: 4000 }),
  async getStatistics(skillId: string): Promise<Statistics> {
    const { data } = await adminClient.get<{ statistics: Statistics[] }>("/statistics", { params: { skill_id: skillId } });
    return data.statistics?.[0] ?? { skill_id: skillId, runs: 0, successes: 0, success_rate: 0, by_model: [] };
  },
  async listExecutions(skillId?: string, limit = 100): Promise<Execution[]> {
    const { data } = await adminClient.get<{ executions: Execution[] }>("/executions", { params: { ...(skillId ? { skill_id: skillId } : {}), limit } });
    return data.executions ?? [];
  },
  async listProposals(skillId?: string): Promise<SkillProposal[]> {
    const { data } = await adminClient.get<{ proposals: SkillProposal[] }>("/proposals", { params: skillId ? { skill_id: skillId } : {} });
    return data.proposals ?? [];
  },
  createProposal: (skill: Skill, summary: string) => callTool<SkillProposal>(skill.mcp_project, "teacher", "create_skill_proposal", { skill_id: skill.id, summary }),
  approveProposal: (skill: Skill, proposalId: string) => callTool<SkillProposal>(skill.mcp_project, "teacher", "approve_skill_proposal", { proposal_id: proposalId, note: "Approved in Dashboard" }),
  async publishProposal(skill: Skill, proposalId: string): Promise<Skill> {
    await callTool<Skill>(skill.mcp_project, "teacher", "publish_skill", { proposal_id: proposalId });
    return this.getSkill(skill.id);
  },
  async rollback(skill: Skill, version: number): Promise<Skill> {
    await callTool<Skill>(skill.mcp_project, "teacher", "rollback_skill_version", { skill_id: skill.id, version });
    return this.getSkill(skill.id);
  }
};
