import * as React from "react";
import { cn } from "@/lib/utils";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(({ className, ...props }, ref) => <input ref={ref} className={cn("h-10 w-full rounded-xl border border-border bg-card px-3 text-sm outline-none placeholder:text-muted-foreground focus:ring-2 focus:ring-primary/30", className)} {...props} />);
Input.displayName = "Input";
