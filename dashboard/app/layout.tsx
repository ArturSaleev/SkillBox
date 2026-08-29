import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "@/app/globals.css";
import { Providers } from "@/components/providers";
import { Sidebar } from "@/components/sidebar";
import { Header } from "@/components/header";

const inter = Inter({ subsets: ["latin", "cyrillic"], variable: "--font-inter" });
export const metadata: Metadata = { title: { default: "SkillBox Dashboard", template: "%s · SkillBox" }, description: "Manage reusable AI procedures, releases and execution quality." };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return <html lang="ru" suppressHydrationWarning><body className={`${inter.variable} min-h-screen font-sans antialiased`}><Providers><Sidebar /><div className="transition-[padding] lg:pl-64"><Header /><main>{children}</main></div></Providers></body></html>;
}
