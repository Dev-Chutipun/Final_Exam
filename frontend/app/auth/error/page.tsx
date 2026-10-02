import type { Metadata } from "next";
import Link from "next/link";

import { Button } from "@/components/ui/button";

export const metadata: Metadata = { title: "เข้าสู่ระบบไม่สำเร็จ" };

// ข้อความตาม code ที่ Go ส่งมาจาก /api/auth/callback (02 §2.2)
const messages: Record<string, string> = {
  cancelled: "คุณยกเลิกการเข้าสู่ระบบ หรือผู้ให้บริการปฏิเสธคำขอ",
  login_expired: "หน้าเข้าสู่ระบบหมดอายุแล้ว (เกิน 10 นาที) กรุณาเริ่มใหม่",
  invalid_state: "คำขอเข้าสู่ระบบไม่ถูกต้อง อาจเปิดลิงก์เก่าหรือเปิดหลายแท็บพร้อมกัน",
  verification_failed: "ตรวจสอบตัวตนไม่สำเร็จ กรุณาลองใหม่อีกครั้ง",
};

export default async function AuthErrorPage({ searchParams }: { searchParams: Promise<{ code?: string }> }) {
  const { code } = await searchParams;
  const message = messages[code ?? ""] ?? "เข้าสู่ระบบไม่สำเร็จ กรุณาลองใหม่อีกครั้ง";

  return (
    <main className="mx-auto max-w-xl px-6 py-16">
      <section className="glass flex flex-col gap-4 rounded-(--r-panel) p-8">
        <h1 className="text-3xl font-semibold">เข้าสู่ระบบไม่สำเร็จ</h1>
        <p className="text-(--ink-body)">{message}</p>
        <div className="flex flex-wrap gap-3">
          <Button asChild variant="cta" size="lg">
            <a href="/api/auth/login?return_to=/">เข้าสู่ระบบอีกครั้ง</a>
          </Button>
          <Button asChild variant="glass" size="lg">
            <Link href="/">กลับหน้าแรก</Link>
          </Button>
        </div>
      </section>
    </main>
  );
}
