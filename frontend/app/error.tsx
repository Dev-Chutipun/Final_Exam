"use client";

import Link from "next/link";

import { Button } from "@/components/ui/button";

// error boundary ต้องเป็น Client Component
export default function ErrorPage({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <main className="mx-auto max-w-xl px-6 py-16">
      <section role="alert" className="glass flex flex-col gap-4 rounded-(--r-panel) p-8">
        <h1 className="text-3xl font-semibold">เกิดข้อผิดพลาด</h1>
        <p className="text-(--ink-body)">โหลดหน้านี้ไม่สำเร็จ อาจเชื่อมต่อเซิร์ฟเวอร์ไม่ได้ชั่วคราว</p>
        <div className="flex gap-3">
          <Button onClick={reset}>ลองใหม่</Button>
          <Button asChild variant="glass">
            <Link href="/">กลับหน้าแรก</Link>
          </Button>
        </div>
      </section>
    </main>
  );
}
