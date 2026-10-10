// การจองของฉัน (อาร์ตบอร์ด 4) — Server Component อ่านข้อมูล ปุ่มแก้ไข/ยกเลิกเป็น Client Component
// สถานะแท็บและหน้าเก็บใน URL · ใช้ is_past, can_modify, restaurant_deleted จากเซิร์ฟเวอร์เท่านั้น
import type { Metadata } from "next";
import Link from "next/link";

import { ClockIcon, PeopleIcon } from "@/components/icons";
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiGet, requireMe } from "@/lib/api-server";
import { dateTile, formatTime, formatTimeRange } from "@/lib/time";
import type { MyBooking, MyBookings } from "@/lib/types";

import { BookingActions } from "./booking-actions";

export const metadata: Metadata = { title: "การจองของฉัน" };

const TABS = [
  { key: "upcoming", label: "กำลังจะมาถึง" },
  { key: "past", label: "ผ่านไปแล้ว" },
  { key: "cancelled", label: "ยกเลิกแล้ว" },
] as const;
type Tab = (typeof TABS)[number]["key"];

const PAGE_SIZE = 10;

export default async function BookingsPage({ searchParams }: { searchParams: Promise<{ tab?: string; page?: string }> }) {
  const sp = await searchParams;
  const tab: Tab = TABS.some((t) => t.key === sp.tab) ? (sp.tab as Tab) : "upcoming";
  const page = Math.max(1, Number.parseInt(sp.page ?? "1", 10) || 1);
  await requireMe(`/bookings?tab=${tab}`);

  const data = await apiGet<MyBookings>(`/api/me/bookings?tab=${tab}&page=${page}&page_size=${PAGE_SIZE}`);
  const totalPages = Math.max(1, Math.ceil(data.total / PAGE_SIZE));
  const href = (t: Tab, p = 1) => `/bookings?tab=${t}${p > 1 ? `&page=${p}` : ""}`;

  return (
    <main className="mx-auto max-w-(--w-list) px-4 pt-12 pb-18 sm:px-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[40px] font-semibold">การจองของฉัน</h1>
          <p className="mt-1.5 text-(--ink-2)">ดูรายละเอียด แก้ไข หรือยกเลิกได้ก่อนเวลาที่ร้านกำหนด</p>
        </div>
        <nav aria-label="สถานะการจอง" className="glass inline-flex flex-wrap gap-1 rounded-[18px] p-1.5">
          {TABS.map((t) => {
            const active = t.key === tab;
            return (
              <Link
                key={t.key}
                href={href(t.key)}
                aria-current={active ? "page" : undefined}
                className={`font-heading flex min-h-[42px] items-center rounded-[14px] px-4.5 ${
                  active
                    ? "border border-white/95 bg-linear-to-b from-(--btn-primary-from) to-(--btn-primary-to) font-semibold text-(--ink) shadow-[inset_0_1px_0_#fff,0_6px_14px_rgba(107,91,214,0.2)]"
                    : "font-medium text-(--ink-2)"
                }`}
              >
                {t.label} · {data.counts[t.key]}
              </Link>
            );
          })}
        </nav>
      </div>

      {data.items.length === 0 ? (
        <section className="glass mt-8 flex flex-col items-center gap-3 rounded-(--r-card) p-10 text-center">
          <h2 className="text-xl font-semibold">
            {tab === "upcoming" ? "ยังไม่มีการจองที่กำลังจะมาถึง" : tab === "past" ? "ยังไม่มีการจองที่ผ่านไปแล้ว" : "ไม่มีการจองที่ยกเลิก"}
          </h2>
          {tab === "upcoming" && (
            <Button asChild variant="cta">
              <Link href="/">เลือกร้านแล้วจองโต๊ะ</Link>
            </Button>
          )}
        </section>
      ) : (
        <ul className="mt-8 flex flex-col gap-4.5">
          {data.items.map((b) => (
            <BookingRow key={b.id} b={b} />
          ))}
        </ul>
      )}

      {totalPages > 1 && (
        <nav aria-label="การแบ่งหน้า" className="mt-10 flex items-center justify-center gap-4">
          {page > 1 ? (
            <Button asChild variant="glass">
              <Link href={href(tab, page - 1)}>ก่อนหน้า</Link>
            </Button>
          ) : (
            <Button variant="glass" disabled>
              ก่อนหน้า
            </Button>
          )}
          <span className="glass rounded-2xl px-4.5 py-2.5">
            หน้า {page} จาก {totalPages}
          </span>
          {page < totalPages ? (
            <Button asChild variant="glass">
              <Link href={href(tab, page + 1)}>ถัดไป</Link>
            </Button>
          ) : (
            <Button variant="glass" disabled>
              ถัดไป
            </Button>
          )}
        </nav>
      )}
    </main>
  );
}

