"use client";

import { useQueries } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { useExecutions, useSkills } from "@/lib/hooks";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { MetricsChart } from "@/components/metrics-chart";
import { ErrorState, LoadingGrid } from "@/components/data-state";

export default function AnalyticsPage() {
  const skills = useSkills();
  const executions = useExecutions();
  const stats = useQueries({ queries: (skills.data ?? []).map((skill) => ({ queryKey: ["statistics", skill.id], queryFn: () => api.getStatistics(skill.id), staleTime: 300_000 })) });
  if (skills.isLoading || executions.isLoading) return <div className="page-shell"><LoadingGrid /></div>;
  if (skills.error || executions.error) return <div className="page-shell"><ErrorState error={(skills.error ?? executions.error) as Error} /></div>;
  const skillMap = new Map((skills.data ?? []).map((skill) => [skill.id, skill]));
  const statItems = stats.flatMap((item) => item.data ? [item.data] : []);
  const domainStats = new Map<string, { runs: number; successes: number; duration: number; count: number }>();
  statItems.forEach((stat) => { const skill = skillMap.get(stat.skill_id); (skill?.domains.length ? skill.domains : ["Uncategorized"]).forEach((domain) => { const current = domainStats.get(domain) ?? { runs: 0, successes: 0, duration: 0, count: 0 }; domainStats.set(domain, { ...current, runs: current.runs + stat.runs, successes: current.successes + stat.successes }); }); });
  (executions.data ?? []).forEach((execution) => { const skill = skillMap.get(execution.skill_id); (skill?.domains.length ? skill.domains : ["Uncategorized"]).forEach((domain) => { const current = domainStats.get(domain) ?? { runs: 0, successes: 0, duration: 0, count: 0 }; domainStats.set(domain, { ...current, duration: current.duration + (execution.duration_ms ?? 0), count: current.count + 1 }); }); });
  const byDomain = Array.from(domainStats, ([name, value]) => ({ name, value: value.runs ? Math.round(value.successes / value.runs * 100) : 0 }));
  const duration = Array.from(domainStats, ([name, value]) => ({ name, value: value.count ? Math.round(value.duration / value.count) : 0 }));
  const statuses = Object.entries((executions.data ?? []).reduce<Record<string, number>>((acc, item) => { const key = item.status || (item.success ? "success" : "failed"); acc[key] = (acc[key] ?? 0) + 1; return acc; }, {})).map(([name, value]) => ({ name, value }));
  const errors = Object.entries((executions.data ?? []).filter((item) => item.error_type).reduce<Record<string, number>>((acc, item) => { const key = item.error_type ?? "unknown"; acc[key] = (acc[key] ?? 0) + 1; return acc; }, {})).map(([name, value]) => ({ name, value }));
  const models = new Map<string, { runs: number; successes: number }>();
  statItems.flatMap((item) => item.by_model).forEach((item) => { const key = item.model || item.provider; const current = models.get(key) ?? { runs: 0, successes: 0 }; models.set(key, { runs: current.runs + item.runs, successes: current.successes + item.successes }); });
  const byModel = Array.from(models, ([name, item]) => ({ name, value: item.runs ? Math.round(item.successes / item.runs * 100) : 0 }));
  const charts = [
    { title: "Success rate by domain", description: "Outcome quality by Skill domain", data: byDomain, type: "bar" as const, label: "Success %" },
    { title: "Success rate by model", description: "Aggregated model performance", data: byModel, type: "line" as const, label: "Success %" },
    { title: "Execution distribution", description: "Current status mix", data: statuses, type: "pie" as const, label: "Executions" },
    { title: "Average duration by domain", description: "Mean execution time", data: duration, type: "bar" as const, label: "Milliseconds" },
    { title: "Error types", description: "Breakdown of reported failures", data: errors, type: "pie" as const, label: "Errors" }
  ];
  return <div className="page-shell"><section><p className="eyebrow">Metrics & insights</p><h1 className="page-heading mt-2">Procedure quality</h1><p className="mt-2 text-sm text-muted-foreground">Evidence derived from persisted SkillBox executions—no synthetic values.</p></section><section className="grid gap-6 xl:grid-cols-2">{charts.map((chart) => <Card key={chart.title}><CardHeader><CardTitle>{chart.title}</CardTitle><CardDescription>{chart.description}</CardDescription></CardHeader><CardContent><MetricsChart data={chart.data} type={chart.type} valueLabel={chart.label} /></CardContent></Card>)}</section></div>;
}
