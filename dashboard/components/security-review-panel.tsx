"use client";

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Bot, CheckCircle2, ShieldAlert } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api-client";
import { useCodeReviewConfig, useSecurityReviews } from "@/lib/hooks";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState, ErrorState } from "@/components/data-state";
import { formatDate } from "@/lib/utils";

export function SecurityReviewPanel({ skillId, packageHash }: { skillId: string; packageHash?: string }) {
  const config = useCodeReviewConfig();
  const reviews = useSecurityReviews(skillId);
  const queryClient = useQueryClient();
  const [approved, setApproved] = useState(false);
  const [busy, setBusy] = useState(false);

  async function runReview() {
    setBusy(true);
    try {
      await api.runSecurityReview(skillId, approved);
      await queryClient.invalidateQueries({ queryKey: ["security-reviews", skillId] });
      setApproved(false);
      toast.success("AI security review completed");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Security review failed");
    } finally {
      setBusy(false);
    }
  }

  if (config.error || reviews.error) return <ErrorState error={(config.error ?? reviews.error) as Error} />;
  const provider = config.data;
  return <div className="space-y-6">
    <Card>
      <CardHeader><CardTitle>AI-assisted security review</CardTitle><CardDescription>Package files are reviewed as untrusted text and are never executed. AI findings supplement, but do not replace, manual review.</CardDescription></CardHeader>
      <CardContent className="space-y-4">
        {!packageHash && <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4 text-sm"><AlertTriangle className="mr-2 inline size-4" />This Skill has no filesystem package to review.</div>}
        {provider?.configured ? <>
          <div className="grid gap-2 rounded-xl border border-border p-4 text-sm md:grid-cols-4"><span><strong>Provider:</strong> {provider.provider}</span><span><strong>Model:</strong> {provider.model || "—"}</span><span className="break-all md:col-span-2"><strong>Endpoint:</strong> {provider.endpoint}</span></div>
          {provider.external && <label className="flex items-start gap-3 rounded-xl border border-amber-500/40 bg-amber-500/10 p-4 text-sm"><input className="mt-1" type="checkbox" checked={approved} onChange={(event) => setApproved(event.target.checked)} /><span><strong>Allow this transmission.</strong> I understand that all text files in this Skill package will be sent to the external endpoint shown above for this review only.</span></label>}
          <Button onClick={runReview} disabled={busy || !packageHash || Boolean(provider.external && !approved)}><Bot className="size-4" />{busy ? "Reviewing…" : "Run AI review"}</Button>
        </> : <div className="rounded-xl border border-border bg-muted/30 p-4 text-sm"><ShieldAlert className="mr-2 inline size-4" />AI review is not configured. Set <code>code_review</code> in <code>configs/skillbox.yaml</code>; keep API keys in the configured environment variable.</div>}
      </CardContent>
    </Card>
    <Card>
      <CardHeader><CardTitle>Review history</CardTitle><CardDescription>Each result is bound to the exact package SHA-256 and becomes outdated after package changes.</CardDescription></CardHeader>
      <CardContent className="space-y-4">
        {reviews.isLoading ? <p className="text-sm text-muted-foreground">Loading review history…</p> : !(reviews.data?.length) ? <EmptyState title="No AI reviews yet" /> : reviews.data.map((review) => <div key={review.id} className="space-y-3 rounded-xl border border-border p-4">
          <div className="flex flex-wrap items-start justify-between gap-2"><div><p className="font-semibold">{review.provider} · {review.model}</p><p className="text-xs text-muted-foreground">{formatDate(review.reviewed_at)} · <span className="font-mono">{review.package_hash.slice(0, 12)}</span></p></div>{review.outdated ? <Badge className="border-amber-500/40 text-amber-700 dark:text-amber-300">Outdated</Badge> : <Badge className="border-emerald-500/40 text-emerald-700 dark:text-emerald-300"><CheckCircle2 className="mr-1 size-3" />Current package</Badge>}</div>
          <p className="text-sm">{review.summary}</p>
          {(review.findings ?? []).map((finding, index) => <div key={`${review.id}:${index}`} className="rounded-lg bg-muted p-3 text-sm"><div className="flex flex-wrap gap-2"><Badge>{finding.severity}</Badge><strong>{finding.title}</strong>{finding.path && <span className="font-mono text-xs text-muted-foreground">{finding.path}{finding.line ? `:${finding.line}` : ""}</span>}</div><p className="mt-2 text-muted-foreground">{finding.explanation}</p><p className="mt-2"><strong>Recommendation:</strong> {finding.recommendation}</p></div>)}
          <p className="text-xs text-muted-foreground">{review.recommendation}</p>
        </div>)}
      </CardContent>
    </Card>
  </div>;
}
