import Link from "next/link";

import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <main className="mx-auto max-w-xl px-6 py-16">
      <section className="glass flex flex-col gap-4 rounded-(--r-panel) p-8">
        <h1 className="text-3xl font-semibold">ไม่พบหน้านี้</h1>
        <p className="text-(--ink-body)">หน้าที่คุณหาอาจถูกลบไปแล้ว หรือคุณไม่มีสิทธิ์เข้าถึง</p>
        <Button asChild variant="glass" className="self-start">
          <Link href="/">กลับไปดูร้านอาหาร</Link>
        </Button>
      </section>
    </main>
  );
}
