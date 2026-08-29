"use client";

import Link from "next/link";
import { useQueries } from "@tanstack/react-query";
import { Activity, ArrowRight, BookOpen, CheckCircle2, FilePenLine, Gauge, Plus } from "lucide-react";
import { api } from "@/lib/api-client";
import { useSkills } from "@/lib/hooks";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { LoadingGrid, ErrorState } from "@/components/data-state";
import { ExecutionFeed } from "@/components/execution-feed";
import { MetricsChart } from "@/components/metrics-chart";

export default function OverviewPage() {
  const skills = useSkills();
  const stats = useQueries({ queries: (skills.data ?? []).map((skill) => ({ queryKey: ["statistics", skill.id], queryFn: () => api.getStatistics(skill.id), staleTime: 300_000 })) });
  if (skills.isLoading) return <div className="page-shell"><LoadingGrid /></div>;
  if (skills.error) return <div className="page-shell"><ErrorState error={skills.error} /></div>;
  const items = skills.data ?? [];
  const statItems = stats.flatMap((item) => item.data ? [item.data] : []);
  const totalRuns = statItems.reduce((sum, item) => sum + item.runs, 0);
  const totalSuccesses = statItems.reduce((sum, item) => sum + item.successes, 0);
  const successRate = totalRuns ? Math.round(totalSuccesses / totalRuns * 100) : 0;
  const topSkills = statItems.map((stat) => ({ name: items.find((skill) => skill.id === stat.skill_id)?.name ?? stat.skill_id.slice(0, 8), value: Math.round(stat.success_rate * (stat.success_rate <= 1 ? 100 : 1)) })).sort((a, b) => b.value - a.value).slice(0, 5);
  const modelMap = new Map<string, { runs: number; successes: number }>();
  statItems.flatMap((item) => item.by_model).forEach((model) => { const key = model.model || model.provider; const current = modelMap.get(key) ?? { runs: 0, successes: 0 }; modelMap.set(key, { runs: current.runs + model.runs, successes: current.successes + model.successes }); });
  const topModels = Array.from(modelMap, ([name, value]) => ({ name, value: value.runs ? Math.round(value.successes / value.runs * 100) : 0 })).sort((a, b) => b.value - a.value).slice(0, 5);
  const cards = [
    { label: "Total Skills", value: items.length, note: `${items.filter((skill) => skill.scope === "project").length} project scoped`, icon: BookOpen, color: "text-cyan-600 bg-cyan-500/10" },
    { label: "Active", value: items.filter((skill) => skill.status === "active").length, note: "Published procedures", icon: CheckCircle2, color: "text-emerald-600 bg-emerald-500/10" },
    { label: "Draft", value: items.filter((skill) => skill.status === "draft").length, note: "Awaiting validation", icon: FilePenLine, color: "text-amber-600 bg-amber-500/10" },
    { label: "Success Rate", value: `${successRate}%`, note: `${totalRuns} tracked runs`, icon: Gauge, color: "text-violet-600 bg-violet-500/10" }
  ];
  return <div className="page-shell">
    <section className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end"><div><p className="eyebrow">Global admin overview</p><h1 className="page-heading mt-2">Good procedures make agents reliable.</h1><p className="mt-2 text-sm text-muted-foreground">Database-wide control surface across every SkillBox project.</p></div><Button><Link href="/editor" className="flex items-center gap-2"><Plus className="size-4" />New Skill</Link></Button></section>
    <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{cards.map(({ label, value, note, icon: Icon, color }) => <Card key={label}><CardContent className="flex items-start justify-between pt-5"><div><p className="text-sm text-muted-foreground">{label}</p><p className="mt-2 text-3xl font-semibold tracking-tight">{value}</p><p className="mt-2 text-xs text-muted-foreground">{note}</p></div><div className={`grid size-10 place-items-center rounded-xl ${color}`}><Icon className="size-5" /></div></CardContent></Card>)}</section>
    <section className="grid gap-6 xl:grid-cols-[1.35fr_.65fr]"><Card><CardHeader className="flex-row items-center justify-between"><div><CardTitle>Recent executions</CardTitle><CardDescription>Latest evidence from agents</CardDescription></div><Link href="/executions" className="text-xs font-semibold text-primary">Open monitor</Link></CardHeader><CardContent><ExecutionFeed limit={6} /></CardContent></Card><Card><CardHeader><CardTitle>Quick actions</CardTitle><CardDescription>Move the lifecycle forward</CardDescription></CardHeader><CardContent className="space-y-2">{[{ href: "/skills", label: "Browse Skills", icon: BookOpen }, { href: "/editor", label: "Create a draft", icon: FilePenLine }, { href: "/executions", label: "Watch executions", icon: Activity }].map(({ href, label, icon: Icon }) => <Link key={href} href={href} className="flex items-center gap-3 rounded-xl border border-border p-4 transition hover:border-primary"><Icon className="size-4 text-primary" /><span className="flex-1 text-sm font-medium">{label}</span><ArrowRight className="size-4 text-muted-foreground" /></Link>)}</CardContent></Card></section>
    <section className="grid gap-6 xl:grid-cols-2"><Card><CardHeader><CardTitle>Top Skills by success rate</CardTitle><CardDescription>Based on recorded execution outcomes</CardDescription></CardHeader><CardContent><MetricsChart data={topSkills} valueLabel="Success %" /></CardContent></Card><Card><CardHeader><CardTitle>Top models by success rate</CardTitle><CardDescription>Aggregated across visible Skills</CardDescription></CardHeader><CardContent><MetricsChart data={topModels} valueLabel="Success %" /></CardContent></Card></section>
  </div>;
}
