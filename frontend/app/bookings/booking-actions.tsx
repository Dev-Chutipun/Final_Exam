"use client";

// ปุ่มแก้ไขและยกเลิกของการจองหนึ่งรายการ (อาร์ตบอร์ด 4, 8)
// เปิดใช้ตาม can_modify จากเซิร์ฟเวอร์เท่านั้น หน้าเว็บไม่คำนวณ deadline เอง
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { BookingForm, type BookingRestaurant } from "@/components/booking/booking-form";
import { ConfirmDialog } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { apiSend } from "@/lib/api-client";
import { problemMessage } from "@/lib/problem";
import { formatDate, formatTime, formatTimeRange, localDate } from "@/lib/time";
import type { MyBooking, RestaurantDetail } from "@/lib/types";

export function BookingActions({ booking }: { booking: MyBooking }) {
  const router = useRouter();
  const [editOpen, setEditOpen] = useState(false);
  const [restaurant, setRestaurant] = useState<BookingRestaurant | null>(null);
  const [cancelOpen, setCancelOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  if (!booking.can_modify) {
    return (
      <>
        <Button variant="glass" disabled>
          แก้ไข
        </Button>
        <Button variant="danger" disabled>
          ยกเลิกการจอง
        </Button>
      </>
    );
  }

  async function openEdit() {
    try {
      const r = await apiSend<RestaurantDetail>("GET", `/api/restaurants/${booking.restaurant_id}`);
      setRestaurant(r);
      setEditOpen(true);
    } catch (err) {
      toast.error(problemMessage(err));
    }
  }

  async function cancel() {
    setBusy(true);
    try {
      await apiSend("POST", `/api/bookings/${booking.id}/cancel`);
      toast.success("ยกเลิกการจองแล้ว");
      setCancelOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(problemMessage(err));
      router.refresh();
    } finally {
      setBusy(false);
    }
  }

  const number = `#${String(booking.id).padStart(4, "0")}`;

  return (
    <>
      <Button variant="glass" onClick={openEdit}>
        แก้ไข
      </Button>
      <ConfirmDialog
        open={cancelOpen}
        onOpenChange={setCancelOpen}
        trigger={<Button variant="danger">ยกเลิกการจอง</Button>}
        title="ยกเลิกการจองนี้?"
        confirmLabel="ยืนยันยกเลิก"
        cancelLabel="ไม่ยกเลิก"
        busy={busy}
        onConfirm={cancel}
      >
        <div className="rounded-[20px] border border-white/95 bg-white/85 px-4 py-3.5">
          <div className="font-heading font-semibold text-(--ink)">{booking.restaurant_name}</div>
          <div className="text-(--ink-body)">
            {formatDate(booking.start_at)} · {formatTimeRange(booking.start_at, booking.end_at)} · {booking.party_size} คน
          </div>
        </div>
        <p>
          ยกเลิกได้ถึง {formatTime(booking.cancel_deadline)} น. หลังยกเลิก ที่นั่งจะว่างให้คนอื่นจองทันที
          ถ้าจะจองใหม่ต้องผ่านเงื่อนไขของร้านอีกครั้ง
        </p>
      </ConfirmDialog>

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="max-h-[92vh] overflow-y-auto rounded-(--r-panel) border border-white/95 bg-white/92 p-6 sm:max-w-lg">
          <DialogTitle className="font-heading text-2xl font-semibold">
            แก้ไขการจอง <span className="text-(--st-lav-ink)">{number}</span>
          </DialogTitle>
          <DialogDescription className="rounded-2xl bg-(--st-mint-bg) px-3.5 py-2.5 text-sm font-medium text-(--st-mint-ink)">
            แก้ไขได้ถึง {formatTime(booking.cancel_deadline)} น. ที่นั่งของการจองเดิมไม่ถูกนับซ้ำเมื่อตรวจที่ว่าง
          </DialogDescription>
          {restaurant && (
            <BookingForm
              restaurant={restaurant}
              initialDate={localDate(booking.start_at)}
              editing={booking}
              onCancelEdit={() => setEditOpen(false)}
              onSaved={() => {
                toast.success("บันทึกการเปลี่ยนแปลงแล้ว");
                setEditOpen(false);
                router.refresh();
              }}
            />
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
