// [B] การจองของร้าน (อาร์ตบอร์ด 7 ส่วนการจองรายวัน) — Server Component
// สองมุมมองในหน้าเดียว สถานะอยู่ใน URL:
//   ?view=list (ค่าเริ่มต้น) → รายการทุกวันแบ่งหน้า เจ้าของเห็นทันทีว่าลูกค้าจองวันไหนบ้าง
//   ?view=day&date=…        → การจองของวันทำการนั้นพร้อม peak สำหรับใช้หน้างาน
// วันทำการใช้หน้าต่างเปิดร้านเดียวกับ availability จึงรวมช่วงหลังเที่ยงคืนของร้านที่เปิดข้ามคืน
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiGet, apiGetOrNull, requireMe } from "@/lib/api-server";
import { addDays, dateTile, formatDate, formatTime, formatTimeRange, localDate, todayInBangkok } from "@/lib/time";
import type { DailyBookings, RestaurantBooking, RestaurantBookings, RestaurantDetail } from "@/lib/types";

import { DatePicker } from "./date-picker";

export const metadata: Metadata = { title: "การจองของร้าน" };

const TABS = [
  { key: "upcoming", label: "กำลังจะมาถึง" },
  { key: "past", label: "ผ่านไปแล้ว" },
  { key: "cancelled", label: "ยกเลิกแล้ว" },
] as const;
type Tab = (typeof TABS)[number]["key"];

const PAGE_SIZE = 20;

type Search = { view?: string; date?: string; tab?: string; page?: string };

export default async function RestaurantBookingsPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<Search>;
}) {
  const { id } = await params;
  const sp = await searchParams;
  const dayView = sp.view === "day";
  const date = /^\d{4}-\d{2}-\d{2}$/.test(sp.date ?? "") ? sp.date! : todayInBangkok();
  const tab: Tab = TABS.some((t) => t.key === sp.tab) ? (sp.tab as Tab) : "upcoming";
  const page = Math.max(1, Number.parseInt(sp.page ?? "1", 10) || 1);

  await requireMe(`/my/restaurants/${id}/bookings`);
  const r = /^\d+$/.test(id) ? await apiGetOrNull<RestaurantDetail>(`/api/restaurants/${id}`) : null;
  if (!r || !r.is_owner) notFound();

  const base = `/my/restaurants/${r.id}/bookings`;
  const listHref = (t: Tab, p = 1) => `${base}?tab=${t}${p > 1 ? `&page=${p}` : ""}`;
  const dayHref = (d: string) => `${base}?view=day&date=${d}`;

  return (
    <main className="mx-auto max-w-(--w-list) px-4 pt-9 pb-18 sm:px-6">
      <nav aria-label="เส้นทาง" className="flex gap-2 text-sm text-(--ink-2)">
        <Link href="/my/restaurants" className="hover:underline">
          ร้านของฉัน
        </Link>
        <span aria-hidden="true">/</span>
        <Link href={`/my/restaurants/${r.id}`} className="hover:underline">
          {r.name}
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-semibold text-(--ink)">การจองของร้าน</span>
      </nav>

      {/* สลับมุมมอง: รายการทั้งหมด หรือ ดูรายวัน */}
      <nav aria-label="มุมมอง" className="glass mt-4 inline-flex gap-1 rounded-[18px] p-1.5">
        <ViewLink href={listHref(tab)} active={!dayView}>
          รายการทั้งหมด
        </ViewLink>
        <ViewLink href={dayHref(date)} active={dayView}>
          ดูรายวัน
        </ViewLink>
      </nav>

      {dayView ? (
        <DayView r={r} date={date} base={base} />
      ) : (
        <ListView r={r} tab={tab} page={page} listHref={listHref} dayHref={dayHref} />
      )}
    </main>
  );
}

