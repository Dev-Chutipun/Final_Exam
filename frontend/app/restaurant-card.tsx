import Link from "next/link";

import { CoverImage } from "@/components/cover-image";
import { FavoriteButton } from "@/components/favorite-button";
import { ClockIcon, PeopleIcon, StarIcon } from "@/components/icons";
import { Pill } from "@/components/pill";
import type { RestaurantListItem } from "@/lib/types";

export function RestaurantCard({ r, loggedIn }: { r: RestaurantListItem; loggedIn: boolean }) {
  return (
    <Link href={`/restaurants/${r.id}`} className="glass flex flex-col overflow-hidden rounded-(--r-card) p-2.5 transition hover:-translate-y-0.5">
      <CoverImage
        restaurantId={r.id}
        imageId={r.cover_image_id}
        categorySlug={r.categories[0]?.slug}
        alt={`ภาพร้าน ${r.name}`}
        className="h-[204px] rounded-[20px]"
      >
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
        {loggedIn && <FavoriteButton restaurantId={r.id} initial={r.is_favorite} className="absolute right-3 bottom-3" />}
        {!r.accepting_bookings && (
          <Pill tone="gray" className="absolute top-3 left-3 border border-white/90">
            พักรับจอง
          </Pill>
        )}
        {r.is_mine && (
          <Pill tone="peach" className="absolute bottom-3 left-3 border border-white/90">
            ร้านของคุณ
          </Pill>
        )}
      </CoverImage>
      <div className="flex flex-col gap-2.5 px-2.5 pt-4 pb-2.5">
        <h2 className="text-[21px] leading-snug font-semibold">{r.name}</h2>
        <div className="flex flex-wrap gap-1.5">
          {r.categories.map((c) => (
            <Pill key={c.id} tone="lav">
              {c.name_th}
            </Pill>
          ))}
        </div>
        <div className="flex flex-wrap gap-x-3.5 gap-y-1 text-sm text-(--ink-2)">
          <span className="flex items-center gap-1.5">
            <ClockIcon />
            {r.open_time}–{r.close_time} น.
          </span>
          <span className="flex items-center gap-1.5">
            <PeopleIcon />
            {r.capacity} ที่นั่ง
          </span>
          <span>{r.review_count} รีวิว</span>
        </div>
      </div>
    </Link>
  );
}
