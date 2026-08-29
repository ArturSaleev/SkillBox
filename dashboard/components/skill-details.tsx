"use client";

import Link from "next/link";
import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { ArrowLeft, CheckCircle2, Pencil, RotateCcw, Rocket, Send, ShieldCheck } from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { useProposals, useSkill, useStatistics } from "@/lib/hooks";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { StatusBadge } from "@/components/status-badge";
import { DependencyGraph } from "@/components/dependency-graph";
import { ExecutionFeed } from "@/components/execution-feed";
import { MetricsChart } from "@/components/metrics-chart";
import { ErrorState, LoadingGrid, EmptyState } from "@/components/data-state";
import { formatDate } from "@/lib/utils";

const tabs = ["Overview", "Steps", "Tools", "Context", "Dependencies", "Examples", "Versions", "Executions"] as const;
type Tab = typeof tabs[number];

export function SkillDetails() {
  const skillId = useSearchParams().get("id") ?? "";
  const skill = useSkill(skillId);
  const stats = useStatistics(skillId);
  const proposals = useProposals(skillId);
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<Tab>("Overview");
  const [busy, setBusy] = useState(false);
  if (skill.isLoading) return <div className="page-shell"><LoadingGrid /></div>;
  if (skill.error || !skill.data) return <div className="page-shell"><ErrorState error={skill.error ?? new Error("Skill not found")} /></div>;
  const item = skill.data;
  const refresh = async () => { await Promise.all([queryClient.invalidateQueries({ queryKey: ["skill", item.id] }), queryClient.invalidateQueries({ queryKey: ["skills"] }), queryClient.invalidateQueries({ queryKey: ["proposals", item.id] })]); };
  const lifecycle = async (action: "validate" | "propose" | "approve" | "publish") => {
    setBusy(true);
    try {
      if (action === "validate") { const result = await api.validateSkill(item); if (result.valid) toast.success("Skill is valid"); else toast.error(result.issues.join(" · ")); }
      if (action === "propose") { await api.createProposal(item, `Publish ${item.name} v${item.current_version}`); toast.success("Proposal created"); }
      if (action === "approve") { const pending = proposals.data?.find((proposal) => proposal.status === "pending"); if (!pending) throw new Error("No pending proposal"); await api.approveProposal(item, pending.id); toast.success("Proposal approved"); }
      if (action === "publish") { const approved = proposals.data?.find((proposal) => proposal.status === "approved"); if (!approved) throw new Error("No approved proposal"); await api.publishProposal(item, approved.id); toast.success("Skill published"); }
      await refresh();
    } catch (error) { toast.error(error instanceof Error ? error.message : "Action failed"); } finally { setBusy(false); }
  };
  return <div className="page-shell">
    <Link href="/skills" className="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"><ArrowLeft className="size-4" />Back to library</Link>
    <section className="flex flex-col justify-between gap-4 lg:flex-row lg:items-start"><div><div className="flex flex-wrap items-center gap-2"><StatusBadge status={item.status} /><Badge>{item.scope}</Badge>{item.project && <Badge>{item.project.name}</Badge>}<span className="text-xs text-muted-foreground">v{item.current_version}</span></div><h1 className="page-heading mt-3">{item.name}</h1><p className="mt-2 max-w-3xl text-sm text-muted-foreground">{item.description}</p></div><div className="flex flex-wrap gap-2"><Button variant="outline"><Link href={`/editor?id=${encodeURIComponent(item.id)}`} className="flex items-center gap-2"><Pencil className="size-4" />Edit</Link></Button><Button variant="outline" disabled={busy} onClick={() => lifecycle("validate")}><ShieldCheck className="size-4" />Validate</Button><Button variant="outline" disabled={busy} onClick={() => lifecycle("propose")}><Send className="size-4" />Propose</Button><Button disabled={busy || !proposals.data?.some((proposal) => proposal.status === "approved")} onClick={() => lifecycle("publish")}><Rocket className="size-4" />Publish</Button></div></section>
    <div className="flex gap-1 overflow-auto border-b border-border">{tabs.map((name) => <button key={name} onClick={() => setTab(name)} className={`whitespace-nowrap border-b-2 px-4 py-3 text-sm font-medium transition ${tab === name ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"}`}>{name}</button>)}</div>
    {tab === "Overview" && <div className="grid gap-6 xl:grid-cols-[1.3fr_.7fr]"><Card><CardHeader><CardTitle>Procedure definition</CardTitle></CardHeader><CardContent className="space-y-5"><Info label="Purpose" value={item.purpose} /><Info label="When to use" value={item.when_to_use} /><Info label="When not to use" value={item.when_not_to_use || "Not specified"} /><Info label="Instructions" value={item.instructions} pre /></CardContent></Card><div className="space-y-6"><Card><CardHeader><CardTitle>Selection metadata</CardTitle></CardHeader><CardContent className="space-y-4"><Tags label="Domains" values={item.domains} /><Tags label="Intents" values={item.intents} /><Tags label="Keywords" values={item.keywords} /><Info label="Priority" value={String(item.priority)} /></CardContent></Card><Card><CardHeader><CardTitle>Success criteria</CardTitle></CardHeader><CardContent><ul className="space-y-2">{item.success_criteria.map((criterion) => <li key={criterion} className="flex gap-2 text-sm"><CheckCircle2 className="mt-0.5 size-4 shrink-0 text-emerald-500" />{criterion}</li>)}</ul></CardContent></Card></div></div>}
    {tab === "Steps" && <Card><CardContent className="p-0">{item.steps.length ? <Table><TableHeader><TableRow><TableHead>#</TableHead><TableHead>Title</TableHead><TableHead>Instruction</TableHead><TableHead>Required</TableHead><TableHead>Expected result</TableHead></TableRow></TableHeader><TableBody>{item.steps.map((step) => <TableRow key={step.id || step.position}><TableCell>{step.position}</TableCell><TableCell className="font-medium">{step.title}</TableCell><TableCell className="max-w-xl whitespace-pre-wrap">{step.instruction}</TableCell><TableCell>{step.is_required ? "Yes" : "No"}</TableCell><TableCell>{step.expected_result || "—"}</TableCell></TableRow>)}</TableBody></Table> : <div className="p-5"><EmptyState /></div>}</CardContent></Card>}
    {tab === "Tools" && <Card><CardContent className="p-0">{item.tools.length ? <Table><TableHeader><TableRow><TableHead>Tool</TableHead><TableHead>Requirement</TableHead><TableHead>Purpose</TableHead><TableHead>Usage hint</TableHead></TableRow></TableHeader><TableBody>{item.tools.map((tool) => <TableRow key={tool.id || tool.tool_name}><TableCell className="font-mono text-xs">{tool.tool_namespace ? `${tool.tool_namespace}.` : ""}{tool.tool_name}</TableCell><TableCell><Badge>{tool.requirement}</Badge></TableCell><TableCell>{tool.purpose || "—"}</TableCell><TableCell>{tool.usage_hint || "—"}</TableCell></TableRow>)}</TableBody></Table> : <div className="p-5"><EmptyState title="No tool requirements" /></div>}</CardContent></Card>}
    {tab === "Context" && <Card><CardContent className="p-0">{item.context_requirements.length ? <Table><TableHeader><TableRow><TableHead>Type</TableHead><TableHead>Query</TableHead><TableHead>Required</TableHead><TableHead>Priority</TableHead><TableHead>Max tokens</TableHead></TableRow></TableHeader><TableBody>{item.context_requirements.map((context) => <TableRow key={context.id || context.query}><TableCell><Badge>{context.type}</Badge></TableCell><TableCell className="max-w-xl font-mono text-xs">{context.query}</TableCell><TableCell>{context.required ? "Yes" : "No"}</TableCell><TableCell>{context.priority}</TableCell><TableCell>{context.max_tokens ?? "—"}</TableCell></TableRow>)}</TableBody></Table> : <div className="p-5"><EmptyState title="No context requirements" /></div>}</CardContent></Card>}
    {tab === "Dependencies" && <DependencyGraph skill={item} dependencies={item.dependencies} />}
    {tab === "Examples" && <div className="grid gap-4 lg:grid-cols-2">{item.examples.length ? item.examples.map((example) => <Card key={example.id || example.title}><CardHeader><CardTitle>{example.title}</CardTitle><CardDescription>Priority {example.priority}</CardDescription></CardHeader><CardContent className="space-y-4"><Info label="Input" value={example.input_example} pre /><Info label="Expected behavior" value={example.expected_behavior} pre />{example.bad_behavior && <Info label="Bad behavior" value={example.bad_behavior} pre />}</CardContent></Card>) : <EmptyState title="No examples" />}</div>}
    {tab === "Versions" && <Card><CardHeader><div className="flex items-center justify-between"><div><CardTitle>Lifecycle history</CardTitle><CardDescription>MCP currently exposes proposals and the current version; immutable version listing remains backend-internal.</CardDescription></div>{proposals.data?.some((proposal) => proposal.status === "pending") && <Button size="sm" onClick={() => lifecycle("approve")} disabled={busy}>Approve pending</Button>}</div></CardHeader><CardContent className="space-y-3"><div className="flex items-center justify-between rounded-xl border border-primary/30 bg-primary/5 p-4"><div><p className="font-semibold">Current version · v{item.current_version}</p><p className="text-xs text-muted-foreground">Updated {formatDate(item.updated_at)}</p></div><Button variant="outline" size="sm" disabled={item.current_version <= 1 || busy} onClick={async () => { setBusy(true); try { await api.rollback(item, item.current_version - 1); toast.success("Rollback created a new immutable version"); await refresh(); } catch (error) { toast.error(error instanceof Error ? error.message : "Rollback failed"); } finally { setBusy(false); } }}><RotateCcw className="size-4" />Rollback to v{item.current_version - 1}</Button></div>{(proposals.data ?? []).map((proposal) => <div key={proposal.id} className="flex items-center justify-between rounded-xl border border-border p-4"><div><p className="font-medium">{proposal.summary}</p><p className="text-xs text-muted-foreground">Base v{proposal.base_version} · {formatDate(proposal.created_at)}</p></div><StatusBadge status={proposal.status} /></div>)}</CardContent></Card>}
    {tab === "Executions" && <div className="grid gap-6 xl:grid-cols-[1fr_.65fr]"><Card><CardHeader><CardTitle>Recent executions</CardTitle><CardDescription>Runs that used this Skill</CardDescription></CardHeader><CardContent><ExecutionFeed skillId={item.id} live /></CardContent></Card><Card><CardHeader><CardTitle>Success by model</CardTitle><CardDescription>{stats.data?.runs ?? 0} total runs</CardDescription></CardHeader><CardContent><MetricsChart data={(stats.data?.by_model ?? []).map((model) => ({ name: model.model || model.provider, value: Math.round(model.success_rate * (model.success_rate <= 1 ? 100 : 1)) }))} valueLabel="Success %" /></CardContent></Card></div>}
  </div>;
}

function Info({ label, value, pre = false }: { label: string; value: string; pre?: boolean }) { return <div><p className="field-label">{label}</p><p className={pre ? "whitespace-pre-wrap rounded-xl bg-muted p-4 text-sm leading-6" : "text-sm leading-6"}>{value}</p></div>; }
function Tags({ label, values }: { label: string; values: string[] }) { return <div><p className="field-label">{label}</p><div className="flex flex-wrap gap-1.5">{values.length ? values.map((value) => <Badge key={value}>{value}</Badge>) : <span className="text-sm text-muted-foreground">—</span>}</div></div>; }
