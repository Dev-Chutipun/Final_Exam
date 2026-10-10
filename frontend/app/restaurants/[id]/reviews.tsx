"use client";

// รีวิว (01 §7.7): ฟอร์มเขียน/แก้/ลบ และรายการแบ่งหน้า
// ส่งข้อมูลผ่าน /api/* แล้ว router.refresh() ให้ Server Component อ่านคะแนนเฉลี่ยและ my_review ใหม่
import { useRouter } from "next/navigation";
import { Suspense, useState } from "react";
import { toast } from "sonner";

import { ConfirmDialog } from "@/components/confirm-dialog";
import { StarIcon } from "@/components/icons";
import { LoginLink } from "@/components/login-link";
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { useSuccessDialog } from "@/components/success-dialog";
import { apiSend } from "@/lib/api-client";
import { ApiError, problemMessage } from "@/lib/problem";
import { formatDate } from "@/lib/time";
import { ASPECTS, type Page, type RestaurantDetail, type Review, type ReviewAspects } from "@/lib/types";

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
          {/* เขียน "/ 5" ติดกับตัวเลขเลย กันอ่านสลับกับจำนวนรีวิวด้านล่าง */}
          <span className="font-heading text-[42px] leading-none font-semibold text-(--accent-strong)">
            {restaurant.avg_rating?.toFixed(1) ?? "–"}
            <span className="text-[22px] font-medium text-(--ink-2)"> / 5</span>
          </span>
          <span className="text-sm leading-snug text-(--ink-2)">
            คะแนนเฉลี่ย
            <br />
            จาก {restaurant.review_count} รีวิว
          </span>
        </div>
      </div>

      {/* แยกรายด้านให้เห็นว่าคะแนนรวมมาจากอะไร เช่น อาหารดีแต่บริการช้า */}
      {restaurant.aspect_averages && (
        <div className="grid gap-x-6 gap-y-2 sm:grid-cols-2">
          {ASPECTS.map((a) => {
            const score = restaurant.aspect_averages![a.key];
            return (
              <div key={a.key} className="flex items-center gap-3">
                <span className="min-w-[80px] text-sm text-(--ink-2)">{a.label}</span>
                <span aria-hidden="true" className="h-2 grow overflow-hidden rounded-full bg-(--st-gray-bg)">
                  <span className="block h-full rounded-full bg-linear-to-r from-(--btn-primary-from) to-(--btn-primary-to)" style={{ width: `${(score / 5) * 100}%` }} />
                </span>
                <span className="font-heading min-w-[28px] text-right text-sm font-semibold">{score.toFixed(1)}</span>
              </div>
            );
          })}
        </div>
      )}

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
                    {r.rating.toFixed(1)}/5
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
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const showSuccess = useSuccessDialog();

  async function remove() {
    if (!mine) return;
    setBusy(true);
    try {
      await apiSend("DELETE", `/api/reviews/${mine.id}`);
      showSuccess({ title: "ลบรีวิวแล้ว" });
      setConfirmOpen(false);
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
            {mine.rating.toFixed(1)}/5
          </Pill>
        </span>
        <div className="flex gap-2">
          {/* ลบรีวิวย้อนกลับไม่ได้ และปุ่มอยู่ในแถบที่เห็นตลอด จึงต้องถามยืนยันก่อน */}
          <ConfirmDialog
            open={confirmOpen}
            onOpenChange={setConfirmOpen}
            trigger={<Button variant="danger">ลบรีวิว</Button>}
            title="ลบรีวิวนี้?"
            confirmLabel="ยืนยันลบ"
            cancelLabel="ไม่ลบ"
            busy={busy}
            onConfirm={remove}
          >
            <div className="rounded-[20px] border border-white/95 bg-white/85 px-4 py-3.5">
              <div className="font-heading flex items-center gap-2 font-semibold text-(--ink)">
                <StarIcon size={14} />
                {mine.rating.toFixed(1)}/5
              </div>
              <p className="break-words whitespace-pre-line text-(--ink-body)">{mine.comment}</p>
            </div>
            <p>ลบแล้วกู้คืนไม่ได้ คะแนนเฉลี่ยของร้านจะคำนวณใหม่ทันที · เขียนรีวิวใหม่ได้ภายหลัง</p>
          </ConfirmDialog>
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
  const [aspects, setAspects] = useState<ReviewAspects>(mine?.aspects ?? { food: 0, ambience: 0, service: 0, value: 0 });
  const [comment, setComment] = useState(mine?.comment ?? "");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const showSuccess = useSuccessDialog();

  // ต้องให้ครบทั้ง 4 ด้าน เพราะคะแนนรวมคือค่าเฉลี่ยของทั้งสี่ (เซิร์ฟเวอร์ตรวจซ้ำอยู่แล้ว)
  const complete = ASPECTS.every((a) => aspects[a.key] >= 1);
  const overall = ASPECTS.reduce((sum, a) => sum + aspects[a.key], 0) / ASPECTS.length;

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErrors({});
    try {
      if (mine) await apiSend("PUT", `/api/reviews/${mine.id}`, { aspects, comment });
      else await apiSend("POST", `/api/restaurants/${restaurantId}/reviews`, { aspects, comment });
      showSuccess({
        title: mine ? "บันทึกรีวิวแล้ว" : "ส่งรีวิวแล้ว",
        detail: `คะแนนรวม ${overall.toFixed(1)} จาก 5`,
      });
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
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="font-heading font-semibold">{mine ? "รีวิวของคุณ" : "เขียนรีวิวของคุณ"}</span>
        {complete && (
          <Pill tone="butter">
            <StarIcon size={13} />
            คะแนนรวม {overall.toFixed(1)}
          </Pill>
        )}
      </div>

      {/* ให้คะแนนทีละด้าน คะแนนรวมไม่ได้กรอกเอง ระบบเฉลี่ยให้ */}
      <div className="flex flex-col gap-1">
        {ASPECTS.map((a) => (
          <div key={a.key} className="flex flex-wrap items-center gap-x-3">
            <span className="min-w-[88px] text-sm font-semibold">{a.label}</span>
            <div className="flex gap-0.5" role="group" aria-label={`ให้คะแนนด้าน${a.label}`}>
              {[1, 2, 3, 4, 5].map((i) => (
                <button
                  key={i}
                  type="button"
                  aria-label={`${a.label} ${i} ดาว`}
                  aria-pressed={aspects[a.key] === i}
                  onClick={() => setAspects((prev) => ({ ...prev, [a.key]: i }))}
                  className="flex size-10 items-center justify-center rounded-xl"
                >
                  <StarIcon size={26} filled={i <= aspects[a.key]} />
                </button>
              ))}
            </div>
            {errors[`aspects.${a.key}`] && (
              <span className="text-[13px] font-medium text-(--st-pink-ink)">{errors[`aspects.${a.key}`]}</span>
            )}
          </div>
        ))}
      </div>
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
          <Button type="submit" disabled={busy || !complete || length === 0}>
            {mine ? "บันทึกรีวิว" : "ส่งรีวิว"}
          </Button>
        </div>
      </div>
    </form>
  );
}
