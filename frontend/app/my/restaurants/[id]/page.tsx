// จัดการร้าน (อาร์ตบอร์ด 7) — Server Component อ่านข้อมูล ส่วนโต้ตอบอยู่ใน manage-sections.tsx
// ตรวจ is_owner เพื่อซ่อนหน้า (UX) สิทธิ์จริงตรวจที่ Go ทุก endpoint
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiGet, apiGetOrNull, requireMe } from "@/lib/api-server";
import type { MyRestaurant, RestaurantDetail } from "@/lib/types";

import { AcceptingSwitch, DeleteRestaurant, ImagesManager, SettingsForm } from "./manage-sections";

export const metadata: Metadata = { title: "จัดการร้าน" };

export default async function ManagePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  await requireMe(`/my/restaurants/${id}`);
  const r = /^\d+$/.test(id) ? await apiGetOrNull<RestaurantDetail>(`/api/restaurants/${id}`) : null;
  if (!r || !r.is_owner) notFound();
  const mine = await apiGet<MyRestaurant[]>("/api/me/restaurants");
  const upcoming = mine.find((m) => m.id === r.id)?.upcoming_count ?? 0;

  return (
    <main className="mx-auto max-w-(--w-list) px-4 pt-9 pb-18 sm:px-6">
      <nav aria-label="เส้นทาง" className="flex gap-2 text-sm text-(--ink-2)">
        <Link href="/my/restaurants" className="hover:underline">
          ร้านของฉัน
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-semibold text-(--ink)">{r.name}</span>
      </nav>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-[40px] font-semibold">{r.name}</h1>
        <Button asChild variant="glass">
          <Link href={`/restaurants/${r.id}`}>ดูหน้าร้าน</Link>
        </Button>
      </div>

      <div className="mt-5.5 grid grid-cols-2 gap-3.5 sm:grid-cols-4">
        <Stat label="สถานะ" tone={r.accepting_bookings ? "bg-(--st-mint-bg) text-(--st-mint-ink)" : "bg-(--st-gray-bg) text-(--st-gray-ink)"}>
          {r.accepting_bookings ? "เปิดรับจอง" : "พักรับจอง"}
        </Stat>
        <Stat label="การจองล่วงหน้า" tone="bg-(--st-peach-bg)/80 text-(--st-peach-ink)">
          {upcoming} รายการ
        </Stat>
        <Stat label="ที่นั่ง" tone="bg-(--st-lav-bg)/80 text-(--st-lav-ink)">
          {r.capacity} ที่
        </Stat>
        <Stat label="คะแนน" tone="bg-(--st-butter-bg)/85 text-(--st-butter-ink)">
          {r.avg_rating === null ? "ยังไม่มีรีวิว" : `${r.avg_rating.toFixed(1)} · ${r.review_count} รีวิว`}
        </Stat>
      </div>

      <div className="mt-5.5 flex flex-col gap-5.5">
        <Section className="sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-col gap-1">
            <h2 className="text-[21px] font-semibold">การรับจอง</h2>
            <p className="text-sm text-(--ink-2)">ปิดรับเพื่อไม่ให้มีการจองใหม่หรือการแก้ไขการจอง ลูกค้ายังยกเลิกการจองเดิมได้ตามปกติ</p>
          </div>
          <AcceptingSwitch restaurantId={r.id} accepting={r.accepting_bookings} />
        </Section>

        <Section>
          <h2 className="text-[21px] font-semibold">ตั้งค่าการจอง</h2>
          {/* key เปลี่ยนตามค่าที่บันทึกแล้ว ฟอร์มจึงรีเซ็ตเป็นค่าใหม่หลัง router.refresh() */}
          <SettingsForm key={`${r.capacity}-${r.open_time}-${r.close_time}-${r.cancel_window_minutes}-${r.min_lead_minutes}`} r={r} />
        </Section>

        <Section>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-[21px] font-semibold">ข้อมูลร้าน</h2>
            <Button asChild variant="glass">
              <Link href={`/my/restaurants/${r.id}/edit`}>แก้ไขข้อมูลร้าน</Link>
            </Button>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {r.categories.map((c) => (
              <Pill key={c.id} tone="lav">
                {c.name_th}
              </Pill>
            ))}
          </div>
          <p className="break-words whitespace-pre-line text-(--ink-body)">{r.description}</p>
        </Section>

        <Section>
          <div className="flex items-baseline justify-between gap-3">
            <h2 className="text-[21px] font-semibold">รูปร้าน</h2>
            <Pill tone={r.image_ids.length >= 3 ? "butter" : "lav"}>ใช้แล้ว {r.image_ids.length} จาก 3 รูป</Pill>
          </div>
          <ImagesManager restaurantId={r.id} imageIds={r.image_ids} />
        </Section>

        <Section className="sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-col gap-1">
            <h2 className="text-[21px] font-semibold">การจองรายวัน</h2>
            <p className="text-sm text-(--ink-2)">ดูรายการจองของลูกค้าในแต่ละวันทำการ และจำนวนที่นั่งที่ใช้พร้อมกันสูงสุด</p>
          </div>
          <Button asChild variant="glass" className="shrink-0">
            <Link href={`/my/restaurants/${r.id}/bookings`}>ดูการจอง</Link>
          </Button>
        </Section>

        <section className="flex flex-col gap-4 rounded-(--r-card) border border-(--st-pink-border) bg-(--st-pink-bg)/55 p-6 backdrop-blur-[20px] sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-col gap-1">
            <h2 className="text-[21px] font-semibold text-(--btn-danger-ink)">ลบร้าน</h2>
            <p className="text-sm text-(--ink-2)">ลบได้เมื่อไม่มีการจองล่วงหน้า ร้านจะถูกซ่อนจากทุกคน ประวัติการจองที่ผ่านมาของลูกค้ายังอยู่</p>
          </div>
          <DeleteRestaurant r={r} />
        </section>
      </div>
    </main>
  );
}

function Section({ className = "", children }: { className?: string; children: React.ReactNode }) {
  return <section className={`glass flex flex-col gap-4 rounded-(--r-card) p-6 ${className}`}>{children}</section>;
}

function Stat({ label, tone, children }: { label: string; tone: string; children: React.ReactNode }) {
  return (
    <div className={`flex flex-col gap-0.5 rounded-[22px] border border-white/95 px-4.5 py-4 ${tone}`}>
      <span className="text-[13px]">{label}</span>
      <span className="font-heading text-[22px] font-semibold text-(--ink)">{children}</span>
    </div>
  );
}