function BookingRow({ b }: { b: MyBooking }) {
  const tile = dateTile(b.start_at);
  const active = b.status === "active" && !b.is_past;
  const status =
    b.status === "cancelled" ? <Pill tone="gray">ยกเลิกแล้ว</Pill> : b.is_past ? <Pill tone="gray">ผ่านไปแล้ว</Pill> : <Pill tone="mint">ยืนยันแล้ว</Pill>;

  let note: React.ReactNode = null;
  if (b.restaurant_deleted) note = <Pill tone="gray">ร้านนี้ถูกลบแล้ว</Pill>;
  else if (b.can_modify) note = <Pill tone="mint" className="font-medium">ยกเลิกหรือแก้ไขได้ถึง {formatTime(b.cancel_deadline)} น.</Pill>;
  else if (active) note = <Pill tone="butter" className="font-medium">หมดเวลายกเลิกและแก้ไขแล้ว (ถึง {formatTime(b.cancel_deadline)} น.)</Pill>;

  return (
    <li className="glass flex flex-col gap-5 rounded-(--r-card) p-4.5 sm:flex-row sm:items-center">
      <div
        className={`flex h-28 w-[104px] shrink-0 flex-col items-center justify-center rounded-[22px] border border-white/95 shadow-[inset_0_1px_0_rgba(255,255,255,0.9),0_8px_20px_rgba(107,91,214,0.12)] ${
          active ? "bg-linear-160 from-(--btn-primary-from) to-(--btn-primary-to) text-(--ink)" : "bg-(--st-gray-bg) text-(--st-gray-ink)"
        }`}
      >
        <span className="text-[13px] font-medium">{tile.weekday}</span>
        <span className="font-heading text-[40px] leading-none font-semibold">{tile.day}</span>
        <span className="text-[13px] font-medium">{tile.monthYear}</span>
      </div>

      <div className="flex grow flex-col gap-1.5">
        <div className="flex flex-wrap items-center gap-2.5">
          {b.restaurant_deleted ? (
            <span className="font-heading text-[21px] font-semibold">{b.restaurant_name}</span>
          ) : (
            <Link href={`/restaurants/${b.restaurant_id}`} className="font-heading text-[21px] font-semibold hover:underline">
              {b.restaurant_name}
            </Link>
          )}
          {status}
        </div>
        <div className="flex flex-wrap gap-4.5 text-(--ink-body)">
          <span className="flex items-center gap-1.5">
            <ClockIcon />
            {formatTimeRange(b.start_at, b.end_at)}
          </span>
          <span className="flex items-center gap-1.5">
            <PeopleIcon />
            {b.party_size} คน
          </span>
        </div>
        {note && <div className="self-start">{note}</div>}
      </div>

      {active && !b.restaurant_deleted && (
        <div className="flex shrink-0 gap-2 sm:w-40 sm:flex-col">
          <BookingActions booking={b} />
        </div>
      )}
    </li>
  );
}
