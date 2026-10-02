"use client";

// ฟอร์มร้าน (อาร์ตบอร์ด 6) ใช้ทั้งสร้างร้าน (5 ส่วน) และแก้ข้อมูลร้าน (ส่วน 1–2)
// ตรวจฝั่ง client เพื่อ UX เท่านั้น เซิร์ฟเวอร์ตรวจซ้ำทุกช่อง และ error ของเซิร์ฟเวอร์แสดงใต้ช่องตาม errors[].field
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";

import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiSend } from "@/lib/api-client";
import { ApiError, problemMessage } from "@/lib/problem";
import { formatMinutes, halfHours } from "@/lib/time";
import type { Category, RestaurantDetail } from "@/lib/types";

const MAX_IMAGE = 2 * 1024 * 1024;
const IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"];
const CLOCK = halfHours();

type Props =
  | { mode: "create"; categories: Category[] }
  | { mode: "edit"; categories: Category[]; restaurant: RestaurantDetail };

export function RestaurantForm(props: Props) {
  const router = useRouter();
  const editing = props.mode === "edit" ? props.restaurant : null;

  const [name, setName] = useState(editing?.name ?? "");
  const [description, setDescription] = useState(editing?.description ?? "");
  const [categoryIds, setCategoryIds] = useState<number[]>(editing?.categories.map((c) => c.id) ?? []);
  const [images, setImages] = useState<{ file: File; url: string }[]>([]);
  const [imageError, setImageError] = useState<string | null>(null);
  const [openTime, setOpenTime] = useState("10:00");
  const [closeTime, setCloseTime] = useState("22:00");
  const [capacity, setCapacity] = useState("10");
  const [cancelWindow, setCancelWindow] = useState("30");
  const [minLead, setMinLead] = useState("120");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function toggleCategory(id: number) {
    setCategoryIds((ids) => (ids.includes(id) ? ids.filter((x) => x !== id) : ids.length < 3 ? [...ids, id] : ids));
  }

  function addImages(files: FileList | null) {
    setImageError(null);
    const next = [...images];
    for (const f of Array.from(files ?? [])) {
      if (next.length >= 3) {
        setImageError("เพิ่มได้สูงสุด 3 รูป");
        break;
      }
      if (!IMAGE_TYPES.includes(f.type)) {
        setImageError(`${f.name} ไม่ใช่รูป JPEG, PNG หรือ WebP`);
        continue;
      }
      if (f.size > MAX_IMAGE) {
        setImageError(`${f.name} ใหญ่เกิน 2 MB ลองย่อรูปแล้วเลือกใหม่`);
        continue;
      }
      next.push({ file: f, url: URL.createObjectURL(f) });
    }
    setImages(next);
  }

  function removeImage(i: number) {
    URL.revokeObjectURL(images[i].url);
    setImages(images.filter((_, k) => k !== i));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErrors({});
    setFormError(null);
    try {
      if (editing) {
        await apiSend("PATCH", `/api/restaurants/${editing.id}`, { name, description, category_ids: categoryIds });
        toast.success("บันทึกข้อมูลร้านแล้ว");
        router.push(`/my/restaurants/${editing.id}`);
        router.refresh();
        return;
      }
      // multipart: ฟิลด์ data เป็น JSON + ไฟล์ images (ห้ามตั้ง Content-Type เอง apiSend จัดการให้)
      const fd = new FormData();
      fd.append(
        "data",
        JSON.stringify({
          name,
          description,
          capacity: Number(capacity),
          open_time: openTime,
          close_time: closeTime,
          cancel_window_minutes: Number(cancelWindow),
          min_lead_minutes: Number(minLead),
          category_ids: categoryIds,
        }),
      );
      images.forEach((i) => fd.append("images", i.file));
      const created = await apiSend<RestaurantDetail>("POST", "/api/restaurants", fd);
      toast.success("สร้างร้านแล้ว");
      router.push(`/my/restaurants/${created!.id}`);
      router.refresh();
    } catch (err) {
      if (err instanceof ApiError) {
        const fe = err.fieldErrors;
        if (err.type === "restaurant-name-taken") fe.name = "คุณมีร้านชื่อนี้อยู่แล้ว ใช้ชื่ออื่น หรือแก้ไขร้านเดิมแทน";
        if (err.type === "image-too-large" || err.type === "unsupported-image-type") setImageError(problemMessage(err));
        if (err.type === "invalid-restaurant-policy") fe.min_lead_minutes = problemMessage(err);
        setErrors(fe);
      }
      setFormError(problemMessage(err));
    } finally {
      setBusy(false);
    }
  }

  const err = (field: string) =>
    errors[field] ? <span className="text-[13px] font-medium text-(--st-pink-ink)">{errors[field]}</span> : null;
  const lead = Number(minLead);

  return (
    <form onSubmit={submit} noValidate className="mt-7 flex flex-col gap-5.5">
      <Section n={1} title="ข้อมูลร้าน">
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          ชื่อร้าน
          <input className="field" value={name} maxLength={80} required aria-invalid={!!errors.name} onChange={(e) => setName(e.target.value)} />
          {err("name")}
        </label>
        <label className="flex flex-col gap-1.5 text-sm font-semibold">
          รายละเอียด
          <textarea
            rows={5}
            className="field resize-y py-3 font-normal"
            value={description}
            maxLength={2000}
            required
            aria-invalid={!!errors.description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <span className="text-[13px] font-normal text-(--ink-3)">
            ใส่ประเภทอาหาร ที่ตั้ง หรือสิ่งที่ลูกค้าควรรู้ · {[...description.trim()].length.toLocaleString("th-TH")} / 2,000 ตัวอักษร
          </span>
          {err("description")}
        </label>
      </Section>

      <Section n={2} title="หมวดหมู่" aside={<Pill tone="lav">เลือกแล้ว {categoryIds.length} จาก 3</Pill>}>
        <div className="flex flex-wrap gap-2.5" role="group" aria-label="หมวดหมู่ร้าน">
          {props.categories.map((c) => {
            const on = categoryIds.includes(c.id);
            return (
              <button
                key={c.id}
                type="button"
                aria-pressed={on}
                disabled={!on && categoryIds.length >= 3}
                onClick={() => toggleCategory(c.id)}
                className={`flex min-h-[42px] items-center gap-1.5 rounded-full border px-4.5 font-medium disabled:opacity-50 ${
                  on
                    ? "border-(--btn-primary-to) bg-linear-to-b from-(--btn-primary-from) to-(--btn-primary-to) font-semibold shadow-[inset_0_1px_0_#fff,0_6px_16px_rgba(107,91,214,0.2)]"
                    : "border-white/95 bg-white/62"
                }`}
              >
                {on && (
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--st-lav-ink)" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <polyline points="4 12.5 9.5 18 20 6.5" />
                  </svg>
                )}
                {c.name_th}
              </button>
            );
          })}
        </div>
        {err("category_ids")}
      </Section>

      {props.mode === "create" && (
        <>
          <Section n={3} title="รูปร้าน" aside={<span className="text-[13px] text-(--ink-3)">1–3 รูป · JPG, PNG, WebP · ไม่เกิน 2 MB ต่อรูป</span>}>
            <div className="grid gap-3.5 sm:grid-cols-3">
              {images.map((img, i) => (
                <div key={img.url} className="flex flex-col gap-2">
                  <div className="relative h-[132px] overflow-hidden rounded-[20px] bg-white/60">
                    {/* eslint-disable-next-line @next/next/no-img-element -- พรีวิวไฟล์ในเครื่อง (blob URL) */}
                    <img src={img.url} alt={`พรีวิวรูปที่ ${i + 1}`} className="size-full object-cover" />
                    {i === 0 && (
                      <Pill tone="peach" className="absolute top-2 left-2 border border-white">
                        รูปปก
                      </Pill>
                    )}
                  </div>
                  <Button type="button" variant="danger" size="sm" onClick={() => removeImage(i)}>
                    ลบรูป
                  </Button>
                </div>
              ))}
              {images.length < 3 && (
                <label className="relative flex h-[182px] cursor-pointer flex-col items-center justify-center gap-1.5 rounded-[20px] border-2 border-dashed border-(--btn-primary-to) bg-white/55 p-2 text-center text-sm focus-within:outline-3 focus-within:outline-(--focus)">
                  <span className="font-heading font-semibold">เพิ่มรูป</span>
                  <span className="text-xs text-(--ink-3)">กดเพื่อเลือกไฟล์</span>
                  <input
                    type="file"
                    multiple
                    accept={IMAGE_TYPES.join(",")}
                    className="absolute size-px opacity-0"
                    onChange={(e) => {
                      addImages(e.target.files);
                      e.target.value = "";
                    }}
                  />
                </label>
              )}
            </div>
            {(imageError || errors.images) && (
              <div role="alert" className="rounded-2xl border border-(--st-pink-border) bg-(--st-pink-bg) px-3.5 py-3 text-sm text-(--btn-danger-ink)">
                {imageError ?? errors.images}
              </div>
            )}
          </Section>

          <Section n={4} title="เวลาเปิด–ปิดและที่นั่ง">
            <div className="grid gap-3 sm:grid-cols-3">
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
                จำนวนที่นั่ง
                <input className="field" type="number" min={1} max={500} value={capacity} aria-invalid={!!errors.capacity} onChange={(e) => setCapacity(e.target.value)} />
                {err("capacity")}
              </label>
            </div>
            <p className="rounded-[18px] bg-(--st-butter-bg) px-3.5 py-3 text-sm text-(--st-butter-ink)">
              เลือกเวลาได้ทีละ 30 นาที ถ้าเวลาปิดน้อยกว่าเวลาเปิด หมายถึงปิดหลังเที่ยงคืน เช่น 18:00–02:00 · ที่นั่ง 1–500 นับรวมทั้งร้าน ไม่แยกโต๊ะ
            </p>
          </Section>

          <Section n={5} title="นโยบายการจอง">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="flex flex-col gap-1.5 text-sm font-semibold">
                ยกเลิกหรือแก้ไขได้ก่อนเวลาจอง (นาที)
                <input className="field" type="number" min={30} value={cancelWindow} aria-invalid={!!errors.cancel_window_minutes} onChange={(e) => setCancelWindow(e.target.value)} />
                <span className="text-[13px] font-normal text-(--ink-3)">ขั้นต่ำ 30 นาที</span>
                {err("cancel_window_minutes")}
              </label>
              <label className="flex flex-col gap-1.5 text-sm font-semibold">
                จองล่วงหน้าอย่างน้อย (นาที)
                <input className="field" type="number" min={30} value={minLead} aria-invalid={!!errors.min_lead_minutes} onChange={(e) => setMinLead(e.target.value)} />
                <span className="text-[13px] font-normal text-(--ink-3)">
                  {Number.isFinite(lead) && lead > 0 ? `เท่ากับ ${formatMinutes(lead)} ` : ""}ต้องไม่น้อยกว่าเวลายกเลิก
                </span>
                {err("min_lead_minutes")}
              </label>
            </div>
          </Section>
        </>
      )}

      {formError && (
        <div role="alert" className="rounded-[18px] border border-(--st-pink-border) bg-(--st-pink-bg) px-4 py-3 text-sm text-(--btn-danger-ink)">
          {formError}
        </div>
      )}

      <div className="glass flex justify-end gap-3 rounded-3xl p-3.5">
        <Button asChild variant="glass" size="lg">
          <Link href={editing ? `/my/restaurants/${editing.id}` : "/my/restaurants"}>ยกเลิก</Link>
        </Button>
        <Button type="submit" variant="cta" size="lg" disabled={busy}>
          {busy ? "กำลังบันทึก…" : editing ? "บันทึกข้อมูลร้าน" : "สร้างร้าน"}
        </Button>
      </div>
    </form>
  );
}

function Section({ n, title, aside, children }: { n: number; title: string; aside?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="glass flex flex-col gap-4 rounded-(--r-card) p-6 sm:p-7">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <span
            aria-hidden="true"
            className="font-heading flex size-9 shrink-0 items-center justify-center rounded-[13px] border border-white bg-linear-to-b from-(--btn-cta-from) to-(--btn-cta-to) text-lg font-semibold text-(--btn-cta-ink)"
          >
            {n}
          </span>
          <h2 className="text-[21px] font-semibold">{title}</h2>
        </div>
        {aside}
      </div>
      {children}
    </section>
  );
}

export function ClockSelect({ value, onChange, invalid }: { value: string; onChange: (v: string) => void; invalid?: boolean }) {
  return (
    <select className="field" value={value} aria-invalid={invalid} onChange={(e) => onChange(e.target.value)}>
      {CLOCK.map((t) => (
        <option key={t} value={t}>
          {t} น.
        </option>
      ))}
    </select>
  );
}
