"use client";

// รีวิว (01 §7.7): ฟอร์มเขียน/แก้/ลบ และรายการแบ่งหน้า
// ส่งข้อมูลผ่าน /api/* แล้ว router.refresh() ให้ Server Component อ่านคะแนนเฉลี่ยและ my_review ใหม่
import { useRouter } from "next/navigation";
import { Suspense, useState } from "react";
import { toast } from "sonner";

import { StarIcon } from "@/components/icons";
import { LoginLink } from "@/components/login-link";
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { useSuccessDialog } from "@/components/success-dialog";
import { apiSend } from "@/lib/api-client";
import { ApiError, problemMessage } from "@/lib/problem";
import { formatDate } from "@/lib/time";
import type { Page, RestaurantDetail, Review } from "@/lib/types";

const PAGE_SIZE = 10;

export function Reviews({
  restaurant,
  loggedIn,
  initial,
}: {
  restaurant: RestaurantDetail;
  loggedIn: boolean;
  initial: Page<Review>;
}) {
  const router = useRouter();
  const [data, setData] = useState(initial);
  const totalPages = Math.max(1, Math.ceil(data.total / PAGE_SIZE));

  async function loadPage(page: number) {
    try {
      const d = await apiSend<Page<Review>>("GET", `/api/restaurants/${restaurant.id}/reviews?page=${page}&page_size=${PAGE_SIZE}`);
      if (d) setData(d);
    } catch (err) {
      toast.error(problemMessage(err));
    }
  }

  async function changed() {
    router.refresh(); // คะแนนเฉลี่ยและ my_review มาจาก Server Component
    await loadPage(1);
  }

  return (
    <section aria-label="รีวิว" className="glass flex flex-col gap-4.5 rounded-(--r-card) p-6 sm:p-7">
      <div className="flex items-center justify-between gap-4">
        <h2 className="text-[22px] font-semibold">รีวิวจากลูกค้า</h2>
        <div className="flex items-center gap-3">
          <span className="font-heading text-[42px] leading-none font-semibold text-(--accent-strong)">
            {restaurant.avg_rating?.toFixed(1) ?? "–"}
          </span>
          <span className="text-sm leading-snug text-(--ink-2)">
            เฉลี่ยจาก 5
            <br />
            {restaurant.review_count} รีวิว
          </span>
        </div>
      </div>

      {restaurant.is_owner ? (
        <p className="rounded-[18px] bg-(--st-peach-bg) px-4 py-3 text-sm text-(--st-peach-ink)">เจ้าของร้านรีวิวร้านของตัวเองไม่ได้</p>
      ) : !loggedIn ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-[22px] border border-white/95 bg-white/60 p-4">
          <span className="text-(--ink-2)">เข้าสู่ระบบเพื่อเขียนรีวิว</span>
          <Suspense fallback={null}>
            <LoginLink variant="glass" />
          </Suspense>
        </div>
      ) : (
        <MyReview restaurantId={restaurant.id} mine={restaurant.my_review} onChanged={changed} />
      )}

      {data.items.length === 0 ? (
        <p className="text-(--ink-2)">ยังไม่มีรีวิว</p>
      ) : (
        <ul className="flex flex-col gap-3">
          {data.items.map((r) => (
            <li key={r.id} className="row-plain flex gap-3.5 rounded-[22px] p-4.5">
              <span
                aria-hidden="true"
                className="font-heading flex size-[46px] shrink-0 items-center justify-center rounded-full border border-white bg-linear-135 from-(--btn-primary-from) to-(--btn-primary-to) font-semibold"
              >
                {r.display_name.charAt(0)}
              </span>
              <div className="flex min-w-0 flex-col gap-1.5">
                <div className="flex flex-wrap items-center gap-2.5">
                  <span className="font-heading font-semibold">{r.display_name}</span>
                  <Pill tone="butter">
                    <StarIcon size={13} />
                    {r.rating}/5
                  </Pill>
                  <span className="text-sm text-(--ink-2)">{formatDate(r.created_at)}</span>
                  {r.edited && <span className="text-[13px] text-(--ink-2)">แก้ไขแล้ว</span>}
                  {r.is_mine && <Pill tone="lav">รีวิวของคุณ</Pill>}
                </div>
                <p className="break-words whitespace-pre-line text-(--ink-body)">{r.comment}</p>
              </div>
            </li>
          ))}
        </ul>
      )}

      {totalPages > 1 && (
        <nav aria-label="หน้ารีวิว" className="flex items-center justify-center gap-3">
          <Button variant="glass" size="sm" disabled={data.page <= 1} onClick={() => loadPage(data.page - 1)}>
            ก่อนหน้า
          </Button>
          <span className="text-sm text-(--ink-2)">
            หน้า {data.page} จาก {totalPages}
          </span>
          <Button variant="glass" size="sm" disabled={data.page >= totalPages} onClick={() => loadPage(data.page + 1)}>
            ถัดไป
          </Button>
        </nav>
      )}
    </section>
  );
}

