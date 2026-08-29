import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva("inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-xl text-sm font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none disabled:opacity-50", {
  variants: {
    variant: {
      default: "bg-primary text-primary-foreground shadow-sm hover:brightness-110",
      secondary: "bg-muted text-foreground hover:bg-border",
      outline: "border border-border bg-card hover:bg-muted",
      ghost: "hover:bg-muted",
      destructive: "bg-destructive text-white hover:brightness-110"
    },
    size: { default: "h-10 px-4", sm: "h-8 rounded-lg px-3 text-xs", icon: "size-10 p-0" }
  },
  defaultVariants: { variant: "default", size: "default" }
});

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {}
export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(({ className, variant, size, ...props }, ref) => <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />);
Button.displayName = "Button";