/** รายการการจองทุกวันของร้าน แบ่งหน้า */
async function ListView({
  r,
  tab,
  page,
  listHref,
  dayHref,
}: {
  r: RestaurantDetail;
  tab: Tab;
  page: number;
  listHref: (t: Tab, p?: number) => string;
  dayHref: (d: string) => string;
}) {
  const data = await apiGet<RestaurantBookings>(`/api/restaurants/${r.id}/bookings?tab=${tab}&page=${page}&page_size=${PAGE_SIZE}`);
  const totalPages = Math.max(1, Math.ceil(data.total / PAGE_SIZE));

  return (
    <section className="glass mt-4 flex flex-col gap-4 rounded-(--r-card) p-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-[28px] font-semibold">การจองของร้าน</h1>
        <nav aria-label="สถานะการจอง" className="inline-flex flex-wrap gap-1">
          {TABS.map((t) => (
            <Link
              key={t.key}
              href={listHref(t.key)}
              aria-current={t.key === tab ? "page" : undefined}
              className={`font-heading flex min-h-[42px] items-center rounded-[14px] px-4 ${
                t.key === tab
                  ? "border border-white/95 bg-linear-to-b from-(--btn-primary-from) to-(--btn-primary-to) font-semibold text-(--ink) shadow-[inset_0_1px_0_#fff,0_6px_14px_rgba(107,91,214,0.2)]"
                  : "font-medium text-(--ink-2) hover:bg-white/55"
              }`}
            >
              {t.label} · {data.counts[t.key]}
            </Link>
          ))}
        </nav>
      </div>

      {data.items.length === 0 ? (
        <p className="row-plain rounded-[18px] px-4 py-6 text-center text-(--ink-2)">
          {tab === "upcoming" ? "ยังไม่มีลูกค้าจองเข้ามา" : tab === "past" ? "ยังไม่มีการจองที่ผ่านไปแล้ว" : "ไม่มีการจองที่ถูกยกเลิก"}
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {data.items.map((b) => {
            const d = localDate(b.start_at);
            const tile = dateTile(b.start_at);
            return (
              <li key={b.id} className="row-plain flex flex-wrap items-center gap-x-4 gap-y-2 rounded-[18px] px-4 py-3">
                {/* ไทล์วันที่ เพราะรายการนี้รวมหลายวัน ต้องเห็นวันชัดกว่ามุมมองรายวัน */}
                <span className="flex min-w-[76px] flex-col items-center rounded-[14px] border border-white/95 bg-white/70 px-2.5 py-1.5 leading-tight">
                  <span className="text-[11px] text-(--ink-2)">{tile.weekday}</span>
                  <span className="font-heading text-[20px] font-semibold">{tile.day}</span>
                  <span className="text-[11px] text-(--ink-2)">{tile.monthYear}</span>
                </span>
                <span className="font-heading min-w-[128px] font-semibold">{formatTimeRange(b.start_at, b.end_at)}</span>
                <span className="min-w-[120px]">{b.customer_display_name}</span>
                <span className="min-w-[64px]">{b.party_size} คน</span>
                <StatusPill status={b.status} />
                <Link href={dayHref(d)} className="ml-auto rounded-xl px-3 py-2 text-sm text-(--ink-2) hover:bg-white/60">
                  ดูทั้งวันนี้ ›
                </Link>
              </li>
            );
          })}
        </ul>
      )}

      {totalPages > 1 && (
        <nav aria-label="การแบ่งหน้า" className="flex items-center justify-center gap-3">
          <Button asChild={page > 1} variant="glass" size="sm" disabled={page <= 1}>
            {page > 1 ? <Link href={listHref(tab, page - 1)}>ก่อนหน้า</Link> : <span>ก่อนหน้า</span>}
          </Button>
          <span className="text-sm text-(--ink-2)">
            หน้า {page} จาก {totalPages}
          </span>
          <Button asChild={page < totalPages} variant="glass" size="sm" disabled={page >= totalPages}>
            {page < totalPages ? <Link href={listHref(tab, page + 1)}>ถัดไป</Link> : <span>ถัดไป</span>}
          </Button>
        </nav>
      )}
    </section>
  );
}

/** การจองของวันทำการเดียว พร้อม peak สำหรับดูหน้างาน */
async function DayView({ r, date, base }: { r: RestaurantDetail; date: string; base: string }) {
  const data = await apiGet<DailyBookings>(`/api/restaurants/${r.id}/bookings?date=${date}`);
  const active = data.items.filter((b) => b.status === "active");
  const dayHref = (d: string) => `${base}?view=day&date=${d}`;

  return (
    <section className="glass mt-4 flex flex-col gap-4 rounded-(--r-card) p-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-[28px] font-semibold">การจองรายวัน · {formatDate(`${date}T12:00:00+07:00`)}</h1>
        <div className="flex flex-wrap items-center gap-2">
          <Link href={dayHref(addDays(date, -1))} className="rounded-xl px-3 py-2 text-sm text-(--ink-2) hover:bg-white/60">
            ‹ วันก่อน
          </Link>
          <DatePicker value={date} />
          <Link href={dayHref(addDays(date, 1))} className="rounded-xl px-3 py-2 text-sm text-(--ink-2) hover:bg-white/60">
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
                <StatusPill status={b.status} />
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
  );
}

function StatusPill({ status }: { status: RestaurantBooking["status"] }) {
  return status === "active" ? <Pill tone="mint">ยืนยันแล้ว</Pill> : <Pill tone="gray">ยกเลิกแล้ว</Pill>;
}

function ViewLink({ href, active, children }: { href: string; active: boolean; children: React.ReactNode }) {
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={`font-heading flex min-h-[42px] items-center rounded-[14px] px-4.5 ${
        active
          ? "border border-white/95 bg-white/92 font-semibold text-(--ink) shadow-[inset_0_1px_0_#fff,0_4px_12px_rgba(107,91,214,0.10)]"
          : "font-medium text-(--ink-2) hover:bg-white/50"
      }`}
    >
      {children}
    </Link>
  );
}
