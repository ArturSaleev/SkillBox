import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatDate(value?: string) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("ru-RU", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

export function formatDuration(value?: number) {
  if (value === undefined) return "—";
  return value < 1000 ? `${value} ms` : `${(value / 1000).toFixed(1)} s`;
}

export function percent(value?: number) {
  return `${Math.round((value ?? 0) * (value && value > 1 ? 1 : 100))}%`;
}
