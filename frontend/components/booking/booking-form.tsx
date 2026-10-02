"use client";

// ฟอร์มจอง/แก้ไขการจอง (อาร์ตบอร์ด 2–3)
// ตัวเลือกเวลาคำนวณจาก availability เพื่อ UX เท่านั้น กติกาจริง (ที่นั่ง, deadline, เวลาเปิด) ตัดสินที่ Go
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiSend } from "@/lib/api-client";
import { ApiError, problemMessage } from "@/lib/problem";
import { addDays, addMinutes, formatDate, formatTime, formatTimeRange, localDate, splitApiTime, todayInBangkok, toApiTime } from "@/lib/time";
import type { Availability, Booking, Slot } from "@/lib/types";

export type BookingRestaurant = {
  id: number;
  name: string;
  capacity: number;
  cancel_window_minutes: number;
  accepting_bookings: boolean;
};

type Props = {
  restaurant: BookingRestaurant;
  initialDate: string;
  /** แก้ไขการจองเดิม: ตัดที่นั่งของการจองนี้ออกตอนแสดงที่ว่าง แล้วส่ง PATCH */
  editing?: { id: number; party_size: number; start_at: string; end_at: string };
  onSaved?: (b: Booking) => void;
  onCancelEdit?: () => void;
};

const MAX_SLOTS = 6; // ครั้งละไม่เกิน 3 ชั่วโมง

