"use client";

// ส่วนโต้ตอบของหน้าจัดการร้าน (อาร์ตบอร์ด 7–8)
// ทุกปุ่มส่งผ่าน /api/* แล้ว router.refresh() ให้ Server Component อ่านข้อมูลใหม่
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { useSuccessDialog } from "@/components/success-dialog";
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import { apiSend } from "@/lib/api-client";
import { imageUrl } from "@/lib/cover";
import { ApiError, problemMessage } from "@/lib/problem";
import { formatDate, formatDateTime, formatTimeRange } from "@/lib/time";
import type { RestaurantDetail, SettingsConflict } from "@/lib/types";

import { ClockSelect } from "../restaurant-form";

/** สวิตช์พักรับจอง [B] */
export function AcceptingSwitch({ restaurantId, accepting }: { restaurantId: number; accepting: boolean }) {
  const router = useRouter();
  const showSuccess = useSuccessDialog();
  const [busy, setBusy] = useState(false);

  async function change(next: boolean) {
    setBusy(true);
    try {
      await apiSend("PATCH", `/api/restaurants/${restaurantId}/booking-status`, { accepting_bookings: next });
      showSuccess({ title: next ? "เปิดรับจองแล้ว" : "พักรับจองแล้ว", detail: next ? "ลูกค้าจองร้านนี้ได้แล้ว" : "ลูกค้าจองใหม่ไม่ได้ แต่การจองเดิมยังใช้ได้ตามปกติ" });
      router.refresh();
    } catch (err) {
      toast.error(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex shrink-0 items-center gap-3">
      <span className={`font-heading font-semibold ${accepting ? "text-(--st-mint-ink)" : "text-(--st-gray-ink)"}`}>
        {accepting ? "เปิดรับจอง" : "พักรับจอง"}
      </span>
      <Switch checked={accepting} disabled={busy} onCheckedChange={change} aria-label="เปิดรับการจอง" />
    </div>
  );
}

/** ตั้งค่าการจอง (01 §7.6): 409 แสดงรายการการจองที่ชน */
export function SettingsForm({ r }: { r: RestaurantDetail }) {
  const router = useRouter();
  const showSuccess = useSuccessDialog();
  const [capacity, setCapacity] = useState(String(r.capacity));
  const [openTime, setOpenTime] = useState(r.open_time);
  const [closeTime, setCloseTime] = useState(r.close_time);
  const [cancelWindow, setCancelWindow] = useState(String(r.cancel_window_minutes));
  const [minLead, setMinLead] = useState(String(r.min_lead_minutes));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [problem, setProblem] = useState<{ detail: string; conflicts: SettingsConflict[] } | null>(null);
  const [busy, setBusy] = useState(false);

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErrors({});
    setProblem(null);
    try {
      await apiSend("PATCH", `/api/restaurants/${r.id}/settings`, {
        capacity: Number(capacity),
        open_time: openTime,
        close_time: closeTime,
        cancel_window_minutes: Number(cancelWindow),
        min_lead_minutes: Number(minLead),
      });
      showSuccess({ title: "บันทึกการตั้งค่าแล้ว" });
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        setErrors(err.fieldErrors);
        if (err.type === "invalid-restaurant-policy") setErrors({ min_lead_minutes: problemMessage(err) });
        setProblem({ detail: problemMessage(err), conflicts: (err.problem.conflicts as SettingsConflict[] | undefined) ?? [] });
      } else {
        setProblem({ detail: problemMessage(err), conflicts: [] });
      }
    } finally {
      setBusy(false);
    }
  }

  const err = (f: string) => (errors[f] ? <span className="text-[13px] font-medium text-(--st-pink-ink)">{errors[f]}</span> : null);

  return (
    <form onSubmit={save} noValidate className="flex flex-col gap-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          จำนวนที่นั่ง
          <input className="field" type="number" min={1} max={500} value={capacity} aria-invalid={!!errors.capacity} onChange={(e) => setCapacity(e.target.value)} />
          <span className="text-[13px] font-normal text-(--ink-3)">ค่าปัจจุบัน {r.capacity} ที่นั่ง</span>
          {err("capacity")}
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          เวลาเปิด
          <ClockSelect value={openTime} onChange={setOpenTime} invalid={!!errors.open_time} />
          {err("open_time")}
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          เวลาปิด
          <ClockSelect value={closeTime} onChange={setCloseTime} invalid={!!errors.close_time} />
          {err("close_time")}
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          ยกเลิกหรือแก้ไขได้ก่อน (นาที)
          <input className="field" type="number" min={30} value={cancelWindow} aria-invalid={!!errors.cancel_window_minutes} onChange={(e) => setCancelWindow(e.target.value)} />
          {err("cancel_window_minutes")}
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          จองล่วงหน้าอย่างน้อย (นาที)
          <input className="field" type="number" min={30} value={minLead} aria-invalid={!!errors.min_lead_minutes} onChange={(e) => setMinLead(e.target.value)} />
          {err("min_lead_minutes")}
        </label>
      </div>
      <p className="text-[13px] text-(--ink-3)">
        การเปลี่ยนเวลายกเลิกหรือเวลาจองล่วงหน้าไม่กระทบการจองเดิม เพราะ deadline ถูกบันทึกไว้ตอนจองแล้ว
      </p>

      {problem && (
        <div role="alert" className="flex flex-col gap-2 rounded-[20px] border border-(--st-pink-border) bg-(--st-pink-bg) px-4.5 py-4 text-sm text-(--btn-danger-ink)">
          <div className="font-heading text-base font-semibold">
            {problem.conflicts.length > 0 ? "บันทึกไม่ได้: ค่าใหม่ชนกับการจองที่มีอยู่" : "บันทึกไม่ได้"}
          </div>
          <div>{problem.detail}</div>
          {problem.conflicts.length > 0 && (
            <ul className="flex flex-col gap-1.5">
              {problem.conflicts.map((c) => (
                <li key={`${c.booking_id}-${c.reason}`} className="rounded-[14px] bg-white/75 px-3 py-2 text-(--ink)">
                  การจอง #{String(c.booking_id).padStart(4, "0")} · {formatDate(c.start_at)} · {formatTimeRange(c.start_at, c.end_at)} ·{" "}
                  {c.reason === "capacity" ? "ที่นั่งไม่พอ" : "อยู่นอกเวลาเปิดใหม่"}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      <div className="flex justify-end">
        <Button type="submit" size="lg" disabled={busy}>
          {busy ? "กำลังบันทึก…" : "บันทึกการตั้งค่า"}
        </Button>
      </div>
    </form>
  );
}

const IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"];

/** รูปร้าน 1–3 รูป: ครบ 3 แล้วปิดการเพิ่ม · เหลือรูปเดียวลบไม่ได้ */
export function ImagesManager({ restaurantId, imageIds }: { restaurantId: number; imageIds: number[] }) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);

  async function upload(file: File | undefined) {
    if (!file) return;
    if (!IMAGE_TYPES.includes(file.type)) return toast.error(`${file.name} ไม่ใช่รูป JPEG, PNG หรือ WebP`);
    if (file.size > 2 * 1024 * 1024) return toast.error(`${file.name} ใหญ่เกิน 2 MB ลองย่อรูปแล้วเลือกใหม่`);
    setBusy(true);
    try {
      const fd = new FormData();
      fd.append("image", file);
      await apiSend("POST", `/api/restaurants/${restaurantId}/images`, fd);
      toast.success("เพิ่มรูปแล้ว");
      router.refresh();
    } catch (err) {
      toast.error(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: number) {
    setBusy(true);
    try {
      await apiSend("DELETE", `/api/restaurants/${restaurantId}/images/${id}`);
      toast.success("ลบรูปแล้ว");
      setDeleting(null);
      router.refresh();
    } catch (err) {
      toast.error(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {imageIds.map((id, i) => (
        <div key={id} className="flex flex-col gap-2">
          <div className="relative h-29 overflow-hidden rounded-[20px] bg-white/60">
            {/* eslint-disable-next-line @next/next/no-img-element -- รูปจาก Go ตาม 05 §2 */}
            <img src={imageUrl(restaurantId, id)} alt={`รูปที่ ${i + 1}`} width={220} height={116} className="size-full object-cover" />
            {i === 0 && (
              <Pill tone="peach" className="absolute top-2 left-2 border border-white">
                รูปปก
              </Pill>
            )}
          </div>
          <ConfirmDialog
            open={deleting === id}
            onOpenChange={(o) => setDeleting(o ? id : null)}
            trigger={
              <Button variant="danger" size="sm" disabled={busy || imageIds.length <= 1} title={imageIds.length <= 1 ? "ร้านต้องมีรูปอย่างน้อย 1 รูป" : undefined}>
                ลบรูป
              </Button>
            }
            title="ลบรูปนี้?"
            confirmLabel="ลบรูป"
            cancelLabel="ไม่ลบ"
            busy={busy}
            onConfirm={() => remove(id)}
          >
            <p>{i === 0 ? "รูปนี้เป็นรูปปก หลังลบ รูปถัดไปจะเป็นรูปปกแทน" : "ลบแล้วกู้คืนไม่ได้"}</p>
          </ConfirmDialog>
        </div>
      ))}
      {imageIds.length < 3 ? (
        <label className="relative flex h-[166px] cursor-pointer flex-col items-center justify-center gap-1 rounded-[20px] border-2 border-dashed border-(--btn-primary-to) bg-white/55 p-2 text-center text-sm focus-within:outline-3 focus-within:outline-(--focus)">
          <span className="font-heading font-semibold">{busy ? "กำลังอัปโหลด…" : "เพิ่มรูป"}</span>
          <span className="text-xs text-(--ink-3)">JPG, PNG, WebP ไม่เกิน 2 MB</span>
          <input
            type="file"
            accept={IMAGE_TYPES.join(",")}
            disabled={busy}
            className="absolute size-px opacity-0"
            onChange={(e) => {
              upload(e.target.files?.[0]);
              e.target.value = "";
            }}
          />
        </label>
      ) : (
        <div className="flex h-[166px] flex-col items-center justify-center gap-1 rounded-[20px] border-2 border-dashed border-(--st-gray-bg) bg-white/40 p-2 text-center text-sm text-(--ink-2)">
          <span className="font-heading font-semibold">เพิ่มรูปไม่ได้</span>
          <span className="text-xs">ครบ 3 รูปแล้ว ลบรูปเดิมก่อน</span>
        </div>
      )}
    </div>
  );
}

/** ลบร้าน: 409 → แสดงว่ายังมีการจอง พร้อมปุ่มพักรับจอง (อาร์ตบอร์ด 8) */
export function DeleteRestaurant({ r }: { r: RestaurantDetail }) {
  const router = useRouter();
  const showSuccess = useSuccessDialog();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [blocked, setBlocked] = useState<{ count: number; latest: string; accepting: boolean } | null>(null);
  const [busy, setBusy] = useState(false);

  async function remove() {
    setBusy(true);
    try {
      await apiSend("DELETE", `/api/restaurants/${r.id}`);
      showSuccess({ title: "ลบร้านแล้ว", detail: r.name });
      router.push("/my/restaurants");
      router.refresh();
    } catch (err) {
      setConfirmOpen(false);
      if (err instanceof ApiError && err.type === "restaurant-has-upcoming-bookings") {
        setBlocked({
          count: Number(err.problem.upcoming_count),
          latest: String(err.problem.latest_booking_end),
          accepting: Boolean(err.problem.accepting_bookings),
        });
      } else {
        toast.error(problemMessage(err));
      }
    } finally {
      setBusy(false);
    }
  }

  async function suspend() {
    setBusy(true);
    try {
      await apiSend("PATCH", `/api/restaurants/${r.id}/booking-status`, { accepting_bookings: false });
      showSuccess({ title: "พักรับจองแล้ว", detail: "ลูกค้าจองใหม่ไม่ได้ แต่การจองเดิมยังใช้ได้ตามปกติ" });
      setBlocked(null);
      router.refresh();
    } catch (err) {
      toast.error(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        trigger={
          <Button variant="danger" size="lg" className="shrink-0">
            ลบร้านนี้
          </Button>
        }
        title="ลบร้านนี้?"
        confirmLabel="ลบร้าน"
        cancelLabel="ไม่ลบ"
        busy={busy}
        onConfirm={remove}
      >
        <div className="rounded-[20px] border border-white/95 bg-white/85 px-4 py-3.5">
          <div className="font-heading font-semibold text-(--ink)">{r.name}</div>
          <div className="text-(--ink-body)">รีวิว {r.review_count} รายการ</div>
        </div>
        <p>ร้านจะถูกซ่อนจากทุกคนและเปิดหน้าร้านไม่ได้ ประวัติการจองที่ผ่านมาของลูกค้ายังอยู่ในหน้าการจองของพวกเขา</p>
      </ConfirmDialog>

      <Dialog open={blocked !== null} onOpenChange={(o) => !o && setBlocked(null)}>
        <DialogContent className="flex flex-col gap-4 rounded-[32px] border border-white/95 bg-white/90 p-7 sm:max-w-md">
          <DialogTitle className="font-heading text-2xl font-semibold">ลบร้านไม่ได้ในตอนนี้</DialogTitle>
          {blocked && (
            <div role="alert" className="rounded-[20px] border border-(--st-butter-ink)/20 bg-(--st-butter-bg) px-4 py-3.5 text-[15px] text-(--st-butter-ink)">
              <div className="font-heading font-semibold">
                {r.name}ยังมีการจองล่วงหน้า {blocked.count} รายการ
              </div>
              <div>การจองสุดท้ายสิ้นสุด {formatDateTime(blocked.latest)}</div>
            </div>
          )}
          <DialogDescription className="text-[15px] text-(--ink-2)">
            พักรับจองก่อน แล้วรอให้การจองเดิมสิ้นสุดหรือถูกยกเลิก จึงจะลบร้านได้ ลูกค้ายังยกเลิกการจองของตัวเองได้ตามปกติ
          </DialogDescription>
          <div className="flex justify-end gap-2.5">
            <Button variant="glass" onClick={() => setBlocked(null)}>
              ปิดหน้าต่าง
            </Button>
            {blocked?.accepting && (
              <Button onClick={suspend} disabled={busy}>
                พักรับจองตอนนี้
              </Button>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
