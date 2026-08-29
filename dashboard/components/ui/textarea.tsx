import * as React from "react";
import { cn } from "@/lib/utils";

export const Textarea = React.forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(({ className, ...props }, ref) => <textarea ref={ref} className={cn("min-h-28 w-full rounded-xl border border-border bg-card px-3 py-2 text-sm outline-none placeholder:text-muted-foreground focus:ring-2 focus:ring-primary/30", className)} {...props} />);
Textarea.displayName = "Textarea";