export function BookingForm({ restaurant, initialDate, editing, onSaved, onCancelEdit }: Props) {
  const [party, setParty] = useState(String(editing?.party_size ?? 2));
  const [date, setDate] = useState(initialDate);
  const [start, setStart] = useState<string | null>(editing?.start_at ?? null);
  const [end, setEnd] = useState<string | null>(editing?.end_at ?? null);
  const [availability, setAvailability] = useState<Availability | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [error, setError] = useState<{ title: string; detail: string } | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const [booked, setBooked] = useState<Booking | null>(null);

  const today = todayInBangkok();
  const n = Number.parseInt(party, 10);
  const partyValid = Number.isInteger(n) && n >= 1;

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const a = await apiSend<Availability>("GET", `/api/restaurants/${restaurant.id}/availability?date=${date}`);
      setAvailability(a);
    } catch (err) {
      setAvailability(null);
      setLoadError(problemMessage(err));
    }
  }, [restaurant.id, date]);

  useEffect(() => {
    // ดึงที่ว่างใหม่เมื่อเปลี่ยนวันที่ (ข้อมูลจากภายนอก จึงต้องโหลดใน effect)
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  // ที่ว่างของแต่ละช่อง ถ้ากำลังแก้ไข บวกที่นั่งของการจองเดิมคืนให้ (ไม่นับที่นั่งตัวเองซ้ำ — สไลด์ 8)
  const slots = (availability?.slots ?? []).map((s) => {
    if (!editing) return { ...s, free: s.remaining };
    const t = new Date(s.start).getTime();
    const mine = t >= new Date(editing.start_at).getTime() && t < new Date(editing.end_at).getTime();
    return { ...s, free: s.remaining + (mine ? editing.party_size : 0) };
  });

  // เวลาเริ่ม: ช่องที่ bookable และที่ว่างพอสำหรับจำนวนคน
  const canStart = (s: Slot & { free: number }) => s.bookable && partyValid && s.free >= n;
  const startOptions = slots.filter(canStart);

  // เวลาสิ้นสุดจากช่องเริ่ม: +30 นาทีถึง +3 ชั่วโมง ตราบที่ทุกช่องระหว่างทางยังพอ
  const endsFrom = (startIso: string | null): string[] => {
    const i = slots.findIndex((s) => s.start === startIso);
    const out: string[] = [];
    for (let k = i; i >= 0 && partyValid && k < slots.length && k - i < MAX_SLOTS && slots[k].free >= n; k++) {
      out.push(slots[k].end);
    }
    return out;
  };
  const endOptions = endsFrom(start);

  // ถ้าตัวเลือกเดิมใช้ไม่ได้แล้ว (เปลี่ยนวันหรือจำนวนคน) ให้ล้างออก
  const startValid = start !== null && startOptions.some((s) => s.start === start);
  const endValid = end !== null && endOptions.includes(end);
  const selected = startValid && endValid ? slots.filter((s) => s.start >= start! && s.end <= end!) : [];
  const minFree = selected.length > 0 ? Math.min(...selected.map((s) => s.free)) : null;

  const pickStart = (s: string) => {
    setStart(s);
    const ends = endsFrom(s);
    setEnd(ends[1] ?? ends[0] ?? null); // ค่าเริ่มต้น 1 ชั่วโมง ถ้าไม่พอใช้ 30 นาที
  };

  const dayLabel = (iso: string) => (localDate(iso) !== date ? " (วันถัดไป)" : "");

  async function submit() {
    if (!startValid || !endValid || submitting) return;
    setSubmitting(true);
    setError(null);
    setFieldErrors({});
    // ประกอบเวลาเป็นสตริง +07:00 เอง (ห้ามใช้ toISOString)
    const s = splitApiTime(start!);
    const e = splitApiTime(end!);
    const body = { party_size: n, start_at: toApiTime(s.date, s.hhmm), end_at: toApiTime(e.date, e.hhmm) };
    try {
      const b = editing
        ? await apiSend<Booking>("PATCH", `/api/bookings/${editing.id}`, body)
        : await apiSend<Booking>("POST", `/api/restaurants/${restaurant.id}/bookings`, body);
      if (editing) onSaved?.(b!);
      else setBooked(b);
    } catch (err) {
      if (err instanceof ApiError) {
        setFieldErrors(err.fieldErrors);
        setError({ title: err.problem.title, detail: problemMessage(err) });
        // 409 = สถานะเปลี่ยนไปแล้ว (ที่นั่งถูกจองไป) ดึงที่ว่างใหม่
        if (err.status === 409) load();
      } else {
        setError({ title: "ส่งไม่สำเร็จ", detail: problemMessage(err) });
      }
    } finally {
      setSubmitting(false);
    }
  }

  if (booked) return <BookingTicket booking={booked} restaurantName={restaurant.name} onAgain={() => { setBooked(null); setStart(null); load(); }} />;

  const paused = !restaurant.accepting_bookings && !editing;

  return (
    <div className="flex flex-col gap-4">
      {paused && (
        <p className="text-sm text-(--ink-2)">ร้านนี้ปิดรับการจองใหม่ชั่วคราว การจองที่มีอยู่ยังใช้ได้ตามปกติ และยกเลิกได้ภายในเวลาที่กำหนด</p>
      )}

      <div className="grid grid-cols-2 gap-3">
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          จำนวนคน
          <input
            className="field"
            type="number"
            inputMode="numeric"
            min={1}
            max={restaurant.capacity}
            value={party}
            disabled={paused}
            aria-invalid={!partyValid || !!fieldErrors.party_size}
            onChange={(e) => setParty(e.target.value)}
          />
          {(!partyValid || fieldErrors.party_size) && (
            <span className="text-[13px] font-medium text-(--st-pink-ink)">{fieldErrors.party_size ?? "ต้องเป็นจำนวนเต็มตั้งแต่ 1 ขึ้นไป"}</span>
          )}
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          วันที่
          <input
            className="field"
            type="date"
            min={today}
            max={addDays(today, 30)}
            value={date}
            disabled={paused}
            onChange={(e) => e.target.value && setDate(e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          เวลาเริ่ม
          <select
            className="field"
            value={startValid ? start! : ""}
            disabled={paused || startOptions.length === 0}
            aria-invalid={!!fieldErrors.start_at}
            onChange={(e) => pickStart(e.target.value)}
          >
            <option value="" disabled>
              {startOptions.length === 0 ? "ไม่มีช่องว่าง" : "เลือกเวลา"}
            </option>
            {startOptions.map((s) => (
              <option key={s.start} value={s.start}>
                {formatTime(s.start)} น.{dayLabel(s.start)}
              </option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          เวลาสิ้นสุด
          <select
            className="field"
            value={endValid ? end! : ""}
            disabled={paused || endOptions.length === 0}
            aria-invalid={!!fieldErrors.end_at}
            onChange={(e) => setEnd(e.target.value)}
          >
            <option value="" disabled>
              เลือกเวลา
            </option>
            {endOptions.map((e) => (
              <option key={e} value={e}>
                {formatTime(e)} น.{dayLabel(e)}
              </option>
            ))}
          </select>
          {(fieldErrors.start_at || fieldErrors.end_at) && (
            <span className="text-[13px] font-medium text-(--st-pink-ink)">{fieldErrors.end_at ?? fieldErrors.start_at}</span>
          )}
        </label>
      </div>

      <SlotGrid
        slots={slots}
        loading={!availability && !loadError}
        loadError={loadError}
        isSelected={(s) => selected.some((x) => x.start === s.start)}
        isPickable={(s) => !paused && canStart(s)}
        onPick={pickStart}
        dayLabel={dayLabel}
      />

      {error && (
        <div role="alert" className="flex flex-col gap-1 rounded-[18px] border border-(--st-pink-border) bg-(--st-pink-bg) p-3.5 text-sm text-(--btn-danger-ink)">
          <div className="font-heading text-[15px] font-semibold">{error.title}</div>
          <div>{error.detail}</div>
        </div>
      )}

      {startValid && endValid && (
        <div className="flex flex-col gap-1 rounded-[20px] border border-white/95 bg-linear-135 from-(--st-lav-bg)/90 to-white/70 px-4 py-3.5 text-sm">
          <div className="font-heading text-[15px] font-semibold">
            {formatDate(start!)} · {formatTimeRange(start!, end!)} · {n} คน
          </div>
          <div className="text-(--ink-2)">
            ที่ว่างต่ำสุดในช่วงนี้ {minFree} ที่ · ยกเลิกได้ถึง {formatTime(addMinutes(start!, -restaurant.cancel_window_minutes))} น.
          </div>
        </div>
      )}

      {editing ? (
        <div className="flex gap-2.5">
          <Button className="grow" onClick={submit} disabled={!startValid || !endValid || submitting}>
            {submitting ? "กำลังบันทึก…" : "บันทึกการเปลี่ยนแปลง"}
          </Button>
          <Button variant="glass" onClick={onCancelEdit} disabled={submitting}>
            ไม่แก้ไข
          </Button>
        </div>
      ) : (
        <Button variant="cta" size="confirm" onClick={submit} disabled={paused || !startValid || !endValid || submitting}>
          {paused ? "ยังไม่เปิดรับจอง" : submitting ? "กำลังจอง…" : "ยืนยันการจอง"}
        </Button>
      )}
    </div>
  );
}

function SlotGrid({
  slots,
  loading,
  loadError,
  isSelected,
  isPickable,
  onPick,
  dayLabel,
}: {
  slots: (Slot & { free: number })[];
  loading: boolean;
  loadError: string | null;
  isSelected: (s: Slot) => boolean;
  isPickable: (s: Slot & { free: number }) => boolean;
  onPick: (start: string) => void;
  dayLabel: (iso: string) => string;
}) {
  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-heading text-[15px] font-semibold">ที่ว่างรายช่วง 30 นาที</span>
        <span className="flex gap-2.5 text-xs text-(--ink-2)">
          <span className="flex items-center gap-1">
            <span className="size-2.5 rounded-[3px] border border-(--st-butter-ink)/30 bg-(--st-butter-bg)" />
            เหลือน้อย
          </span>
          <span className="flex items-center gap-1">
            <span className="size-2.5 rounded-[3px] shadow-[0_0_0_2px_var(--focus)]" />
            ที่เลือก
          </span>
        </span>
      </div>
      {loadError ? (
        <p role="alert" className="text-sm text-(--st-pink-ink)">{loadError}</p>
      ) : loading ? (
        <p className="text-sm text-(--ink-2)">กำลังโหลดที่ว่าง…</p>
      ) : slots.length === 0 ? (
        <p className="text-sm text-(--ink-2)">ไม่มีช่วงเวลาในวันนี้</p>
      ) : (
        <div className="grid max-h-[248px] grid-cols-3 gap-2 overflow-y-auto p-0.5">
          {slots.map((s) => {
            const sel = isSelected(s);
            const tone =
              s.free === 0
                ? "bg-white/45 text-(--ink-disabled)"
                : s.limited
                  ? "bg-(--st-butter-bg) text-(--st-butter-ink)"
                  : "bg-white/80 text-(--ink)";
            return (
              <button
                key={s.start}
                type="button"
                disabled={!isPickable(s)}
                onClick={() => onPick(s.start)}
                aria-pressed={sel}
                aria-label={`${formatTime(s.start)}${dayLabel(s.start)} ว่าง ${s.free} ที่`}
                className={`flex min-h-14 flex-col justify-center rounded-2xl border border-white/95 px-3 py-1.5 text-left ${tone} ${
                  sel ? "shadow-[0_0_0_2px_var(--focus),0_6px_16px_rgba(107,91,214,0.22)]" : "shadow-[0_4px_12px_rgba(107,91,214,0.07)]"
                } disabled:cursor-not-allowed`}
              >
                <span className="font-heading text-[15px] font-semibold">
                  {formatTime(s.start)}
                  {dayLabel(s.start) && <span className="text-[11px] font-medium">{dayLabel(s.start)}</span>}
                </span>
                <span className="text-xs">{s.free === 0 ? "เต็ม" : s.limited ? `เหลือ ${s.free} ที่` : `ว่าง ${s.free} ที่`}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

/** ใบจองหลังจองสำเร็จ (อาร์ตบอร์ด 3) */
function BookingTicket({ booking, restaurantName, onAgain }: { booking: Booking; restaurantName: string; onAgain: () => void }) {
  const rows: [string, string][] = [
    ["เลขที่การจอง", `#${String(booking.id).padStart(4, "0")}`],
    ["ร้าน", restaurantName],
    ["วันที่", formatDate(booking.start_at)],
    ["เวลา", formatTimeRange(booking.start_at, booking.end_at)],
    ["จำนวน", `${booking.party_size} คน`],
  ];
  return (
    <div className="flex flex-col gap-4" role="status">
      <div className="font-heading flex items-center gap-2.5 text-[22px] font-semibold">
        <span className="flex size-[42px] items-center justify-center rounded-full border border-white bg-(--st-mint-bg)" aria-hidden="true">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--st-mint-ink)" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="5 12.5 10 17.5 19 7.5" />
          </svg>
        </span>
        จองสำเร็จ
      </div>
      <dl className="flex flex-col gap-2 rounded-[22px] border border-white/95 bg-linear-135 from-(--st-lav-bg)/90 to-white/75 p-4.5 text-[15px]">
        {rows.map(([k, v]) => (
          <div key={k} className="flex justify-between gap-3">
            <dt className="text-(--ink-2)">{k}</dt>
            <dd className="text-right font-semibold">{v}</dd>
          </div>
        ))}
      </dl>
      <Pill tone="butter" className="self-start px-3.5 py-2 text-sm font-medium">
        ยกเลิกหรือแก้ไขได้ถึง {formatDateTime2(booking.cancel_deadline)}
      </Pill>
      <div className="flex gap-2.5">
        <Button asChild className="grow">
          <Link href="/bookings">ดูการจองของฉัน</Link>
        </Button>
        <Button variant="glass" onClick={onAgain}>
          จองเพิ่ม
        </Button>
      </div>
    </div>
  );
}

const formatDateTime2 = (iso: string) => `${formatTime(iso)} น. ${formatDate(iso)}`;
