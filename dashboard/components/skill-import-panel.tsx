"use client";

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { AlertTriangle, FileArchive, GitBranch, PackageCheck } from "lucide-react";
import { api } from "@/lib/api-client";
import type { ImportPreview } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

export function SkillImportPanel() {
  const queryClient = useQueryClient();
  const [source, setSource] = useState<"zip" | "git">("zip");
  const [file, setFile] = useState<File>();
  const [url, setURL] = useState("");
  const [revision, setRevision] = useState("HEAD");
  const [preview, setPreview] = useState<ImportPreview>();
  const [busy, setBusy] = useState(false);

  async function loadPreview() {
    setBusy(true); setPreview(undefined);
    try {
      const next = source === "zip" ? (file ? await api.previewZIP(file) : (() => { throw new Error("Choose a ZIP file"); })()) : await api.previewGit(url, revision);
      setPreview(next);
    } catch (error) { toast.error(error instanceof Error ? error.message : "Preview failed"); }
    finally { setBusy(false); }
  }

  async function importPackage() {
    if (!preview) return;
    setBusy(true);
    try {
      if (source === "zip") { if (!file) throw new Error("Choose a ZIP file"); await api.importZIP(file); }
      else await api.importGit(url, revision);
      await queryClient.invalidateQueries({ queryKey: ["skills"] });
      toast.success(`${preview.name} imported`); setPreview(undefined);
    } catch (error) { toast.error(error instanceof Error ? error.message : "Import failed"); }
    finally { setBusy(false); }
  }

  return <Card><CardHeader><CardTitle>Import portable Skill</CardTitle><CardDescription>Preview a ZIP upload or Git repository before importing. Package code is never executed.</CardDescription></CardHeader><CardContent className="space-y-4"><Select value={source} onChange={(event) => { setSource(event.target.value as "zip" | "git"); setPreview(undefined); }}><option value="zip">ZIP archive</option><option value="git">Git repository</option></Select>{source === "zip" ? <label className="flex cursor-pointer items-center gap-3 rounded-xl border border-dashed border-border p-4 text-sm"><FileArchive className="size-5 text-primary" /><span className="flex-1 truncate">{file?.name ?? "Choose .zip file"}</span><input className="hidden" type="file" accept=".zip,application/zip" onChange={(event) => { setFile(event.target.files?.[0]); setPreview(undefined); }} /></label> : <div className="grid gap-3 md:grid-cols-[1fr_180px]"><div className="relative"><GitBranch className="absolute left-3 top-3 size-4 text-muted-foreground" /><Input className="pl-10" value={url} onChange={(event) => { setURL(event.target.value); setPreview(undefined); }} placeholder="https://host/team/skill.git" /></div><Input value={revision} onChange={(event) => { setRevision(event.target.value); setPreview(undefined); }} placeholder="HEAD, tag or branch" /></div>}<Button variant="outline" onClick={loadPreview} disabled={busy}>{busy ? "Validating…" : "Preview package"}</Button>{preview && <div className="space-y-4 rounded-xl border border-border bg-muted/30 p-4"><div className="flex items-start gap-3"><PackageCheck className="mt-0.5 size-5 text-primary" /><div><p className="font-semibold">{preview.name}</p><p className="text-sm text-muted-foreground">{preview.description}</p></div></div><div className="grid gap-2 text-xs sm:grid-cols-4"><span>{preview.files.length} files</span><span>{preview.scripts} scripts</span><span>{preview.references} references</span><span>{preview.assets} assets</span></div>{preview.executable_code.length > 0 && <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-sm"><div className="flex gap-2 font-semibold text-amber-700 dark:text-amber-300"><AlertTriangle className="size-4" />Executable code detected</div><div className="mt-2 space-y-1 font-mono text-xs">{preview.executable_code.map((finding) => <p key={finding.path}>{finding.path} — {finding.language || "unknown"} ({finding.reason})</p>)}</div></div>}<div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-sm"><div className="flex gap-2 font-semibold text-amber-700 dark:text-amber-300"><AlertTriangle className="size-4" />Manual security review recommended</div><p className="mt-1 text-muted-foreground">{preview.security_scan.recommendation}</p><p className="mt-2 text-xs">Capabilities: {preview.security_scan.capabilities.join(", ") || "none detected"}</p><div className="mt-2 max-h-40 space-y-1 overflow-auto font-mono text-xs">{preview.security_scan.findings.map((finding, index) => <p key={`${finding.path}:${finding.line}:${finding.rule}:${index}`}>[{finding.severity}] {finding.path}:{finding.line} — {finding.capability} / {finding.rule}</p>)}</div></div><div className="space-y-1 text-xs text-muted-foreground"><p className="break-all">Source: {preview.source.url}{preview.source.revision ? ` @ ${preview.source.revision}` : ""}</p><p className="break-all font-mono">SHA-256: {preview.package_hash}</p></div><div className="max-h-40 overflow-auto rounded-lg bg-card p-3 font-mono text-xs">{preview.files.map((item) => <p key={item.path}>{item.kind.padEnd(10)} {item.path}</p>)}</div><Button onClick={importPackage} disabled={busy}>{busy ? "Importing…" : "Import validated package"}</Button></div>}</CardContent></Card>;
}
