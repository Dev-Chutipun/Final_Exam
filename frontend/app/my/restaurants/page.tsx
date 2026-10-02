// ร้านของฉัน (อาร์ตบอร์ด 5) — Server Component
import type { Metadata } from "next";
import Link from "next/link";

import { CoverImage } from "@/components/cover-image";
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiGet, requireMe } from "@/lib/api-server";
import type { MyRestaurant } from "@/lib/types";

export const metadata: Metadata = { title: "ร้านของฉัน" };

export default async function MyRestaurantsPage() {
  await requireMe("/my/restaurants");
  const list = await apiGet<MyRestaurant[]>("/api/me/restaurants");

  return (
    <main className="mx-auto max-w-[1000px] px-4 pt-12 pb-18 sm:px-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[40px] font-semibold">ร้านของฉัน</h1>
          <p className="mt-1.5 text-(--ink-2)">บัญชีเดียวเป็นได้ทั้งลูกค้าและเจ้าของร้าน จัดการได้เฉพาะร้านที่คุณสร้าง</p>
        </div>
        <Button asChild variant="cta" size="lg">
          <Link href="/my/restaurants/new">+ สร้างร้านใหม่</Link>
        </Button>
      </div>

      {list.length === 0 ? (
        <section className="glass mt-8 flex flex-col items-center gap-3 rounded-(--r-card) p-10 text-center">
          <h2 className="text-xl font-semibold">คุณยังไม่มีร้าน</h2>
          <p className="text-(--ink-2)">สร้างร้านแรกของคุณ ใส่รูป เวลาเปิด–ปิด และจำนวนที่นั่ง แล้วลูกค้าจะเริ่มจองได้ทันที</p>
        </section>
      ) : (
        <ul className="mt-8 flex flex-col gap-5">
          {list.map((r) => (
            <li key={r.id} className="glass flex flex-col gap-5 rounded-(--r-panel) p-4 sm:flex-row">
              <CoverImage restaurantId={r.id} imageId={r.cover_image_id} alt={`ภาพปกร้าน ${r.name}`} className="h-44 shrink-0 rounded-[22px] sm:w-[220px]" />
              <div className="flex grow flex-col gap-3 py-1.5">
                <div className="flex flex-wrap items-center gap-2.5">
                  <h2 className="text-2xl font-semibold">{r.name}</h2>
                  {r.accepting_bookings ? <Pill tone="mint">เปิดรับจอง</Pill> : <Pill tone="gray">พักรับจอง</Pill>}
                </div>
                <div className="flex flex-wrap gap-2.5">
                  <Stat label="ที่นั่ง" tone="bg-(--st-lav-bg)/75 text-(--st-lav-ink)">{r.capacity} ที่</Stat>
                  <Stat label="เวลาเปิด" tone="bg-(--st-mint-bg)/75 text-(--st-mint-ink)">
                    {r.open_time}–{r.close_time}
                  </Stat>
                  <Stat label="คะแนน" tone="bg-(--st-butter-bg)/80 text-(--st-butter-ink)">
                    {r.avg_rating === null ? "ยังไม่มีรีวิว" : `${r.avg_rating.toFixed(1)} · ${r.review_count} รีวิว`}
                  </Stat>
                  <Stat label="การจองล่วงหน้า" tone="bg-(--st-peach-bg)/80 text-(--st-peach-ink)">
                    {r.upcoming_count === 0 ? "ไม่มี" : `${r.upcoming_count} รายการ`}
                  </Stat>
                </div>
                <div className="flex flex-wrap gap-2.5">
                  <Button asChild>
                    <Link href={`/my/restaurants/${r.id}`}>จัดการร้าน</Link>
                  </Button>
                  <Button asChild variant="glass">
                    <Link href={`/my/restaurants/${r.id}/bookings`}>ดูการจอง</Link>
                  </Button>
                  <Button asChild variant="glass">
                    <Link href={`/restaurants/${r.id}`}>ดูหน้าร้าน</Link>
                  </Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}

function Stat({ label, tone, children }: { label: string; tone: string; children: React.ReactNode }) {
  return (
    <div className={`flex min-w-[124px] flex-col rounded-[18px] border border-white/95 px-3.5 py-2.5 ${tone}`}>
      <span className="text-xs">{label}</span>
      <span className="font-heading text-[17px] font-semibold text-(--ink)">{children}</span>
    </div>
  );
}
