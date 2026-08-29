"use client";

import { Bar, BarChart, CartesianGrid, Cell, Line, LineChart, Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { EmptyState } from "@/components/data-state";

export interface ChartPoint { name: string; value: number; secondary?: number }
const colors = ["#0ea5a8", "#f59e0b", "#6366f1", "#f43f5e", "#22c55e"];

export function MetricsChart({ data, type = "bar", valueLabel = "Value" }: { data: ChartPoint[]; type?: "bar" | "line" | "pie"; valueLabel?: string }) {
  if (!data.length) return <EmptyState title="Нет метрик" description="График появится после первых execution-событий." />;
  return <div className="h-72 w-full"><ResponsiveContainer width="100%" height="100%">
    {type === "pie" ? <PieChart><Pie data={data} dataKey="value" nameKey="name" innerRadius={58} outerRadius={92} paddingAngle={4}>{data.map((item, index) => <Cell key={item.name} fill={colors[index % colors.length]} />)}</Pie><Tooltip /></PieChart> : type === "line" ? <LineChart data={data} margin={{ left: -16, right: 12 }}><CartesianGrid strokeDasharray="4 4" vertical={false} opacity={0.2} /><XAxis dataKey="name" tickLine={false} axisLine={false} fontSize={11} /><YAxis tickLine={false} axisLine={false} fontSize={11} /><Tooltip /><Line type="monotone" dataKey="value" name={valueLabel} stroke="#0ea5a8" strokeWidth={3} dot={{ r: 3 }} /></LineChart> : <BarChart data={data} margin={{ left: -16, right: 12 }}><CartesianGrid strokeDasharray="4 4" vertical={false} opacity={0.2} /><XAxis dataKey="name" tickLine={false} axisLine={false} fontSize={11} /><YAxis tickLine={false} axisLine={false} fontSize={11} /><Tooltip /><Bar dataKey="value" name={valueLabel} fill="#0ea5a8" radius={[8, 8, 0, 0]} /></BarChart>}
  </ResponsiveContainer></div>;
}