/**
 * ส่วนรีวิวของผู้ใช้ที่ login แล้ว
 * ยังไม่เคยรีวิว → เปิดฟอร์มเขียนไว้เลย · เคยรีวิวแล้ว → แสดงแถบสรุปกับปุ่ม ไม่เปิดฟอร์มแก้ไขให้เอง
 * (เนื้อรีวิวอยู่ในรายการข้างล่างพร้อมป้าย "รีวิวของคุณ" อยู่แล้ว จึงไม่ต้องแสดงซ้ำ)
 */
function MyReview({
  restaurantId,
  mine,
  onChanged,
}: {
  restaurantId: number;
  mine: RestaurantDetail["my_review"];
  onChanged: () => Promise<void>;
}) {
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const showSuccess = useSuccessDialog();

  async function remove() {
    if (!mine) return;
    setBusy(true);
    try {
      await apiSend("DELETE", `/api/reviews/${mine.id}`);
      showSuccess({ title: "ลบรีวิวแล้ว" });
      setEditing(false);
      await onChanged();
    } catch (err) {
      toast.error(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (mine && !editing) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-[22px] border border-white/95 bg-linear-135 from-(--st-peach-bg)/70 to-(--st-lav-bg)/70 p-4.5">
        <span className="font-heading flex items-center gap-2 font-semibold">
          คุณรีวิวร้านนี้ไว้แล้ว
          <Pill tone="butter">
            <StarIcon size={13} />
            {mine.rating}/5
          </Pill>
        </span>
        <div className="flex gap-2">
          <Button variant="danger" onClick={remove} disabled={busy}>
            ลบรีวิว
          </Button>
          <Button variant="glass" onClick={() => setEditing(true)} disabled={busy}>
            แก้รีวิว
          </Button>
        </div>
      </div>
    );
  }

  return (
    <ReviewForm
      key={mine?.id ?? "new"}
      restaurantId={restaurantId}
      mine={mine}
      onCancel={mine ? () => setEditing(false) : undefined}
      onChanged={async () => {
        setEditing(false);
        await onChanged();
      }}
    />
  );
}

function ReviewForm({
  restaurantId,
  mine,
  onCancel,
  onChanged,
}: {
  restaurantId: number;
  mine: RestaurantDetail["my_review"];
  /** มีเฉพาะตอนแก้รีวิวเดิม กดแล้วกลับไปแถบสรุปโดยไม่บันทึก */
  onCancel?: () => void;
  onChanged: () => Promise<void>;
}) {
  const [rating, setRating] = useState(mine?.rating ?? 0);
  const [comment, setComment] = useState(mine?.comment ?? "");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const showSuccess = useSuccessDialog();

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErrors({});
    try {
      if (mine) await apiSend("PUT", `/api/reviews/${mine.id}`, { rating, comment });
      else await apiSend("POST", `/api/restaurants/${restaurantId}/reviews`, { rating, comment });
      showSuccess({ title: mine ? "บันทึกรีวิวแล้ว" : "ส่งรีวิวแล้ว", detail: `ให้ ${rating} ดาวกับร้านนี้` });
      await onChanged();
    } catch (err) {
      if (err instanceof ApiError) setErrors(err.fieldErrors);
      toast.error(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  const length = [...comment.trim()].length;

  return (
    <form
      onSubmit={save}
      className="flex flex-col gap-3 rounded-[22px] border border-white/95 bg-linear-135 from-(--st-peach-bg)/70 to-(--st-lav-bg)/70 p-4.5"
    >
      <div className="font-heading font-semibold">{mine ? "รีวิวของคุณ" : "เขียนรีวิวของคุณ"}</div>
      <div className="flex gap-0.5" role="group" aria-label="ให้คะแนน">
        {[1, 2, 3, 4, 5].map((i) => (
          <button
            key={i}
            type="button"
            aria-label={`${i} ดาว`}
            aria-pressed={rating === i}
            onClick={() => setRating(i)}
            className="flex size-11 items-center justify-center rounded-xl"
          >
            <StarIcon size={30} filled={i <= rating} />
          </button>
        ))}
      </div>
      {errors.rating && <span className="text-[13px] font-medium text-(--st-pink-ink)">{errors.rating}</span>}
      <label className="flex flex-col gap-1.5 text-sm font-semibold">
        ข้อความรีวิว
        <textarea
          rows={3}
          maxLength={1000}
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          aria-invalid={!!errors.comment}
          placeholder="บอกเล่าประสบการณ์ของคุณที่ร้านนี้"
          className="field resize-y py-3 font-normal"
        />
        {errors.comment && <span className="text-[13px] font-medium text-(--st-pink-ink)">{errors.comment}</span>}
      </label>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-[13px] text-(--ink-2)">{length.toLocaleString("th-TH")} / 1,000 ตัวอักษร</span>
        <div className="flex gap-2">
          {onCancel && (
            <Button type="button" variant="glass" onClick={onCancel} disabled={busy}>
              ไม่แก้ไข
            </Button>
          )}
          <Button type="submit" disabled={busy || rating === 0 || length === 0}>
            {mine ? "บันทึกรีวิว" : "ส่งรีวิว"}
          </Button>
        </div>
      </div>
    </form>
  );
}
