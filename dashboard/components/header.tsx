"use client";

import { Command, Moon, Search, Sun } from "lucide-react";
import { useTheme } from "next-themes";
import { useEffect, useRef } from "react";
import { useDashboardStore } from "@/lib/store";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

export function Header() {
  const query = useDashboardStore((state) => state.searchQuery);
  const setQuery = useDashboardStore((state) => state.setSearchQuery);
  const inputRef = useRef<HTMLInputElement>(null);
  const { resolvedTheme, setTheme } = useTheme();
  useEffect(() => {
    const handler = (event: KeyboardEvent) => { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") { event.preventDefault(); inputRef.current?.focus(); } };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);
  return <header className="sticky top-0 z-20 flex h-20 items-center justify-between gap-4 border-b border-border bg-background/85 px-4 backdrop-blur-xl md:px-8">
    <div className="relative w-full max-w-xl"><Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" /><Input ref={inputRef} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search Skills…" className="pl-10 pr-20" aria-label="Global Skill search" /><span className="absolute right-3 top-1/2 flex -translate-y-1/2 items-center gap-1 rounded-md border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground"><Command className="size-3" />K</span></div>
    <div className="flex items-center gap-2"><span className="hidden rounded-full bg-emerald-500/10 px-3 py-1.5 text-xs font-medium text-emerald-600 sm:block">Teacher · dashboard</span><Button variant="outline" size="icon" onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")} aria-label="Toggle theme"><Moon className="size-4 dark:hidden" /><Sun className="hidden size-4 dark:block" /></Button></div>
  </header>;
}
