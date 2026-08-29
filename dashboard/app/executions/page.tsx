"use client";

import { useMemo, useState } from "react";
import { Activity, Radio } from "lucide-react";
import { useExecutions, useSkills } from "@/lib/hooks";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ExecutionFeed } from "@/components/execution-feed";
import { Select } from "@/components/ui/select";
import { StatusBadge } from "@/components/status-badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState, ErrorState } from "@/components/data-state";
import { formatDate, formatDuration } from "@/lib/utils";

export default function ExecutionsPage() {
  const skills = useSkills();
  const executions = useExecutions(undefined, 3000);
  const [status, setStatus] = useState("all");
  const [skillId, setSkillId] = useState("all");
  const skillMap = useMemo(() => new Map((skills.data ?? []).map((skill) => [skill.id, skill.name])), [skills.data]);
  const filtered = (executions.data ?? []).filter((item) => (status === "all" || item.status === status || (status === "success" && item.success)) && (skillId === "all" || item.skill_id === skillId));
  return <div className="page-shell"><section><div className="flex items-center gap-2"><p className="eyebrow">Execution monitor</p><span className="flex items-center gap-1 text-xs text-emerald-600"><Radio className="size-3 animate-pulse" />live</span></div><h1 className="page-heading mt-2">Agent activity</h1><p className="mt-2 text-sm text-muted-foreground">Auto-refreshes every three seconds from SkillBox telemetry.</p></section>
    <div className="grid gap-6 xl:grid-cols-[1.4fr_.6fr]"><Card><CardHeader><div className="flex flex-col justify-between gap-4 md:flex-row md:items-center"><div><CardTitle>All executions</CardTitle><CardDescription>{filtered.length} visible records</CardDescription></div><div className="flex gap-2"><Select value={skillId} onChange={(event) => setSkillId(event.target.value)}><option value="all">All Skills</option>{(skills.data ?? []).map((skill) => <option key={skill.id} value={skill.id}>{skill.name}</option>)}</Select><Select value={status} onChange={(event) => setStatus(event.target.value)}><option value="all">All statuses</option><option value="running">Running</option><option value="success">Success</option><option value="failed">Failed</option></Select></div></div></CardHeader><CardContent className="p-0">{executions.error ? <div className="p-5"><ErrorState error={executions.error} /></div> : filtered.length ? <Table><TableHeader><TableRow><TableHead>Skill</TableHead><TableHead>Model</TableHead><TableHead>Status</TableHead><TableHead>Duration</TableHead><TableHead>Started</TableHead></TableRow></TableHeader><TableBody>{filtered.map((execution) => <TableRow key={execution.id}><TableCell><p className="font-medium">{skillMap.get(execution.skill_id) ?? execution.skill_id.slice(0, 8)}</p><p className="max-w-xs truncate text-xs text-muted-foreground">{execution.task_summary}</p></TableCell><TableCell>{execution.model_name ?? "—"}</TableCell><TableCell><StatusBadge status={execution.status || (execution.success ? "success" : "failed")} /></TableCell><TableCell>{formatDuration(execution.duration_ms)}</TableCell><TableCell>{formatDate(execution.started_at)}</TableCell></TableRow>)}</TableBody></Table> : <div className="p-5"><EmptyState title="Нет подходящих executions" description="Измените фильтры или дождитесь следующего запуска." /></div>}</CardContent></Card>
      <Card><CardHeader><div className="flex items-center gap-2"><Activity className="size-4 text-primary" /><CardTitle>Live feed</CardTitle></div><CardDescription>Последние события</CardDescription></CardHeader><CardContent><ExecutionFeed live limit={8} /></CardContent></Card></div>
  </div>;
}
