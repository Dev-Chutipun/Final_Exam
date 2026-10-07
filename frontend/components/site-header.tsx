// เมนูกระจกด้านบน (Server Component): อ่านผู้ใช้ปัจจุบันจาก /api/me แล้วแสดงปุ่ม login หรือชื่อ + logout
import Link from "next/link";
import { Suspense } from "react";

import { LoginLink } from "@/components/login-link";
import { NavLinks } from "@/components/nav-links";
import { Button } from "@/components/ui/button";
import { getMe } from "@/lib/api-server";

export async function SiteHeader() {
  // header อยู่ใน root layout: ถ้าเรียก API ไม่ได้ (Go ไม่ได้รัน/ล่ม) อย่าให้ทั้งเว็บพัง
  // แสดงเหมือนยังไม่ login พร้อมป้ายเตือน แล้วให้แต่ละหน้าแสดง error.tsx ของตัวเอง
  let me: Awaited<ReturnType<typeof getMe>> = null;
  let apiDown = false;
  try {
    me = await getMe();
  } catch {
    apiDown = true;
  }

  return (
    <div className="px-4 pt-4 sm:px-6">
      <header className="glass mx-auto flex min-h-[68px] max-w-[1232px] flex-wrap items-center gap-x-6 gap-y-2 rounded-[22px] px-3 py-2 sm:pl-[18px]">
        <Link href="/" className="font-heading flex items-center gap-2.5 text-[21px] font-bold">
          <span
            aria-hidden="true"
            className="flex size-[38px] items-center justify-center rounded-[13px] bg-linear-135 from-(--btn-cta-to) to-(--btn-primary-to) shadow-[inset_0_1px_0_rgba(255,255,255,0.8),0_6px_14px_rgba(107,91,214,0.2)]"
          >
            <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M7 3v8M5 3v5a2 2 0 0 0 4 0V3M7 11v10M17 3c-2 1.5-3 4-3 7 0 2 1 3 3 3v8" />
            </svg>
          </span>
          จองโต๊ะ
        </Link>

        <NavLinks loggedIn={me !== null} />

        {apiDown && (
          <span role="status" className="rounded-full bg-(--st-butter-bg) px-3 py-1 text-[13px] font-semibold text-(--st-butter-ink)">
            เชื่อมต่อเซิร์ฟเวอร์ไม่ได้
          </span>
        )}

        {me ? (
          <div className="flex items-center gap-2.5">
            <span
              aria-hidden="true"
              className="font-heading flex size-[38px] items-center justify-center rounded-full border border-white bg-linear-135 from-(--btn-primary-from) to-(--btn-primary-to) font-semibold"
            >
              {me.display_name.charAt(0)}
            </span>
            <span className="max-w-40 truncate font-semibold">{me.display_name}</span>
            {/* logout เป็นฟอร์ม POST ธรรมดา (เว็บอื่นสั่ง logout ด้วย <img> ไม่ได้) */}
            <form method="POST" action="/api/auth/logout">
              <Button type="submit" variant="glass" size="sm">
                ออกจากระบบ
              </Button>
            </form>
          </div>
        ) : (
          <Suspense fallback={null}>
            <LoginLink />
          </Suspense>
        )}
      </header>
    </div>
  );
}
