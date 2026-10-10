import type { Metadata } from "next";

import { SiteHeader } from "@/components/site-header";
import { SuccessDialogProvider } from "@/components/success-dialog";
import { Toaster } from "@/components/ui/sonner";

import { plex, prompt } from "./fonts";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "กินนี่", template: "%s · กินนี่" },
  description: "กินนี่ — จองโต๊ะร้านอาหารและรีวิวร้าน",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="th" className={`${plex.variable} ${prompt.variable}`}>
      <body>
        <SiteHeader />
        <SuccessDialogProvider>{children}</SuccessDialogProvider>
        {/* แจ้งเตือนผลของทุกรายการ: สำเร็จเป็นพื้นมินต์ ผิดพลาดเป็นพื้นชมพู (สีจาก token ใน globals.css เท่านั้น)
            อยู่ 6 วินาทีและไม่ต้องกดปิด · ตั้งค่าผ่าน props ไม่แก้ไฟล์ใน components/ui/ */}
        <Toaster
          position="top-center"
          duration={6000}
          toastOptions={{
            classNames: {
              toast:
                "w-auto! min-w-[320px]! gap-3! rounded-[20px]! border! px-5! py-4! font-heading! text-[16px]! font-semibold! shadow-[0_18px_40px_rgba(75,62,160,0.26)]!",
              success: "border-(--st-mint-ink)/25! bg-(--st-mint-bg)! text-(--st-mint-ink)!",
              error: "border-(--st-pink-border)! bg-(--st-pink-bg)! text-(--st-pink-ink)!",
              icon: "size-6! [&>svg]:size-6!",
            },
          }}
        />
      </body>
    </html>
  );
}
