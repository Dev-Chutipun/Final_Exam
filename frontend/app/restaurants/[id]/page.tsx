// หน้ารายละเอียดร้าน (อาร์ตบอร์ด 2) — Server Component อ่านข้อมูล
// แผงจองและรีวิวเป็น Client Component ที่ส่งข้อมูลผ่าน /api/*
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { CoverImage } from "@/components/cover-image";
import { StarIcon } from "@/components/icons";
import { Pill } from "@/components/pill";
import { apiGet, apiGetOrNull, getMe } from "@/lib/api-server";
import { coverGradient, imageUrl } from "@/lib/cover";
import { formatMinutes, todayInBangkok } from "@/lib/time";
import type { Page, RestaurantDetail, Review } from "@/lib/types";

import { BookingPanel } from "./booking-panel";
import { Reviews } from "./reviews";

type Params = { params: Promise<{ id: string }> };

async function load(id: string) {
  if (!/^\d+$/.test(id)) return null;
  return apiGetOrNull<RestaurantDetail>(`/api/restaurants/${id}`);
}

export async function generateMetadata({ params }: Params): Promise<Metadata> {
  const r = await load((await params).id);
  return { title: r?.name ?? "ไม่พบร้าน" };
}

export default async function RestaurantPage({ params }: Params) {
  const { id } = await params;
  const [r, me] = await Promise.all([load(id), getMe()]);
  if (!r) notFound();
  const reviews = await apiGet<Page<Review>>(`/api/restaurants/${r.id}/reviews?page=1&page_size=10`);
  const [cover, ...others] = r.image_ids;
  const firstSlug = r.categories[0]?.slug;

  return (
    <main className="mx-auto max-w-[1232px] px-4 pt-5 pb-18 sm:px-6">
      <nav aria-label="เส้นทาง" className="flex gap-2 px-1.5 text-sm text-(--ink-2)">
        <Link href="/" className="hover:underline">
          ร้านอาหาร
        </Link>
        <span aria-hidden="true">/</span>
        <span className="font-semibold text-(--ink)">{r.name}</span>
      </nav>

      <CoverImage
        restaurantId={r.id}
        imageId={cover ?? null}
        categorySlug={firstSlug}
        alt={`ภาพปกร้าน ${r.name}`}
        className="mt-3.5 min-h-[380px] rounded-[32px] shadow-[0_20px_50px_rgba(250,172,166,0.30)]"
      >
        {others.length > 0 && (
          <div className="absolute top-5 right-5 hidden gap-2.5 sm:flex">
            {others.map((imgId, i) => (
              // eslint-disable-next-line @next/next/no-img-element -- รูปจาก Go ตาม 05 §2
              <img
                key={imgId}
                src={imageUrl(r.id, imgId)}
                alt={`รูปที่ ${i + 2} ของ ${r.name}`}
                width={136}
                height={94}
                className="h-[94px] w-[136px] rounded-[18px] border-2 border-white/90 object-cover"
                style={{ background: coverGradient(firstSlug) }}
              />
            ))}
          </div>
        )}
        <div className="glass absolute inset-x-4 bottom-4 flex flex-col gap-4 rounded-3xl bg-white/72 px-6 py-5 sm:inset-x-5 sm:bottom-5 lg:flex-row lg:items-end lg:justify-between">
          <div className="flex flex-col gap-2.5">
            <h1 className="text-3xl font-semibold sm:text-[42px]">{r.name}</h1>
            <div className="flex flex-wrap gap-2">
              {r.categories.map((c) => (
                <Pill key={c.id} tone="lav">
                  {c.name_th}
                </Pill>
              ))}
              {!r.accepting_bookings && <Pill tone="gray">พักรับจอง</Pill>}
            </div>
          </div>
          <div className="flex flex-wrap gap-3">
            <Stat label="คะแนน" tone="bg-(--st-butter-bg) text-(--st-butter-ink)">
              <StarIcon size={18} />
              {r.avg_rating === null ? "ยังไม่มีรีวิว" : `${r.avg_rating.toFixed(1)} · ${r.review_count} รีวิว`}
            </Stat>
            <Stat label="เวลาเปิด" tone="bg-(--st-lav-bg) text-(--st-lav-ink)">
              {r.open_time}–{r.close_time} น.
            </Stat>
            <Stat label="ที่นั่ง" tone="bg-(--st-mint-bg) text-(--st-mint-ink)">
              {r.capacity} ที่
            </Stat>
          </div>
        </div>
      </CoverImage>

      <div className="mt-7 grid items-start gap-7 lg:grid-cols-[minmax(0,1fr)_400px]">
        <div className="flex flex-col gap-6">
          <section className="glass flex flex-col gap-4 rounded-(--r-card) p-6 sm:p-7">
            <h2 className="text-[22px] font-semibold">เกี่ยวกับร้าน</h2>
            <p className="break-words whitespace-pre-line text-(--ink-body)">{r.description}</p>
            {/* นโยบายร้าน แสดงก่อนกดจอง (05 §4) */}
            <div className="grid gap-3 sm:grid-cols-3">
              <Policy title="ยกเลิกหรือแก้ไข" tone="from-(--st-peach-bg)/85">
                ได้ถึง {formatMinutes(r.cancel_window_minutes)}ก่อนเวลาจอง
              </Policy>
              <Policy title="จองล่วงหน้า" tone="from-(--st-lav-bg)/90">
                อย่างน้อย {formatMinutes(r.min_lead_minutes)} ไม่เกิน 30 วัน
              </Policy>
              <Policy title="ระยะเวลาต่อครั้ง" tone="from-(--st-mint-bg)/90">
                30 นาทีถึง 3 ชั่วโมง นับเป็นที่นั่ง
              </Policy>
            </div>
          </section>
          <Reviews restaurant={r} loggedIn={me !== null} initial={reviews} />
        </div>

        <BookingPanel
          restaurant={{
            id: r.id,
            name: r.name,
            capacity: r.capacity,
            cancel_window_minutes: r.cancel_window_minutes,
            accepting_bookings: r.accepting_bookings,
          }}
          loggedIn={me !== null}
          isOwner={r.is_owner}
          initialDate={todayInBangkok()}
        />
      </div>
    </main>
  );
}

function Stat({ label, tone, children }: { label: string; tone: string; children: React.ReactNode }) {
  return (
    <div className={`flex flex-col rounded-[18px] px-4 py-2.5 ${tone}`}>
      <span className="text-[13px]">{label}</span>
      <span className="font-heading flex items-center gap-1.5 text-lg font-semibold text-(--ink) sm:text-xl">{children}</span>
    </div>
  );
}

function Policy({ title, tone, children }: { title: string; tone: string; children: React.ReactNode }) {
  return (
    <div className={`flex flex-col gap-1.5 rounded-[20px] border border-white/95 bg-linear-160 ${tone} to-white/60 p-4`}>
      <span className="font-heading font-semibold">{title}</span>
      <span className="text-sm text-(--ink-2)">{children}</span>
    </div>
  );
}
