"use client";

import Link from "next/link";

import { Button } from "@/components/ui/button";

export default function RestaurantError({ reset }: { error: Error; reset: () => void }) {
  return (
    <main className="mx-auto max-w-xl px-6 py-16">
      <section role="alert" className="glass flex flex-col gap-4 rounded-(--r-panel) p-8">
        <h1 className="text-3xl font-semibold">โหลดข้อมูลร้านไม่สำเร็จ</h1>
        <p className="text-(--ink-body)">ลองใหม่อีกครั้ง หรือกลับไปเลือกร้านอื่น</p>
        <div className="flex gap-3">
          <Button onClick={reset}>ลองใหม่</Button>
          <Button asChild variant="glass">
            <Link href="/">กลับไปดูร้านอาหาร</Link>
          </Button>
        </div>
      </section>
    </main>
  );
}
