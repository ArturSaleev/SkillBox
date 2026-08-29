"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { type ColumnDef, flexRender, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, type SortingState, useReactTable } from "@tanstack/react-table";
import { ArrowUpDown, ChevronLeft, ChevronRight, Eye, Pencil } from "lucide-react";
import type { Skill } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { StatusBadge } from "@/components/status-badge";
import { Badge } from "@/components/ui/badge";

export function SkillTable({ skills, query = "" }: { skills: Skill[]; query?: string }) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const columns = useMemo<ColumnDef<Skill>[]>(() => [
    { accessorKey: "name", header: ({ column }) => <Button variant="ghost" size="sm" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>Name <ArrowUpDown className="size-3" /></Button>, cell: ({ row }) => <div><Link className="font-semibold hover:text-primary" href={`/skills/view?id=${encodeURIComponent(row.original.id)}`}>{row.original.name}</Link><p className="mt-0.5 max-w-xs truncate text-xs text-muted-foreground">{row.original.description}</p></div> },
    { id: "project", accessorFn: (row) => row.project?.name ?? "Global", header: "Project", cell: ({ row }) => <div><p className="font-medium">{row.original.project?.name ?? "All projects"}</p><p className="text-xs text-muted-foreground">{row.original.project?.external_id || row.original.project?.slug || "global"}</p></div> },
    { id: "domain", accessorFn: (row) => row.domains.join(", "), header: "Domain", cell: ({ row }) => <div className="flex max-w-48 flex-wrap gap-1">{row.original.domains.slice(0, 2).map((domain) => <Badge key={domain}>{domain}</Badge>)}</div> },
    { id: "intent", accessorFn: (row) => row.intents.join(", "), header: "Intent", cell: ({ row }) => row.original.intents[0] ?? "—" },
    { accessorKey: "status", header: "Status", cell: ({ row }) => <StatusBadge status={row.original.status} /> },
    { accessorKey: "scope", header: "Scope" },
    { accessorKey: "current_version", header: "Version", cell: ({ row }) => `v${row.original.current_version}` },
    { id: "actions", header: "", cell: ({ row }) => <div className="flex justify-end gap-1"><Link className={cn("inline-flex size-10 items-center justify-center rounded-xl hover:bg-muted")} title="View" href={`/skills/view?id=${encodeURIComponent(row.original.id)}`}><Eye className="size-4" /></Link><Link className={cn("inline-flex size-10 items-center justify-center rounded-xl hover:bg-muted")} title="Edit" href={`/editor?id=${encodeURIComponent(row.original.id)}`}><Pencil className="size-4" /></Link></div> }
  ], []);
  const table = useReactTable({ data: skills, columns, state: { sorting, globalFilter: query }, onSortingChange: setSorting, getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel(), getFilteredRowModel: getFilteredRowModel(), getPaginationRowModel: getPaginationRowModel(), initialState: { pagination: { pageSize: 10 } } });
  return <div><Table><TableHeader>{table.getHeaderGroups().map((group) => <TableRow key={group.id}>{group.headers.map((header) => <TableHead key={header.id}>{header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}</TableHead>)}</TableRow>)}</TableHeader><TableBody>{table.getRowModel().rows.map((row) => <TableRow key={row.id}>{row.getVisibleCells().map((cell) => <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>)}</TableRow>)}</TableBody></Table>
    <div className="flex items-center justify-between border-t border-border px-4 py-3 text-xs text-muted-foreground"><span>{table.getFilteredRowModel().rows.length} Skills</span><div className="flex items-center gap-2"><span>Page {table.getState().pagination.pageIndex + 1} of {table.getPageCount() || 1}</span><Button variant="outline" size="icon" disabled={!table.getCanPreviousPage()} onClick={() => table.previousPage()}><ChevronLeft className="size-4" /></Button><Button variant="outline" size="icon" disabled={!table.getCanNextPage()} onClick={() => table.nextPage()}><ChevronRight className="size-4" /></Button></div></div>
  </div>;
}
