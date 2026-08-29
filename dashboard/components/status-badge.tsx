import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const colors: Record<string, string> = {
  active: "border-emerald-500/20 bg-emerald-500/10 text-emerald-600",
  success: "border-emerald-500/20 bg-emerald-500/10 text-emerald-600",
  draft: "border-amber-500/20 bg-amber-500/10 text-amber-600",
  running: "border-cyan-500/20 bg-cyan-500/10 text-cyan-600",
  failed: "border-red-500/20 bg-red-500/10 text-red-600",
  error: "border-red-500/20 bg-red-500/10 text-red-600",
  deprecated: "border-slate-500/20 bg-slate-500/10 text-slate-500",
  archived: "border-slate-500/20 bg-slate-500/10 text-slate-500"
};
export function StatusBadge({ status }: { status: string }) { return <Badge className={cn("capitalize", colors[status] ?? "")}>{status}</Badge>; }
