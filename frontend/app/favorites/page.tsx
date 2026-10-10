// ร้านที่ชอบ — Server Component อ่าน GET /api/me/favorites (ใหม่สุดก่อน ไม่รวมร้านที่ถูกลบ)
import type { Metadata } from "next";
import Link from "next/link";

import { CoverImage } from "@/components/cover-image";
import { FavoriteButton } from "@/components/favorite-button";
import { ClockIcon, StarIcon } from "@/components/icons";
import { Pill } from "@/components/pill";
import { Button } from "@/components/ui/button";
import { apiGet, requireMe } from "@/lib/api-server";
import type { FavoriteRestaurant } from "@/lib/types";

export const metadata: Metadata = { title: "ร้านที่ชอบ" };

export default async function FavoritesPage() {
  await requireMe("/favorites");
  const list = await apiGet<FavoriteRestaurant[]>("/api/me/favorites");

  return (
    <main className="mx-auto max-w-(--w-wide) px-4 pt-12 pb-18 sm:px-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-[40px] font-semibold">ร้านที่ชอบ</h1>
          <p className="mt-1.5 text-(--ink-2)">ร้านที่คุณกดหัวใจไว้ กดหัวใจอีกครั้งเพื่อเอาออก</p>
        </div>
        <Pill tone="lav">{list.length} ร้าน</Pill>
      </div>

      {list.length === 0 ? (
        <section className="glass mt-8 flex flex-col items-center gap-3 rounded-(--r-card) p-10 text-center">
          <h2 className="text-xl font-semibold">ยังไม่มีร้านที่ชอบ</h2>
          <p className="text-(--ink-2)">กดหัวใจที่การ์ดร้านหรือในหน้าร้าน เพื่อเก็บไว้กลับมาดูทีหลัง</p>
          <Button asChild variant="cta">
            <Link href="/">เลือกดูร้านอาหาร</Link>
          </Button>
        </section>
      ) : (
        <div className="mt-8 grid gap-7 sm:grid-cols-2 lg:grid-cols-3">
          {list.map((r) => (
            <Link
              key={r.id}
              href={`/restaurants/${r.id}`}
              className="glass flex flex-col overflow-hidden rounded-(--r-card) p-2.5 transition hover:-translate-y-0.5"
            >
              <CoverImage restaurantId={r.id} imageId={r.cover_image_id} alt={`ภาพร้าน ${r.name}`} className="h-[204px] rounded-[20px]">
                <Pill tone="glass" className="absolute top-3 right-3">
                  {r.avg_rating === null ? (
                    "ยังไม่มีรีวิว"
                  ) : (
                    <>
                      <StarIcon />
                      {r.avg_rating.toFixed(1)}
                    </>
                  )}
                </Pill>
                {!r.accepting_bookings && (
                  <Pill tone="gray" className="absolute top-3 left-3 border border-white/90">
                    พักรับจอง
                  </Pill>
                )}
                {/* กดเอาออกแล้วให้ Server Component โหลดรายการใหม่ ร้านจะหายจากหน้านี้ทันที */}
                <FavoriteButton restaurantId={r.id} initial refreshOnDone className="absolute right-3 bottom-3" />
              </CoverImage>
              <div className="flex flex-col gap-2.5 px-2.5 pt-4 pb-2.5">
                <h2 className="text-[21px] leading-snug font-semibold">{r.name}</h2>
                <div className="flex flex-wrap gap-x-3.5 gap-y-1 text-sm text-(--ink-2)">
                  <span className="flex items-center gap-1.5">
                    <ClockIcon />
                    {r.open_time}–{r.close_time} น.
                  </span>
                  <span>{r.review_count} รีวิว</span>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </main>
  );
}
