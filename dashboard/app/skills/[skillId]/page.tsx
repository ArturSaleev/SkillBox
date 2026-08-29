import { Suspense } from "react";
import { SkillDetails } from "@/components/skill-details";
import { Skeleton } from "@/components/ui/skeleton";

export function generateStaticParams() {
  return [{ skillId: "view" }];
}

export default function SkillDetailsPage() {
  return <Suspense fallback={<div className="page-shell"><Skeleton className="h-96" /></div>}><SkillDetails /></Suspense>;
}
