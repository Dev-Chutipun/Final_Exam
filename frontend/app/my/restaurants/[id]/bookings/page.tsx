// [B] การจองรายวันของร้าน (อาร์ตบอร์ด 7 ส่วนการจองรายวัน) — Server Component
// วันทำการใช้หน้าต่างเปิดร้านเดียวกับ availability จึงรวมช่วงหลังเที่ยงคืนของร้านที่เปิดข้ามคืน
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { Pill } from "@/components/pill";
import { apiGet, apiGetOrNull, requireMe } from "@/lib/api-server";
import { addDays, formatDate, formatTime, formatTimeRange, localDate, todayInBangkok } from "@/lib/time";
import type { DailyBookings, RestaurantDetail } from "@/lib/types";

import { DatePicker } from "./date-picker";

export const metadata: Metadata = { title: "การจองรายวัน" };

export default async function DailyBookingsPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ date?: string }>;
}) {
  const { id } = await params;
  const sp = await searchParams;
  const date = /^\d{4}-\d{2}-\d{2}$/.test(sp.date ?? "") ? sp.date! : todayInBangkok();
  await requireMe(`/my/restaurants/${id}/bookings`);
  const r = /^\d+$/.test(id) ? await apiGetOrNull<RestaurantDetail>(`/api/restaurants/${id}`) : null;
  if (!r || !r.is_owner) notFound();
  const data = await apiGet<DailyBookings>(`/api/restaurants/${r.id}/bookings?date=${date}`);
  const active = data.items.filter((b) => b.status === "active");

  return (
    <main className="mx-auto max-w-[980px] px-4 pt-9 pb-18 sm:px-6">
      <nav aria-label="เส้นทาง" className="flex gap-2 text-sm text-(--ink-2)">
        <Link href="/my/restaurants" className="hover:underline">
          ร้านของฉัน
        </Link>
        <span aria-hidden="true">/</span>
        <Link href={`/my/restaurants/${r.id}`} className="hover:underline">
          {r.name}
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-semibold text-(--ink)">การจองรายวัน</span>
      </nav>

      <section className="glass mt-4 flex flex-col gap-4 rounded-(--r-card) p-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <h1 className="text-[28px] font-semibold">การจองรายวัน · {formatDate(`${date}T12:00:00+07:00`)}</h1>
          <div className="flex flex-wrap items-center gap-2">
            <Link href={`?date=${addDays(date, -1)}`} className="rounded-xl px-3 py-2 text-sm text-(--ink-2) hover:bg-white/60">
              ‹ วันก่อน
            </Link>
            <DatePicker value={date} />
            <Link href={`?date=${addDays(date, 1)}`} className="rounded-xl px-3 py-2 text-sm text-(--ink-2) hover:bg-white/60">
              วันถัดไป ›
            </Link>
          </div>
        </div>
        <p className="text-sm text-(--ink-2)">
          เวลาเปิด {r.open_time}–{r.close_time} น.
          {r.close_time < r.open_time && " (ร้านเปิดข้ามเที่ยงคืน รายการหลังเที่ยงคืนนับเป็นวันทำการนี้)"}
        </p>

        {data.items.length === 0 ? (
          <p className="row-plain rounded-[18px] px-4 py-6 text-center text-(--ink-2)">ไม่มีการจองในวันทำการนี้</p>
        ) : (
          <div className="flex flex-col gap-2">
            <div className="hidden grid-cols-4 gap-3 px-4 text-[13px] font-semibold text-(--ink-2) sm:grid">
              <span>เวลา</span>
              <span>ผู้จอง</span>
              <span>จำนวนคน</span>
              <span>สถานะ</span>
            </div>
            <ul className="flex flex-col gap-2">
              {data.items.map((b) => (
                <li key={b.id} className="row-plain grid grid-cols-2 items-center gap-3 rounded-[18px] px-4 py-3 sm:grid-cols-4">
                  <span className="font-heading font-semibold">
                    {formatTimeRange(b.start_at, b.end_at)}
                    {localDate(b.start_at) !== date && <span className="text-xs font-medium"> (วันถัดไป)</span>}
                  </span>
                  <span>{b.customer_display_name}</span>
                  <span>{b.party_size} คน</span>
                  <span>{b.status === "active" ? <Pill tone="mint">ยืนยันแล้ว</Pill> : <Pill tone="gray">ยกเลิกแล้ว</Pill>}</span>
                </li>
              ))}
            </ul>
          </div>
        )}

        <div className="rounded-[18px] bg-linear-135 from-(--st-lav-bg)/90 to-white/70 px-4 py-3 text-sm font-medium text-(--st-lav-ink)">
          รวม {data.items.length} รายการ (ยืนยันแล้ว {active.length})
          {data.peak_at
            ? ` · ใช้ที่นั่งพร้อมกันสูงสุด ${data.peak} จาก ${data.capacity} ที่ (เริ่ม ${formatTime(data.peak_at)} น.)`
            : ` · ที่นั่ง ${data.capacity} ที่`}
        </div>
      </section>
    </main>
  );
}
