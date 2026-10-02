import type { Metadata } from "next";

import { SiteHeader } from "@/components/site-header";
import { Toaster } from "@/components/ui/sonner";

import { plex, prompt } from "./fonts";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "จองโต๊ะ", template: "%s · จองโต๊ะ" },
  description: "จองโต๊ะร้านอาหารและรีวิวร้าน",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="th" className={`${plex.variable} ${prompt.variable}`}>
      <body>
        <SiteHeader />
        {children}
        <Toaster position="top-center" />
      </body>
    </html>
  );
}
