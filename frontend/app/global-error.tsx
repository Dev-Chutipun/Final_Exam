"use client";

// error ที่เกิดใน root layout เอง (error.tsx รับไม่ได้) — แทนที่ทั้งหน้า จึงต้องมี <html> และ <body> เอง
import "./globals.css";

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="th">
      <body>
        <main className="mx-auto max-w-xl px-6 py-16">
          <section role="alert" className="glass flex flex-col gap-4 rounded-(--r-panel) p-8">
            <h1 className="text-3xl font-semibold">ระบบขัดข้องชั่วคราว</h1>
            <p className="text-(--ink-body)">โหลดหน้าเว็บไม่สำเร็จ อาจเชื่อมต่อเซิร์ฟเวอร์ไม่ได้ ลองใหม่อีกครั้งในอีกสักครู่</p>
            <div>
              <button
                type="button"
                onClick={reset}
                className="font-heading min-h-12 rounded-(--r-btn) border border-white/95 bg-linear-to-b from-(--btn-primary-from) to-(--btn-primary-to) px-[22px] font-semibold"
              >
                ลองใหม่
              </button>
            </div>
          </section>
        </main>
      </body>
    </html>
  );
}
