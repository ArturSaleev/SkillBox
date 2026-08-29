import { AlertTriangle, Inbox } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function LoadingGrid() { return <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">{Array.from({ length: 4 }, (_, index) => <Skeleton key={index} className="h-32" />)}</div>; }
export function ErrorState({ error }: { error: Error }) { return <Card className="border-amber-500/30 bg-amber-500/5"><CardContent className="flex items-start gap-3 pt-5"><AlertTriangle className="mt-0.5 size-5 text-amber-500" /><div><p className="font-semibold">Не удалось подключиться к SkillBox</p><p className="mt-1 text-sm text-muted-foreground">{error.message}. Убедитесь, что Go-сервис запущен на адресе из `.env.local`.</p></div></CardContent></Card>; }
export function EmptyState({ title = "Пока нет данных", description = "Создайте первый Skill, чтобы наполнить этот раздел." }: { title?: string; description?: string }) { return <div className="grid min-h-48 place-items-center rounded-2xl border border-dashed border-border p-8 text-center"><div><Inbox className="mx-auto mb-3 size-8 text-muted-foreground" /><p className="font-semibold">{title}</p><p className="mt-1 max-w-md text-sm text-muted-foreground">{description}</p></div></div>; }
