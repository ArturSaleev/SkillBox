"use client";

import { Activity, Bot, Clock3 } from "lucide-react";
import { useExecutions } from "@/lib/hooks";
import { formatDate, formatDuration } from "@/lib/utils";
import { EmptyState, ErrorState } from "@/components/data-state";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/status-badge";

export function ExecutionFeed({ skillId, live = false, limit = 10 }: { skillId?: string; live?: boolean; limit?: number }) {
  const { data, isLoading, error } = useExecutions(skillId, live ? 3000 : undefined);
  if (isLoading) return <div className="space-y-3">{Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-20" />)}</div>;
  if (error) return <ErrorState error={error} />;
  const executions = (data ?? []).slice(0, limit);
  if (!executions.length) return <EmptyState title="Executions ещё не поступали" description="Лента обновится автоматически после первого запуска Skill." />;
  return <div className="space-y-2">{executions.map((execution) => <div key={execution.id} className="flex flex-col gap-3 rounded-xl border border-border p-4 sm:flex-row sm:items-center">
    <div className="grid size-10 shrink-0 place-items-center rounded-xl bg-muted"><Activity className="size-4 text-primary" /></div>
    <div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{execution.task_summary}</p><div className="mt-1 flex flex-wrap gap-3 text-xs text-muted-foreground"><span className="flex items-center gap-1"><Bot className="size-3" />{execution.model_name ?? "Unknown model"}</span><span className="flex items-center gap-1"><Clock3 className="size-3" />{formatDuration(execution.duration_ms)}</span><span>{formatDate(execution.started_at)}</span></div></div>
    <StatusBadge status={execution.status || (execution.success ? "success" : "failed")} />
  </div>)}</div>;
}
