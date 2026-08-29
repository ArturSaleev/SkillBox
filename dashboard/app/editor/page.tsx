"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { SkillForm } from "@/components/skill-form";
import { Skeleton } from "@/components/ui/skeleton";

function EditorContent() {
  const id = useSearchParams().get("id") ?? undefined;
  return <div className="page-shell"><section><p className="eyebrow">Authoring</p><h1 className="page-heading mt-2">{id ? "Edit Skill" : "Create a Skill"}</h1><p className="mt-2 text-sm text-muted-foreground">Autosaved locally. Press Ctrl/⌘ + S to persist a draft in SkillBox.</p></section><SkillForm skillId={id} /></div>;
}

export default function EditorPage() {
  return <Suspense fallback={<div className="page-shell"><Skeleton className="h-64" /></div>}><EditorContent /></Suspense>;
}
