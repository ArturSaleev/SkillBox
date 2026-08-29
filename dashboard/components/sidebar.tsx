"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Activity, BarChart3, BookOpen, ChevronLeft, FilePenLine, LayoutDashboard, Sparkles } from "lucide-react";
import { cn } from "@/lib/utils";
import { useDashboardStore } from "@/lib/store";
import { Button } from "@/components/ui/button";

const links = [
  { href: "/", label: "Overview", icon: LayoutDashboard },
  { href: "/skills", label: "Skills Library", icon: BookOpen },
  { href: "/editor", label: "Skill Editor", icon: FilePenLine },
  { href: "/executions", label: "Executions", icon: Activity },
  { href: "/analytics", label: "Analytics", icon: BarChart3 }
];

export function Sidebar() {
  const pathname = usePathname();
  const open = useDashboardStore((state) => state.sidebarOpen);
  const setOpen = useDashboardStore((state) => state.setSidebarOpen);
  return <aside className={cn("fixed inset-y-0 left-0 z-30 hidden border-r border-border bg-[#101a2a] text-slate-100 transition-all lg:flex lg:flex-col", open ? "w-64" : "w-20")}>
    <div className="flex h-20 items-center gap-3 px-5">
      <div className="grid size-10 shrink-0 place-items-center rounded-xl bg-cyan-300 text-slate-950"><Sparkles className="size-5" /></div>
      {open && <div><p className="font-semibold">SkillBox</p><p className="text-xs text-slate-400">Procedure Control</p></div>}
    </div>
    <nav className="flex-1 space-y-1 px-3 py-4">
      {links.map(({ href, label, icon: Icon }) => {
        const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
        return <Link key={href} href={href} title={label} className={cn("flex h-11 items-center gap-3 rounded-xl px-3 text-sm text-slate-400 transition hover:bg-white/5 hover:text-white", active && "bg-cyan-300/10 text-cyan-200")}><Icon className="size-5 shrink-0" />{open && <span>{label}</span>}</Link>;
      })}
    </nav>
    <div className="p-3"><Button variant="ghost" className="w-full text-slate-400 hover:bg-white/5 hover:text-white" onClick={() => setOpen(!open)}><ChevronLeft className={cn("size-4 transition", !open && "rotate-180")} />{open && "Collapse"}</Button></div>
  </aside>;
}
