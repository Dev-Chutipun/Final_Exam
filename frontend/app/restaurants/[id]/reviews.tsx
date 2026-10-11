"use client";

// รีวิว (01 §7.7): ฟอร์มเขียน/แก้/ลบ และรายการแบ่งหน้า
// ส่งข้อมูลผ่าน /api/* แล้ว router.refresh() ให้ Server Component อ่านคะแนนเฉลี่ยและ my_review ใหม่
import { useRouter } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";
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
import { ASPECTS, PRICE_LEVELS, priceLabel, type Page, type RestaurantDetail, type Review, type ReviewAspects } from "@/lib/types";

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
  const [items, setItems] = useState(initial.items);
  const [total, setTotal] = useState(initial.total);
  const [page, setPage] = useState(initial.page);
  const [loading, setLoading] = useState(false);
  // สองตัวนี้เก็บ element ไว้ใน state ไม่ใช่ ref เพราะต้องให้ effect รันใหม่เมื่อ element พร้อม
  const [box, setBox] = useState<HTMLDivElement | null>(null);
  const [sentinel, setSentinel] = useState<HTMLLIElement | null>(null);

  const done = items.length >= total;

  const fetchPage = useCallback(
    async (p: number) => {
      return apiSend<Page<Review>>("GET", `/api/restaurants/${restaurant.id}/reviews?page=${p}&page_size=${PAGE_SIZE}`);
    },
    [restaurant.id],
  );

  const loadMore = useCallback(async () => {
    setLoading(true);
    try {
      const d = await fetchPage(page + 1);
      if (d) {
        // กันรีวิวซ้ำ: ถ้ามีคนเขียนรีวิวใหม่ระหว่างที่เราเลื่อน แถวจะเลื่อนลงไปหนึ่งช่องและซ้ำได้
        setItems((prev) => {
          const seen = new Set(prev.map((x) => x.id));
          return [...prev, ...d.items.filter((x) => !seen.has(x.id))];
        });
        setTotal(d.total);
        setPage(d.page);
      }
    } catch (err) {
      toast.error(problemMessage(err));
    } finally {
      setLoading(false);
    }
  }, [fetchPage, page]);

  // โหลดหน้าถัดไปเมื่อเลื่อนใกล้ก้นกล่อง ใช้ IntersectionObserver ไม่ใช่ onScroll
  // เพราะ scroll event ยิงทุกเฟรมและทำให้เรนเดอร์ใหม่รัว ๆ
  useEffect(() => {
    if (!sentinel || !box || done || loading) return;
    const io = new IntersectionObserver((entries) => entries[0]?.isIntersecting && loadMore(), {
      root: box,
      rootMargin: "160px", // เริ่มโหลดก่อนถึงก้นจริง ผู้ใช้จึงไม่ค่อยเห็นรอยสะดุด
    });
    io.observe(sentinel);
    return () => io.disconnect();
  }, [sentinel, box, done, loading, loadMore]);

  async function changed() {
    router.refresh(); // คะแนนเฉลี่ยและ my_review มาจาก Server Component
    try {
      const d = await fetchPage(1);
      if (d) {
        setItems(d.items);
        setTotal(d.total);
        setPage(1);
        box?.scrollTo({ top: 0 });
      }
    } catch (err) {
      toast.error(problemMessage(err));
    }
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

      {/* ราคาที่ลูกค้าจ่ายจริง ใช้มัธยฐานจึงไม่ถูกคนจัดเลี้ยงรายเดียวลากขึ้น */}
      {restaurant.price_level !== null && (
        <div className="flex flex-wrap items-center gap-2.5 rounded-[18px] border border-white/95 bg-(--st-mint-bg)/60 px-4 py-3 text-sm">
          <span className="font-heading font-semibold text-(--st-mint-ink)">
            ราคาต่อคนประมาณ {priceLabel(restaurant.price_level)}
          </span>
          <span className="text-(--ink-2)">จากลูกค้าที่ระบุ {restaurant.price_level_count} คน</span>
        </div>
      )}

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

      {items.length === 0 ? (
        <p className="text-(--ink-2)">ยังไม่มีรีวิว</p>
      ) : (
        // กล่องสูงประมาณ 5 แถว เลื่อนในกล่องแล้วโหลดเพิ่มเองจนครบ หน้าร้านจึงไม่ยาวตามจำนวนรีวิว
        <div ref={setBox} className="-mr-2 max-h-[480px] overflow-y-auto pr-2">
        <ul className="flex flex-col gap-3">
          {items.map((r) => (
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
                  {r.price_level !== null && <Pill tone="mint">{priceLabel(r.price_level, true)}/คน</Pill>}
                  <span className="text-sm text-(--ink-2)">{formatDate(r.created_at)}</span>
                  {r.edited && <span className="text-[13px] text-(--ink-2)">แก้ไขแล้ว</span>}
                  {r.is_mine && <Pill tone="lav">รีวิวของคุณ</Pill>}
                </div>
                <p className="break-words whitespace-pre-line text-(--ink-body)">{r.comment}</p>
              </div>
            </li>
          ))}
          {/* จุดสังเกตก้นรายการ เลื่อนมาถึงเมื่อไรคือสัญญาณให้โหลดหน้าถัดไป */}
          <li ref={setSentinel} aria-hidden="true" className="h-px" />
        </ul>
        <p role="status" className="py-3 text-center text-sm text-(--ink-2)">
          {loading ? "กำลังโหลดรีวิวเพิ่ม…" : done ? `แสดงครบ ${total.toLocaleString("th-TH")} รีวิวแล้ว` : "เลื่อนลงเพื่อดูเพิ่ม"}
        </p>
        </div>
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
  const [price, setPrice] = useState<number | null>(mine?.price_level ?? null);
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
      const body = { aspects, comment, price_level: price };
      if (mine) await apiSend("PUT", `/api/reviews/${mine.id}`, body);
      else await apiSend("POST", `/api/restaurants/${restaurantId}/reviews`, body);
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
      {/* ไม่บังคับ: บางคนมีคนเลี้ยงหรือจำไม่ได้ และบังคับกรอกดาว 4 ด้านไปแล้ว */}
      <div className="flex flex-col gap-1.5 text-sm font-semibold">
        <span>
          ค่าใช้จ่ายต่อคน <span className="font-normal text-(--ink-3)">(ไม่บังคับ)</span>
        </span>
        <div className="flex flex-wrap gap-1.5" role="group" aria-label="ช่วงราคาต่อคน">
          {PRICE_LEVELS.map((p) => {
            const on = price === p.level;
            return (
              <button
                key={p.level}
                type="button"
                aria-pressed={on}
                onClick={() => setPrice(on ? null : p.level)}
                className={`flex min-h-[38px] items-center rounded-full border px-3.5 text-[13px] ${
                  on
                    ? "border-(--btn-primary-to) bg-linear-to-b from-(--btn-primary-from) to-(--btn-primary-to) font-semibold"
                    : "border-white/95 bg-white/70 font-medium text-(--ink-2)"
                }`}
              >
                {p.short}
              </button>
            );
          })}
        </div>
        {errors.price_level && <span className="text-[13px] font-medium text-(--st-pink-ink)">{errors.price_level}</span>}
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
