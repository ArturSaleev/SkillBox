"use client";

import Link from "next/link";
import type { Dependency, Skill } from "@/lib/types";
import { EmptyState } from "@/components/data-state";

export function DependencyGraph({ skill, dependencies }: { skill: Skill; dependencies: Dependency[] }) {
  if (!dependencies.length) return <EmptyState title="Нет зависимостей" description="Этот Skill можно выполнять независимо." />;
  return <div className="relative flex min-h-64 items-center justify-center overflow-auto rounded-2xl bg-muted/50 p-8"><div className="flex items-center gap-8"><div className="rounded-2xl border-2 border-primary bg-card px-5 py-4 text-center shadow-panel"><p className="font-semibold">{skill.name}</p><p className="text-xs text-muted-foreground">current</p></div><div className="h-px w-12 bg-primary" /><div className="space-y-3">{dependencies.map((dependency) => <Link key={dependency.id} href={`/skills/view?id=${encodeURIComponent(dependency.depends_on_skill_id)}`} className="block rounded-xl border border-border bg-card px-4 py-3 transition hover:border-primary"><p className="font-medium">{dependency.depends_on_skill_id}</p><p className="text-xs text-muted-foreground">{dependency.type}</p></Link>)}</div></div></div>;
}
