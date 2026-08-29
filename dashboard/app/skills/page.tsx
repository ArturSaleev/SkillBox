"use client";

import Link from "next/link";
import { Plus, Search } from "lucide-react";
import { useMemo, useState } from "react";
import { useProjects, useSkills } from "@/lib/hooks";
import { useDashboardStore } from "@/lib/store";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { SkillTable } from "@/components/skill-table";
import { EmptyState, ErrorState } from "@/components/data-state";
import { Skeleton } from "@/components/ui/skeleton";

export default function SkillsPage() {
  const { data, isLoading, error } = useSkills();
  const projects = useProjects();
  const globalQuery = useDashboardStore((state) => state.searchQuery);
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [scope, setScope] = useState("all");
  const [projectId, setProjectId] = useState("all");
  const filtered = useMemo(() => (data ?? []).filter((skill) => (status === "all" || skill.status === status) && (scope === "all" || skill.scope === scope) && (projectId === "all" || skill.project_id === projectId)), [data, projectId, status, scope]);
  return <div className="page-shell"><section className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end"><div><p className="eyebrow">Admin library</p><h1 className="page-heading mt-2">All reusable procedures</h1><p className="mt-2 text-sm text-muted-foreground">Every Skill in the database, across all MCP projects.</p></div><Button><Link href="/editor" className="flex items-center gap-2"><Plus className="size-4" />Create Skill</Link></Button></section>
    <Card><CardHeader><CardTitle>Skills Library</CardTitle><div className="mt-4 grid gap-3 md:grid-cols-[1fr_180px_180px_200px]"><div className="relative"><Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name, keyword or purpose" className="pl-10" /></div><Select value={projectId} onChange={(event) => setProjectId(event.target.value)}><option value="all">All projects</option>{(projects.data ?? []).map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}</Select><Select value={status} onChange={(event) => setStatus(event.target.value)}><option value="all">All statuses</option><option value="active">Active</option><option value="draft">Draft</option><option value="deprecated">Deprecated</option><option value="archived">Archived</option></Select><Select value={scope} onChange={(event) => setScope(event.target.value)}><option value="all">All scopes</option><option value="global">Global</option><option value="workspace">Workspace</option><option value="project">Project</option></Select></div></CardHeader><CardContent className="p-0">{isLoading ? <div className="space-y-2 p-5">{Array.from({ length: 7 }, (_, index) => <Skeleton key={index} className="h-14" />)}</div> : error || projects.error ? <div className="p-5"><ErrorState error={(error ?? projects.error) as Error} /></div> : filtered.length ? <SkillTable skills={filtered} query={[globalQuery, query].filter(Boolean).join(" ")} /> : <div className="p-5"><EmptyState /></div>}</CardContent></Card>
  </div>;
}
