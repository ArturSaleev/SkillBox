import * as React from "react";
import { cn } from "@/lib/utils";

export const Select = React.forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(({ className, ...props }, ref) => <select ref={ref} className={cn("h-10 rounded-xl border border-border bg-card px-3 text-sm outline-none focus:ring-2 focus:ring-primary/30", className)} {...props} />);
Select.displayName = "Select";
